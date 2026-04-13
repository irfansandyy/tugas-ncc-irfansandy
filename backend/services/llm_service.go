package services

import (
	"bufio"
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

type chatCompletionStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Text    string `json:"text"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type completionRequest struct {
	Model       string  `json:"model,omitempty"`
	Prompt      string  `json:"prompt"`
	NPredict    int     `json:"n_predict,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	Stream      bool    `json:"stream"`
}

type completionResponse struct {
	Content string `json:"content"`
	Choices []struct {
		Text    string      `json:"text"`
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (s *LLMService) apiBaseCandidates() []string {
	base := strings.TrimSuffix(s.baseURL, "/")
	candidates := []string{base}

	if strings.HasSuffix(base, "/engines/v1") {
		enginesBase := strings.TrimSuffix(base, "/v1")
		rootBase := strings.TrimSuffix(enginesBase, "/engines")
		candidates = append(candidates, enginesBase, enginesBase+"/llama.cpp", rootBase, rootBase+"/engines/llama.cpp")
	} else if strings.HasSuffix(base, "/engines") {
		candidates = append(candidates, strings.TrimSuffix(base, "/engines"))
		candidates = append(candidates, base+"/llama.cpp")
	} else if strings.HasSuffix(base, "/v1") {
		withoutV1 := strings.TrimSuffix(base, "/v1")
		candidates = append(candidates, withoutV1)
		if strings.HasSuffix(withoutV1, "/engines") {
			rootBase := strings.TrimSuffix(withoutV1, "/engines")
			candidates = append(candidates, withoutV1+"/llama.cpp", rootBase, rootBase+"/engines/llama.cpp")
		}
	} else {
		candidates = append(candidates, base+"/engines")
		candidates = append(candidates, base+"/engines/llama.cpp")
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

func openAIEndpointCandidates(apiBase, suffix string) []string {
	base := strings.TrimSuffix(apiBase, "/")
	candidates := make([]string, 0, 2)

	if strings.HasSuffix(base, "/v1") || strings.HasSuffix(base, "/engines/v1") {
		candidates = append(candidates, base+suffix)
	} else {
		candidates = append(candidates, base+"/v1"+suffix)
		candidates = append(candidates, base+suffix)
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

func messagesToPrompt(messages []chatMessage) string {
	var b strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			b.WriteString("System: ")
		case "assistant":
			b.WriteString("Assistant: ")
		default:
			b.WriteString("User: ")
		}
		b.WriteString(msg.Content)
		b.WriteString("\n")
	}
	b.WriteString("Assistant:")
	return b.String()
}

func parseCompletionText(respBody []byte) (string, error) {
	var completion completionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return "", err
	}

	if text := strings.TrimSpace(completion.Content); text != "" {
		return text, nil
	}

	if len(completion.Choices) > 0 {
		if text := strings.TrimSpace(completion.Choices[0].Text); text != "" {
			return text, nil
		}
		if text := strings.TrimSpace(completion.Choices[0].Message.Content); text != "" {
			return text, nil
		}
	}

	return "", errors.New("llm completion returned no text")
}

func (s *LLMService) tryCompletionFallback(ctx context.Context, prompt string, maxTokens int) (string, error) {
	payload := completionRequest{
		Model:       s.model,
		Prompt:      prompt,
		NPredict:    maxTokens,
		MaxTokens:   maxTokens,
		Temperature: 0.7,
		TopP:        0.9,
		Stream:      false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var lastErr error
	for _, apiBase := range s.apiBaseCandidates() {
		completionEndpoints := append(openAIEndpointCandidates(apiBase, "/completions"), openAIEndpointCandidates(apiBase, "/completion")...)
		for _, endpoint := range completionEndpoints {
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
			if reqErr != nil {
				lastErr = reqErr
				continue
			}
			req.Header.Set("Content-Type", "application/json")

			resp, doErr := s.client.Do(req)
			if doErr != nil {
				lastErr = fmt.Errorf("llm completion failed via %s: %w", endpoint, doErr)
				continue
			}

			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				lastErr = fmt.Errorf("llm completion endpoint not found at %s: status=%d body=%s", endpoint, resp.StatusCode, string(respBody))
				continue
			}

			if resp.StatusCode >= http.StatusBadRequest {
				lastErr = fmt.Errorf("llm completion failed via %s: status=%d body=%s", endpoint, resp.StatusCode, string(respBody))
				continue
			}

			text, parseErr := parseCompletionText(respBody)
			if parseErr != nil {
				lastErr = parseErr
				continue
			}

			return text, nil
		}
	}

	if lastErr != nil {
		return "", lastErr
	}

	return "", errors.New("llm completion fallback failed: no reachable endpoint")
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
		for _, endpoint := range openAIEndpointCandidates(apiBase, "/chat/completions") {
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
	}

	if lastErr != nil {
		fallbackPrompt := messagesToPrompt(requestMessages)
		fallbackReply, fallbackErr := s.tryCompletionFallback(ctx, fallbackPrompt, payload.MaxTokens)
		if fallbackErr == nil {
			return fallbackReply, nil
		}
		return "", fmt.Errorf("%v; completion fallback error: %w", lastErr, fallbackErr)
	}

	fallbackPrompt := messagesToPrompt(requestMessages)
	fallbackReply, fallbackErr := s.tryCompletionFallback(ctx, fallbackPrompt, payload.MaxTokens)
	if fallbackErr == nil {
		return fallbackReply, nil
	}

	return "", fmt.Errorf("llm request failed: no reachable endpoint; completion fallback error: %w", fallbackErr)
}

func (s *LLMService) GenerateReplyStream(
	ctx context.Context,
	history []models.Message,
	userPrompt string,
	onToken func(string) error,
) (string, error) {
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
		Stream:      true,
		MaxTokens:   512,
		TopP:        0.9,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var lastErr error
	for _, apiBase := range s.apiBaseCandidates() {
		for _, endpoint := range openAIEndpointCandidates(apiBase, "/chat/completions") {
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
			if reqErr != nil {
				lastErr = reqErr
				continue
			}
			req.Header.Set("Content-Type", "application/json")

			resp, doErr := s.client.Do(req)
			if doErr != nil {
				lastErr = fmt.Errorf("llm stream failed via %s: %w", endpoint, doErr)
				continue
			}

			if resp.StatusCode == http.StatusNotFound {
				respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				resp.Body.Close()
				lastErr = fmt.Errorf("llm stream endpoint not found at %s: status=%d body=%s", endpoint, resp.StatusCode, string(respBody))
				continue
			}

			if resp.StatusCode >= http.StatusBadRequest {
				respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				resp.Body.Close()
				return "", fmt.Errorf("llm stream failed via %s: status=%d body=%s", endpoint, resp.StatusCode, string(respBody))
			}

			reply, streamErr := consumeChatCompletionStream(resp.Body, onToken)
			resp.Body.Close()
			if streamErr != nil {
				lastErr = streamErr
				continue
			}

			if strings.TrimSpace(reply) != "" {
				return strings.TrimSpace(reply), nil
			}
			lastErr = errors.New("llm stream returned empty reply")
		}
	}

	fallbackReply, fallbackErr := s.GenerateReply(ctx, history, userPrompt)
	if fallbackErr != nil {
		if lastErr != nil {
			return "", fmt.Errorf("%v; fallback error: %w", lastErr, fallbackErr)
		}
		return "", fallbackErr
	}

	if onToken != nil {
		for _, piece := range strings.SplitAfter(fallbackReply, " ") {
			if piece == "" {
				continue
			}
			if err := onToken(piece); err != nil {
				return "", err
			}
		}
	}

	return fallbackReply, nil
}

func consumeChatCompletionStream(body io.Reader, onToken func(string) error) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var builder strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var chunk chatCompletionStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		delta := ""
		if len(chunk.Choices) > 0 {
			delta = chunk.Choices[0].Delta.Content
			if delta == "" {
				delta = chunk.Choices[0].Text
			}
			if delta == "" {
				delta = chunk.Choices[0].Message.Content
			}
		}

		if delta == "" {
			continue
		}

		builder.WriteString(delta)
		if onToken != nil {
			if err := onToken(delta); err != nil {
				return "", err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return builder.String(), nil
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
		for _, modelsURL := range openAIEndpointCandidates(apiBase, "/models") {
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
	}

	if lastErr != nil {
		probeCtx, probeCancel := context.WithTimeout(ctx, 2*time.Second)
		defer probeCancel()
		if _, probeErr := s.tryCompletionFallback(probeCtx, "User: ping\nAssistant:", 1); probeErr == nil {
			return nil
		} else {
			return fmt.Errorf("%v; completion probe error: %w", lastErr, probeErr)
		}
	}

	probeCtx, probeCancel := context.WithTimeout(ctx, 2*time.Second)
	defer probeCancel()
	if _, probeErr := s.tryCompletionFallback(probeCtx, "User: ping\nAssistant:", 1); probeErr == nil {
		return nil
	}

	return errors.New("llm health failed: no reachable endpoint")
}
