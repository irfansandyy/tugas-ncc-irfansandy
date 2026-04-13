package services

import (
	"context"
	"fmt"
	"strings"

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

func (s *ChatService) ListMessages(ctx context.Context, userID, chatID int64) ([]models.Message, error) {
	return s.chatRepo.ListMessagesByChat(ctx, chatID, userID, 0)
}

func (s *ChatService) SendMessage(ctx context.Context, userID, chatID int64, content string) (models.Message, models.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return models.Message{}, models.Message{}, fmt.Errorf("message content is required")
	}

	if _, err := s.chatRepo.GetChatByID(ctx, chatID, userID); err != nil {
		return models.Message{}, models.Message{}, err
	}

	userMessage, err := s.chatRepo.CreateMessage(ctx, chatID, "user", content)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	history, err := s.chatRepo.ListMessagesByChat(ctx, chatID, userID, 20)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	reply, err := s.llm.GenerateReply(ctx, history, content)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	assistantMessage, err := s.chatRepo.CreateMessage(ctx, chatID, "assistant", reply)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	if err := s.chatRepo.UpdateChatTimestamp(ctx, chatID); err != nil {
		return models.Message{}, models.Message{}, err
	}

	return userMessage, assistantMessage, nil
}

func (s *ChatService) SendMessageStream(
	ctx context.Context,
	userID, chatID int64,
	content string,
	onToken func(string) error,
) (models.Message, models.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return models.Message{}, models.Message{}, fmt.Errorf("message content is required")
	}

	if _, err := s.chatRepo.GetChatByID(ctx, chatID, userID); err != nil {
		return models.Message{}, models.Message{}, err
	}

	userMessage, err := s.chatRepo.CreateMessage(ctx, chatID, "user", content)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	history, err := s.chatRepo.ListMessagesByChat(ctx, chatID, userID, 20)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	reply, err := s.llm.GenerateReplyStream(ctx, history, content, onToken)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	assistantMessage, err := s.chatRepo.CreateMessage(ctx, chatID, "assistant", reply)
	if err != nil {
		return models.Message{}, models.Message{}, err
	}

	if err := s.chatRepo.UpdateChatTimestamp(ctx, chatID); err != nil {
		return models.Message{}, models.Message{}, err
	}

	return userMessage, assistantMessage, nil
}
