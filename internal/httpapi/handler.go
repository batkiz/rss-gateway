package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/batkiz/rss-gateway/internal/model"
	"github.com/batkiz/rss-gateway/internal/pipeline"
	"github.com/batkiz/rss-gateway/internal/rssout"
	"github.com/batkiz/rss-gateway/internal/ui"
)

type Handler struct {
	service *pipeline.Service
}

func New(service *pipeline.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.StripSlashes)
	r.Use(chimiddleware.Timeout(6 * time.Minute))
	r.Use(requestLogger)

	r.Get("/healthz", h.handleHealth)
	r.Get("/admin", h.handleAdminPage)
	r.Post("/admin", h.handleAdminPage)
	r.Get("/admin/status", h.handleStatus)
	r.Post("/admin/refresh", h.handleRefresh)
	r.Post("/admin/reprocess", h.handleReprocess)
	r.Get("/admin/raw-items", h.handleRawItems)
	r.Get("/sources", h.handleSources)
	r.Get("/feeds/{sourceID}.rss", h.handleFeed)
	return r
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleSources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.service.Sources())
}

func (h *Handler) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderAdminPage(w, r, r.URL.Query().Get("message"), r.URL.Query().Get("error"))
	case http.MethodPost:
		h.handleAdminAction(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
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

func (h *Handler) handleAdminAction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectAdmin(w, r, r.FormValue("source"), r.FormValue("lang"), "", "invalid form data")
		return
	}

	action := r.FormValue("action")
	sourceID := r.FormValue("source")
	lang := r.FormValue("lang")
	limit := positiveInt(r.FormValue("limit"), 10)
	log.Printf("admin action=%s source=%s limit=%d", action, sourceID, limit)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	var message string
	var err error
	switch action {
	case "refresh":
		err = h.service.RefreshSource(ctx, sourceID)
		if err == nil {
			message = "refreshed source " + sourceID
		}
	case "refresh_all":
		err = h.service.RefreshAll(ctx)
		if err == nil {
			message = "refreshed all enabled sources"
		}
	case "reprocess":
		err = h.service.ReprocessSource(ctx, sourceID, limit)
		if err == nil {
			message = "reprocessed source " + sourceID
		}
	default:
		h.redirectAdmin(w, r, sourceID, lang, "", "unknown admin action")
		return
	}

	if err != nil {
		h.redirectAdmin(w, r, sourceID, lang, "", err.Error())
		return
	}
	h.redirectAdmin(w, r, sourceID, lang, message, "")
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sourceID := r.URL.Query().Get("source")
	log.Printf("api refresh requested source=%s", sourceID)
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
	log.Printf("api reprocess requested source=%s limit=%d", sourceID, limit)
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
	log.Printf("api raw-items requested source=%s limit=%d", sourceID, limit)
	items, err := h.service.ListRawItems(r.Context(), sourceID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleFeed(w http.ResponseWriter, r *http.Request) {
	sourceID := chi.URLParam(r, "sourceID")
	if sourceID == "" {
		http.NotFound(w, r)
		return
	}

	source, ok := h.service.Sources()[sourceID]
	if !ok {
		http.NotFound(w, r)
		return
	}
	log.Printf("feed requested source=%s max_items=%d", sourceID, source.MaxItems)

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

func positiveInt(value string, defaultValue int) int {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	return parsed
}

func (h *Handler) renderAdminPage(w http.ResponseWriter, r *http.Request, message, errText string) {
	selectedSource := r.URL.Query().Get("source")
	status, err := h.service.FeedStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if selectedSource == "" {
		for id := range h.service.Sources() {
			selectedSource = id
			break
		}
	}

	rawItems := make([]model.RawItem, 0)
	if selectedSource != "" {
		rawItems, err = h.service.ListRawItems(r.Context(), selectedSource, 8)
		if err != nil {
			errText = err.Error()
		}
	}

	vm := ui.BuildAdminPageView(r, h.service.Sources(), status, rawItems, selectedSource, message, errText)
	templ.Handler(ui.AdminPage(vm)).ServeHTTP(w, r)
}

func (h *Handler) redirectAdmin(w http.ResponseWriter, r *http.Request, sourceID, lang, message, errText string) {
	values := r.URL.Query()
	if sourceID != "" {
		values.Set("source", sourceID)
	}
	if lang != "" {
		values.Set("lang", lang)
	}
	if message != "" {
		values.Set("message", message)
	}
	if errText != "" {
		values.Set("error", errText)
	}
	http.Redirect(w, r, "/admin?"+values.Encode(), http.StatusSeeOther)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		requestID := chimiddleware.GetReqID(r.Context())
		log.Printf("http method=%s path=%s status=%d remote=%s request_id=%s duration=%s", r.Method, r.URL.RequestURI(), recorder.status, r.RemoteAddr, requestID, time.Since(start).Round(time.Millisecond))
	})
}
