package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"app-backend/models"
	"app-backend/repositories"
)

type ChatService struct {
	chatRepo repositories.ChatRepository
	llm      *LLMService
}

func NewChatService(chatRepo repositories.ChatRepository, llm *LLMService) *ChatService {
	return &ChatService{chatRepo: chatRepo, llm: llm}
}

func (s *ChatService) CreateChat(ctx context.Context, userID int64, title string) (models.Chat, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New Chat"
	}
	return s.chatRepo.CreateChat(ctx, userID, title)
}

func (s *ChatService) ListChats(ctx context.Context, userID int64) ([]models.Chat, error) {
	return s.chatRepo.ListChatsByUser(ctx, userID)
}

func (s *ChatService) ListMessages(ctx context.Context, userID int64, chatSlug string) ([]models.Message, error) {
	chat, err := s.chatRepo.GetChatBySlug(ctx, chatSlug, userID)
	if err != nil {
		return nil, err
	}

	return s.chatRepo.ListMessagesByChat(ctx, chat.ID, userID, 0)
}

func (s *ChatService) SendMessage(ctx context.Context, userID int64, chatSlug, content string) (models.Message, models.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return models.Message{}, models.Message{}, fmt.Errorf("message content is required")
	}

	chat, err := s.chatRepo.GetChatBySlug(ctx, chatSlug, userID)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	hadMessages := false
	if existing, listErr := s.chatRepo.ListMessagesByChat(ctx, chat.ID, userID, 1); listErr == nil && len(existing) > 0 {
		hadMessages = true
	}

	userMessage, err := s.chatRepo.CreateMessage(ctx, chat.ID, "user", content)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	history, err := s.chatRepo.ListMessagesByChat(ctx, chat.ID, userID, 20)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	reply, err := s.llm.GenerateReply(ctx, history, content)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	assistantMessage, err := s.chatRepo.CreateMessage(ctx, chat.ID, "assistant", reply)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	if err := s.chatRepo.UpdateChatTimestamp(ctx, chat.ID); err != nil {
		return models.Message{}, models.Message{}, err
	}

	if !hadMessages {
		titleCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		title, titleErr := s.llm.GenerateTitle(titleCtx, content)
		cancel()
		if titleErr == nil && strings.TrimSpace(title) != "" {
			_, _ = s.chatRepo.UpdateChatTitle(ctx, chat.ID, userID, strings.TrimSpace(title))
		}
	}

	return userMessage, assistantMessage, nil
}

func (s *ChatService) SendMessageStream(
	ctx context.Context,
	userID int64,
	chatSlug string,
	content string,
	onToken func(string) error,
) (models.Message, models.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return models.Message{}, models.Message{}, fmt.Errorf("message content is required")
	}

	chat, err := s.chatRepo.GetChatBySlug(ctx, chatSlug, userID)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	hadMessages := false
	if existing, listErr := s.chatRepo.ListMessagesByChat(ctx, chat.ID, userID, 1); listErr == nil && len(existing) > 0 {
		hadMessages = true
	}

	userMessage, err := s.chatRepo.CreateMessage(ctx, chat.ID, "user", content)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	history, err := s.chatRepo.ListMessagesByChat(ctx, chat.ID, userID, 20)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	reply, err := s.llm.GenerateReplyStream(ctx, history, content, onToken)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	assistantMessage, err := s.chatRepo.CreateMessage(ctx, chat.ID, "assistant", reply)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	if err := s.chatRepo.UpdateChatTimestamp(ctx, chat.ID); err != nil {
		return models.Message{}, models.Message{}, err
	}

	if !hadMessages {
		titleCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		title, titleErr := s.llm.GenerateTitle(titleCtx, content)
		cancel()
		if titleErr == nil && strings.TrimSpace(title) != "" {
			_, _ = s.chatRepo.UpdateChatTitle(ctx, chat.ID, userID, strings.TrimSpace(title))
		}
	}

	return userMessage, assistantMessage, nil
}
