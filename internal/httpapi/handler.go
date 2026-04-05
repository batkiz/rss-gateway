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

func (h *Handler) Router() *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.StripSlashes)
	r.Use(chimiddleware.Timeout(6 * time.Minute))
	r.Use(requestLogger)

	r.Get("/healthz", h.handleHealth)
	r.Get("/admin", h.handleDashboardPage)
	r.Post("/admin", h.handleAdminAction)
	r.Get("/admin/settings/llm", h.handleLLMPage)
	r.Post("/admin/settings/llm", h.handleSaveLLMSettings)
	r.Get("/admin/modes", h.handleModesPage)
	r.Post("/admin/settings/mode", h.handleSaveMode)
	r.Get("/admin/sources", h.handleSourcesPage)
	r.Post("/admin/settings/source", h.handleSaveSource)
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

func (h *Handler) handleSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.service.ListSources(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

func (h *Handler) handleDashboardPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminPage(w, r, ui.AdminSectionDashboard, r.URL.Query().Get("message"), r.URL.Query().Get("error"))
}

func (h *Handler) handleLLMPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminPage(w, r, ui.AdminSectionLLM, r.URL.Query().Get("message"), r.URL.Query().Get("error"))
}

func (h *Handler) handleModesPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminPage(w, r, ui.AdminSectionModes, r.URL.Query().Get("message"), r.URL.Query().Get("error"))
}

func (h *Handler) handleSourcesPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminPage(w, r, ui.AdminSectionSources, r.URL.Query().Get("message"), r.URL.Query().Get("error"))
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.FeedStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handleAdminAction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectAdmin(w, r, "/admin", r.FormValue("source"), r.FormValue("mode"), r.FormValue("lang"), "", "invalid form data")
		return
	}

	action := r.FormValue("action")
	sourceID := r.FormValue("source")
	modeName := r.FormValue("mode")
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
		h.redirectAdmin(w, r, "/admin", sourceID, modeName, lang, "", "unknown admin action")
		return
	}

	if err != nil {
		h.redirectAdmin(w, r, "/admin", sourceID, modeName, lang, "", err.Error())
		return
	}
	h.redirectAdmin(w, r, "/admin", sourceID, modeName, lang, message, "")
}

func (h *Handler) handleSaveLLMSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectAdmin(w, r, "/admin/settings/llm", r.FormValue("source"), r.FormValue("mode"), r.FormValue("lang"), "", "invalid form data")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()

	current, err := h.service.GetLLMSettings(ctx)
	if err != nil {
		h.redirectAdmin(w, r, "/admin/settings/llm", r.FormValue("source"), r.FormValue("mode"), r.FormValue("lang"), "", err.Error())
		return
	}
	apiKey := strings.TrimSpace(r.FormValue("api_key"))
	if apiKey == "" {
		apiKey = current.APIKey
	}

	err = h.service.SaveLLMSettings(ctx, model.LLMSettings{
		Provider: r.FormValue("provider"),
		Model:    r.FormValue("model"),
		APIKey:   apiKey,
		BaseURL:  r.FormValue("base_url"),
		Timeout:  r.FormValue("timeout"),
	})
	if err != nil {
		h.redirectAdmin(w, r, "/admin/settings/llm", r.FormValue("source"), r.FormValue("mode"), r.FormValue("lang"), "", err.Error())
		return
	}
	h.redirectAdmin(w, r, "/admin/settings/llm", r.FormValue("source"), r.FormValue("mode"), r.FormValue("lang"), "saved llm settings", "")
}

func (h *Handler) handleSaveMode(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectAdmin(w, r, "/admin/modes", r.FormValue("source"), r.FormValue("selected_mode"), r.FormValue("lang"), "", "invalid form data")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()

	extras, err := parseExtraFieldsJSON(r.FormValue("extra_fields_json"))
	if err != nil {
		h.redirectAdmin(w, r, "/admin/modes", r.FormValue("source"), r.FormValue("selected_mode"), r.FormValue("lang"), "", err.Error())
		return
	}
	temperature, err := parseOptionalFloat(r.FormValue("temperature"))
	if err != nil {
		h.redirectAdmin(w, r, "/admin/modes", r.FormValue("source"), r.FormValue("selected_mode"), r.FormValue("lang"), "", err.Error())
		return
	}
	mode := model.Mode{
		Name:            r.FormValue("name"),
		SystemPrompt:    r.FormValue("system_prompt"),
		TaskPrompt:      r.FormValue("task_prompt"),
		Temperature:     temperature,
		MaxOutputTokens: positiveInt(r.FormValue("max_output_tokens"), 0),
		OutputSchema: model.OutputSchema{
			Name:         strings.TrimSpace(r.FormValue("schema_name")),
			TitleField:   strings.TrimSpace(r.FormValue("title_field")),
			SummaryField: strings.TrimSpace(r.FormValue("summary_field")),
			ContentField: strings.TrimSpace(r.FormValue("content_field")),
			Fields: append([]model.OutputField{
				{Name: strings.TrimSpace(r.FormValue("title_field")), Type: "string", Description: "Reader-facing title", Required: true},
				{Name: strings.TrimSpace(r.FormValue("summary_field")), Type: "string", Description: "Short summary", Required: true},
				{Name: strings.TrimSpace(r.FormValue("content_field")), Type: "string", Description: "RSS content body", Required: true},
			}, extras...),
		},
	}
	if err := h.service.SaveMode(ctx, mode); err != nil {
		h.redirectAdmin(w, r, "/admin/modes", r.FormValue("source"), r.FormValue("selected_mode"), r.FormValue("lang"), "", err.Error())
		return
	}
	h.redirectAdmin(w, r, "/admin/modes", r.FormValue("source"), mode.Name, r.FormValue("lang"), "saved mode "+mode.Name, "")
}

func (h *Handler) handleSaveSource(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectAdmin(w, r, "/admin/sources", r.FormValue("selected_source"), r.FormValue("mode"), r.FormValue("lang"), "", "invalid form data")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()

	refreshInterval, err := time.ParseDuration(strings.TrimSpace(r.FormValue("refresh_interval")))
	if err != nil {
		h.redirectAdmin(w, r, "/admin/sources", r.FormValue("selected_source"), r.FormValue("mode"), r.FormValue("lang"), "", "invalid refresh interval")
		return
	}
	temperature, err := parseOptionalFloat(r.FormValue("temperature"))
	if err != nil {
		h.redirectAdmin(w, r, "/admin/sources", r.FormValue("selected_source"), r.FormValue("mode"), r.FormValue("lang"), "", err.Error())
		return
	}

	source := model.Source{
		ID:              r.FormValue("id"),
		Name:            r.FormValue("name"),
		URL:             r.FormValue("url"),
		RefreshInterval: refreshInterval,
		Enabled:         r.FormValue("enabled") != "",
		MaxItems:        positiveInt(r.FormValue("max_items"), 20),
		PipelineMode:    r.FormValue("pipeline_mode"),
		SystemPrompt:    r.FormValue("system_prompt"),
		TaskPrompt:      r.FormValue("task_prompt"),
		MaxInputChars:   positiveInt(r.FormValue("max_input_chars"), 8000),
		ExtractFull:     r.FormValue("extract_full_content") != "",
		Temperature:     temperature,
		MaxOutputTokens: positiveInt(r.FormValue("max_output_tokens"), 0),
	}
	if err := h.service.SaveSource(ctx, source); err != nil {
		h.redirectAdmin(w, r, "/admin/sources", r.FormValue("selected_source"), r.FormValue("mode"), r.FormValue("lang"), "", err.Error())
		return
	}
	h.redirectAdmin(w, r, "/admin/sources", source.ID, r.FormValue("mode"), r.FormValue("lang"), "saved source "+source.ID, "")
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
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

	source, err := h.service.GetSource(r.Context(), sourceID)
	if err != nil {
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

func (h *Handler) renderAdminPage(w http.ResponseWriter, r *http.Request, section, message, errText string) {
	selectedSource := r.URL.Query().Get("source")
	selectedMode := r.URL.Query().Get("mode")

	settings, err := h.service.GetLLMSettings(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sources, err := h.service.ListSources(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	modes, err := h.service.ListModes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	status, err := h.service.FeedStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if selectedSource == "" && len(sources) > 0 {
		selectedSource = sources[0].ID
	}
	if selectedMode == "" && len(modes) > 0 {
		selectedMode = modes[0].Name
	}

	rawItems := make([]model.RawItem, 0)
	processedItems := make([]model.ProcessedItem, 0)
	if selectedSource != "" {
		rawItems, err = h.service.ListRawItems(r.Context(), selectedSource, 8)
		if err != nil {
			errText = err.Error()
		}
		processedItems, err = h.service.ListProcessedItems(r.Context(), selectedSource, 8)
		if err != nil && errText == "" {
			errText = err.Error()
		}
	}

	vm := ui.BuildAdminPageView(r, section, settings, sources, modes, status, rawItems, processedItems, selectedSource, selectedMode, message, errText)
	templ.Handler(ui.AdminPage(vm)).ServeHTTP(w, r)
}

func (h *Handler) redirectAdmin(w http.ResponseWriter, r *http.Request, path, sourceID, modeName, lang, message, errText string) {
	values := r.URL.Query()
	if sourceID != "" {
		values.Set("source", sourceID)
	}
	if modeName != "" {
		values.Set("mode", modeName)
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
	target := path
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
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
	if err != nil || parsed < 0 {
		return defaultValue
	}
	return parsed
}

func parseOptionalFloat(value string) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseExtraFieldsJSON(value string) ([]model.OutputField, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	var fields []model.OutputField
	if err := json.Unmarshal([]byte(value), &fields); err != nil {
		return nil, err
	}
	return fields, nil
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
