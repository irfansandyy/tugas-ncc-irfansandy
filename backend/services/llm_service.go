package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"app-backend/config"
	"app-backend/models"
)

type LLMService struct {
	baseURL string
	model   string
	ctxSize int
	client  *http.Client
}

var (
	llmInstance *LLMService
	llmOnce     sync.Once
)

func GetLLMService(cfg config.Config) *LLMService {
	llmOnce.Do(func() {
		llmInstance = &LLMService{
			baseURL: strings.TrimSuffix(cfg.LLMBaseURL, "/"),
			model:   cfg.LLMModel,
			ctxSize: cfg.LLMCtxSize,
			client: &http.Client{
				Timeout: cfg.LLMTimeout,
			},
		}
	})

	return llmInstance
}

type chatCompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []chatMessage       `json:"messages"`
	Temperature float64             `json:"temperature"`
	Stream      bool                `json:"stream"`
	MaxTokens   int                 `json:"max_tokens"`
	ExtraBody   map[string]any      `json:"extra_body,omitempty"`
	Metadata    map[string]string   `json:"metadata,omitempty"`
	Stop        []string            `json:"stop,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
	PresenceP   float64             `json:"presence_penalty,omitempty"`
	FrequencyP  float64             `json:"frequency_penalty,omitempty"`
	LogitBias   map[string]float64  `json:"logit_bias,omitempty"`
	Tools       []map[string]any    `json:"tools,omitempty"`
	ToolChoice  map[string]string   `json:"tool_choice,omitempty"`
	Seed        *int                `json:"seed,omitempty"`
	User        string              `json:"user,omitempty"`
	Functions   []map[string]string `json:"functions,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (s *LLMService) apiBaseCandidates() []string {
	base := strings.TrimSuffix(s.baseURL, "/")
	candidates := []string{base}

	if strings.HasSuffix(base, "/engines") {
		candidates = append(candidates, strings.TrimSuffix(base, "/engines"))
	} else if !strings.HasSuffix(base, "/v1") && !strings.HasSuffix(base, "/engines/v1") {
		candidates = append(candidates, base+"/engines")
	}

	seen := map[string]bool{}
	unique := make([]string, 0, len(candidates))
	for _, item := range candidates {
		if seen[item] {
			continue
		}
		seen[item] = true
		unique = append(unique, item)
	}

	return unique
}

func toOpenAIEndpoint(apiBase, suffix string) string {
	if strings.HasSuffix(apiBase, "/v1") || strings.HasSuffix(apiBase, "/engines/v1") {
		return apiBase + suffix
	}

	return apiBase + "/v1" + suffix
}

func (s *LLMService) GenerateReply(ctx context.Context, history []models.Message, userPrompt string) (string, error) {
	requestMessages := []chatMessage{
		{
			Role: "system",
			Content: "You are a concise, helpful assistant. " +
				"Keep answers practical and grounded.",
		},
	}

	for _, msg := range history {
		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}
		requestMessages = append(requestMessages, chatMessage{Role: msg.Role, Content: msg.Content})
	}

	shouldAppendPrompt := true
	if len(history) > 0 {
		last := history[len(history)-1]
		if last.Role == "user" && strings.TrimSpace(last.Content) == strings.TrimSpace(userPrompt) {
			shouldAppendPrompt = false
		}
	}

	if shouldAppendPrompt {
		requestMessages = append(requestMessages, chatMessage{Role: "user", Content: userPrompt})
	}

	requestMessages = limitMessagesByContext(requestMessages, s.ctxSize)

	payload := chatCompletionRequest{
		Model:       s.model,
		Messages:    requestMessages,
		Temperature: 0.7,
		Stream:      false,
		MaxTokens:   512,
		TopP:        0.9,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var lastErr error
	for _, apiBase := range s.apiBaseCandidates() {
		endpoint := toOpenAIEndpoint(apiBase, "/chat/completions")
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := s.client.Do(req)
		if doErr != nil {
			lastErr = fmt.Errorf("llm request failed via %s: %w", endpoint, doErr)
			continue
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			lastErr = fmt.Errorf("llm endpoint not found at %s: status=%d body=%s", endpoint, resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode >= http.StatusBadRequest {
			return "", fmt.Errorf("llm request failed via %s: status=%d body=%s", endpoint, resp.StatusCode, string(respBody))
		}

		var completion chatCompletionResponse
		if decodeErr := json.Unmarshal(respBody, &completion); decodeErr != nil {
			return "", decodeErr
		}

		if len(completion.Choices) == 0 {
			return "", errors.New("llm returned no choices")
		}

		return strings.TrimSpace(completion.Choices[0].Message.Content), nil
	}

	if lastErr != nil {
		return "", lastErr
	}

	return "", errors.New("llm request failed: no reachable endpoint")
}

func limitMessagesByContext(messages []chatMessage, ctxSize int) []chatMessage {
	if len(messages) <= 1 || ctxSize <= 0 {
		return messages
	}

	inputBudget := ctxSize - 512
	if inputBudget < 512 {
		inputBudget = ctxSize
	}

	system := messages[0]
	usedTokens := estimateTokens(system.Content) + 8

	recentReverse := make([]chatMessage, 0, len(messages)-1)
	for i := len(messages) - 1; i >= 1; i-- {
		msg := messages[i]
		msgTokens := estimateTokens(msg.Content) + 8

		if usedTokens+msgTokens > inputBudget {
			if len(recentReverse) == 0 {
				recentReverse = append(recentReverse, msg)
			}
			continue
		}

		recentReverse = append(recentReverse, msg)
		usedTokens += msgTokens
	}

	trimmed := make([]chatMessage, 0, len(recentReverse)+1)
	trimmed = append(trimmed, system)
	for i := len(recentReverse) - 1; i >= 0; i-- {
		trimmed = append(trimmed, recentReverse[i])
	}

	return trimmed
}

func estimateTokens(content string) int {
	charCount := len([]rune(content))
	if charCount == 0 {
		return 0
	}

	return (charCount / 4) + 1
}

func (s *LLMService) HealthCheck(ctx context.Context) error {
	for _, apiBase := range s.apiBaseCandidates() {
		healthURL := apiBase + "/health"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err != nil {
			continue
		}

		resp, reqErr := s.client.Do(req)
		if reqErr != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
	}

	fallbackCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var lastErr error
	for _, apiBase := range s.apiBaseCandidates() {
		modelsURL := toOpenAIEndpoint(apiBase, "/models")
		modelsReq, err := http.NewRequestWithContext(fallbackCtx, http.MethodGet, modelsURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, reqErr := s.client.Do(modelsReq)
		if reqErr != nil {
			lastErr = fmt.Errorf("llm health request failed via %s: %w", modelsURL, reqErr)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		lastErr = fmt.Errorf("llm health failed via %s with status %d", modelsURL, resp.StatusCode)
	}

	if lastErr != nil {
		return lastErr
	}

	return errors.New("llm health failed: no reachable endpoint")
}
