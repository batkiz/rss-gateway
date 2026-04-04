package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/batkiz/rss-gateway/internal/pipeline"
	"github.com/batkiz/rss-gateway/internal/rssout"
)

type Handler struct {
	service *pipeline.Service
}

func New(service *pipeline.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/admin/status", h.handleStatus)
	mux.HandleFunc("/admin/refresh", h.handleRefresh)
	mux.HandleFunc("/admin/reprocess", h.handleReprocess)
	mux.HandleFunc("/admin/raw-items", h.handleRawItems)
	mux.HandleFunc("/feeds/", h.handleFeed)
	mux.HandleFunc("/sources", h.handleSources)
	return mux
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleSources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.service.Sources())
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	status, err := h.service.FeedStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sourceID := r.URL.Query().Get("source")
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	if sourceID != "" {
		if err := h.service.RefreshSource(ctx, sourceID); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "source": sourceID})
		return
	}

	if err := h.service.RefreshAll(ctx); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleReprocess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sourceID := r.URL.Query().Get("source")
	if sourceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source query parameter is required"})
		return
	}

	limit := intQuery(r, "limit", 0)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	if err := h.service.ReprocessSource(ctx, sourceID, limit); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "source": sourceID, "limit": limit})
}

func (h *Handler) handleRawItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sourceID := r.URL.Query().Get("source")
	if sourceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source query parameter is required"})
		return
	}

	limit := intQuery(r, "limit", 20)
	items, err := h.service.ListRawItems(r.Context(), sourceID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sourceID := strings.TrimPrefix(r.URL.Path, "/feeds/")
	sourceID = strings.TrimSuffix(sourceID, ".rss")
	if sourceID == "" {
		http.NotFound(w, r)
		return
	}

	source, ok := h.service.Sources()[sourceID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	items, err := h.service.ListProcessedItems(r.Context(), sourceID, source.MaxItems)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	feedTitle := source.Name
	if feedTitle == "" {
		feedTitle = source.ID
	}
	data, err := rssout.RenderFeed(feedTitle, source.URL, "LLM transformed RSS feed", items)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func intQuery(r *http.Request, key string, defaultValue int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultValue
	}
	return value
}
