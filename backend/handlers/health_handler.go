package handlers

import (
	"net/http"

	"app-backend/services"
)

type HealthHandler struct {
	healthService *services.HealthService
}

func NewHealthHandler(healthService *services.HealthService) *HealthHandler {
	return &HealthHandler{healthService: healthService}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	result := h.healthService.Check(r.Context())

	if r.URL.Query().Get("simple") == "true" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if result.Status == "ok" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"services": map[string]string{
				"database": "ok",
				"llm":      "ok",
			},
		})
		return
	}

	payload := map[string]any{
		"status":   result.Status,
		"services": result.Services,
	}
	if len(result.Errors) > 0 {
		payload["errors"] = result.Errors
	}

	writeJSON(w, http.StatusOK, payload)
}
