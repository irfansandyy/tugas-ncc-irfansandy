package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"app-backend/middleware"
	"app-backend/repositories"
	"app-backend/services"

	"github.com/go-chi/chi/v5"
)

type ChatHandler struct {
	chatService *services.ChatService
}

type createChatRequest struct {
	Title string `json:"title"`
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

func NewChatHandler(chatService *services.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

func (h *ChatHandler) ListChats(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	chats, err := h.chatService.ListChats(r.Context(), userID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch chats")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": chats})
}

func (h *ChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	var req createChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	chat, err := h.chatService.CreateChat(r.Context(), userID, req.Title)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			writeJSONError(w, http.StatusUnauthorized, "session is invalid, please login again")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to create chat")
		return
	}

	writeJSON(w, http.StatusCreated, chat)
}

func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	chatID, err := parseChatSlug(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	messages, err := h.chatService.ListMessages(r.Context(), userID, chatID)
	if err != nil {
		if errors.Is(err, repositories.ErrChatNotFound) {
			writeJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch messages")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": messages})
}

func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	chatID, err := parseChatSlug(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userMsg, assistantMsg, err := h.chatService.SendMessage(r.Context(), userID, chatID, req.Content)
	if err != nil {
		if errors.Is(err, repositories.ErrChatNotFound) {
			writeJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to process message")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_message":      userMsg,
		"assistant_message": assistantMsg,
	})
}

func (h *ChatHandler) SendMessageStream(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	chatID, err := parseChatSlug(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	writeSSE := func(event string, payload any) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
			return err
		}
		if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	userMsg, assistantMsg, streamErr := h.chatService.SendMessageStream(
		r.Context(),
		userID,
		chatID,
		req.Content,
		func(delta string) error {
			return writeSSE("token", map[string]string{"delta": delta})
		},
	)
	if streamErr != nil {
		_ = writeSSE("error", map[string]string{"error": streamErr.Error()})
		return
	}

	_ = writeSSE("done", map[string]any{
		"user_message":      userMsg,
		"assistant_message": assistantMsg,
	})
}

func parseChatSlug(r *http.Request) (string, error) {
	chatSlug := chi.URLParam(r, "chatSlug")
	chatSlug = strings.TrimSpace(chatSlug)
	if chatSlug == "" {
		return "", errors.New("missing chat slug")
	}

	return chatSlug, nil
}
