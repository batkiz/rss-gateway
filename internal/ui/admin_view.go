package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/templui/templui/components/badge"

	"github.com/batkiz/rss-gateway/internal/model"
)

const (
	AdminSectionDashboard = "dashboard"
	AdminSectionLLM       = "llm"
	AdminSectionModes     = "modes"
	AdminSectionSources   = "sources"
)

type AdminPageView struct {
	Lang           string
	Text           AdminText
	Section        string
	Message        string
	Error          string
	GeneratedAt    time.Time
	SelectedSource string
	SelectedMode   string
	Sources        []AdminSourceView
	Modes          []AdminModeView
	RawItems       []model.RawItem
	ProcessedItems []model.ProcessedItem
	LLMSettings    AdminLLMSettingsView
	SourceForm     AdminSourceForm
	ModeForm       AdminModeForm
	NavItems       []AdminNavItem
}

type AdminNavItem struct {
	Label  string
	Href   string
	Active bool
}

type AdminText struct {
	PageTitle          string
	ControlPlane       string
	Subtitle           string
	GeneratedAt        string
	Sources            string
	SelectedFeed       string
	StoredItems        string
	QuickActions       string
	ActionsHint        string
	Source             string
	OpenSourceView     string
	RefreshSelected    string
	ReprocessRecent    string
	RefreshAll         string
	SourceStatus       string
	StatusHint         string
	JSON               string
	Mode               string
	State              string
	LastSuccess        string
	Counts             string
	Feed               string
	Enabled            string
	Disabled           string
	Error              string
	Healthy            string
	OpenFeed           string
	SelectedSource     string
	RecentRawItems     string
	RecentProcessed    string
	FeedOutput         string
	NoRawItems         string
	NoProcessed        string
	Summary            string
	ProcessedAt        string
	Model              string
	Fetched            string
	Processed          string
	RawProcessed       string
	Never              string
	RuntimeSettings    string
	SettingsHint       string
	Provider           string
	APIKey             string
	BaseURL            string
	Timeout            string
	SaveSettings       string
	CurrentKey         string
	ModesTitle         string
	ModesHint          string
	Name               string
	SystemPrompt       string
	TaskPrompt         string
	Temperature        string
	MaxOutputTokens    string
	SchemaName         string
	TitleField         string
	SummaryField       string
	ContentField       string
	ExtraFieldsJSON    string
	SaveMode           string
	SelectMode         string
	NewMode            string
	SourcesTitle       string
	SourcesHint        string
	URL                string
	RefreshInterval    string
	MaxItems           string
	MaxInputChars      string
	ExtractFullContent string
	SaveSource         string
	SelectSource       string
	NewSource          string
	Language           string
	Dashboard          string
	LLMSettingsTitle   string
}

type AdminLLMSettingsView struct {
	Provider     string
	Model        string
	APIKeyMasked string
	BaseURL      string
	Timeout      string
}

type AdminSourceView struct {
	ID                   string
	Name                 string
	URL                  string
	Mode                 string
	Enabled              bool
	MaxItems             int
	FeedURL              string
	LastSuccessLabel     string
	LastError            string
	LastFetchedCount     int
	LastProcessedCount   int
	LastReprocessedCount int
	RawItemCount         int
	ProcessedItemCount   int
}

type AdminModeView struct {
	Name            string
	SystemPrompt    string
	TaskPrompt      string
	Temperature     string
	MaxOutputTokens int
	SchemaName      string
	TitleField      string
	SummaryField    string
	ContentField    string
	ExtraFieldsJSON string
}

type AdminSourceForm struct {
	ID              string
	Name            string
	URL             string
	RefreshInterval string
	Enabled         bool
	MaxItems        int
	PipelineMode    string
	SystemPrompt    string
	TaskPrompt      string
	MaxInputChars   int
	ExtractFull     bool
	Temperature     string
	MaxOutputTokens int
}

type AdminModeForm struct {
	Name            string
	SystemPrompt    string
	TaskPrompt      string
	Temperature     string
	MaxOutputTokens int
	SchemaName      string
	TitleField      string
	SummaryField    string
	ContentField    string
	ExtraFieldsJSON string
}

func BuildAdminPageView(r *http.Request, section string, settings model.LLMSettings, sources []model.Source, modes []model.Mode, states []model.FeedState, rawItems []model.RawItem, processedItems []model.ProcessedItem, selectedSource, selectedMode, message, errText string) AdminPageView {
	lang := detectLanguage(r)
	text := textsFor(lang)

	stateByID := make(map[string]model.FeedState, len(states))
	for _, state := range states {
		stateByID[state.SourceID] = state
	}

	sourceViews := make([]AdminSourceView, 0, len(sources))
	for _, source := range sources {
		state := stateByID[source.ID]
		sourceViews = append(sourceViews, AdminSourceView{
			ID:                   source.ID,
			Name:                 fallback(source.Name, source.ID),
			URL:                  source.URL,
			Mode:                 source.PipelineMode,
			Enabled:              source.Enabled,
			MaxItems:             source.MaxItems,
			FeedURL:              absoluteFeedURL(r, source.ID),
			LastSuccessLabel:     formatTime(state.LastSuccessAt, text),
			LastError:            state.LastError,
			LastFetchedCount:     state.LastFetchedCount,
			LastProcessedCount:   state.LastProcessedCount,
			LastReprocessedCount: state.LastReprocessedCount,
			RawItemCount:         state.RawItemCount,
			ProcessedItemCount:   state.ProcessedItemCount,
		})
	}
	sort.Slice(sourceViews, func(i, j int) bool {
		return sourceViews[i].ID < sourceViews[j].ID
	})

	modeViews := make([]AdminModeView, 0, len(modes))
	for _, mode := range modes {
		modeViews = append(modeViews, AdminModeView{
			Name:            mode.Name,
			SystemPrompt:    mode.SystemPrompt,
			TaskPrompt:      mode.TaskPrompt,
			Temperature:     floatValue(mode.Temperature),
			MaxOutputTokens: mode.MaxOutputTokens,
			SchemaName:      mode.OutputSchema.Name,
			TitleField:      mode.OutputSchema.TitleField,
			SummaryField:    mode.OutputSchema.SummaryField,
			ContentField:    mode.OutputSchema.ContentField,
			ExtraFieldsJSON: extraFieldsJSON(mode.OutputSchema),
		})
	}
	sort.Slice(modeViews, func(i, j int) bool {
		return modeViews[i].Name < modeViews[j].Name
	})

	if selectedSource == "" && len(sourceViews) > 0 {
		selectedSource = sourceViews[0].ID
	}
	if selectedMode == "" && len(modeViews) > 0 {
		selectedMode = modeViews[0].Name
	}

	return AdminPageView{
		Lang:           lang,
		Text:           text,
		Section:        normalizeSection(section),
		Message:        message,
		Error:          errText,
		GeneratedAt:    time.Now(),
		SelectedSource: selectedSource,
		SelectedMode:   selectedMode,
		Sources:        sourceViews,
		Modes:          modeViews,
		RawItems:       rawItems,
		ProcessedItems: processedItems,
		LLMSettings: AdminLLMSettingsView{
			Provider:     settings.Provider,
			Model:        settings.Model,
			APIKeyMasked: maskAPIKey(settings.APIKey),
			BaseURL:      settings.BaseURL,
			Timeout:      settings.Timeout,
		},
		SourceForm: buildSourceForm(sources, selectedSource),
		ModeForm:   buildModeForm(modes, selectedMode),
		NavItems:   buildNavItems(r, text, selectedSource, selectedMode, normalizeSection(section)),
	}
}

func badgeVariant(enabled bool) badge.Variant {
	if enabled {
		return badge.VariantDefault
	}
	return badge.VariantSecondary
}

func errorBadgeVariant(hasError bool) badge.Variant {
	if hasError {
		return badge.VariantDestructive
	}
	return badge.VariantSecondary
}

func selectedSourceTitle(vm AdminPageView) string {
	for _, source := range vm.Sources {
		if source.ID == vm.SelectedSource {
			return source.Name
		}
	}
	return ""
}

func selectedSourceMode(vm AdminPageView) string {
	for _, source := range vm.Sources {
		if source.ID == vm.SelectedSource {
			return source.Mode
		}
	}
	return ""
}

func selectedSourceFeedURL(vm AdminPageView) string {
	for _, source := range vm.Sources {
		if source.ID == vm.SelectedSource {
			return source.FeedURL
		}
	}
	return ""
}

func selectedSourceStats(vm AdminPageView) string {
	for _, source := range vm.Sources {
		if source.ID == vm.SelectedSource {
			return fmt.Sprintf("%d / %d", source.RawItemCount, source.ProcessedItemCount)
		}
	}
	return ""
}

func navLinkClass(active bool) string {
	base := "rounded-full border px-3 py-1.5 text-sm"
	if active {
		return base + " border-primary/30 bg-primary/10 text-primary"
	}
	return base + " border-border bg-background/80 text-muted-foreground"
}

func langLinkClass(active bool) string {
	base := "rounded-full border px-3 py-1.5"
	if active {
		return base + " border-primary/30 bg-primary/10 text-primary"
	}
	return base + " border-border bg-background/80 text-muted-foreground"
}

func formatTime(ts time.Time, text AdminText) string {
	if ts.IsZero() {
		return text.Never
	}
	return ts.Local().Format("2006-01-02 15:04:05")
}

func shorten(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func absoluteFeedURL(r *http.Request, sourceID string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}

	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}

	path := "/feeds/" + url.PathEscape(sourceID) + ".rss"
	return scheme + "://" + host + path
}

func fallback(primary, secondary string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return strings.TrimSpace(secondary)
}

func detectLanguage(r *http.Request) string {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang"))) {
	case "zh":
		return "zh"
	case "en":
		return "en"
	}

	acceptLanguage := strings.ToLower(r.Header.Get("Accept-Language"))
	if strings.Contains(acceptLanguage, "zh") {
		return "zh"
	}
	return "en"
}

func textsFor(lang string) AdminText {
	if lang == "zh" {
		return AdminText{
			PageTitle:          "rss-gateway 管理页",
			ControlPlane:       "控制台",
			Subtitle:           "把状态查看、手动处理和运行时配置分开，避免一个页面塞下所有内容。",
			GeneratedAt:        "生成时间",
			Sources:            "订阅源",
			SelectedFeed:       "当前订阅",
			StoredItems:        "已存储条目",
			QuickActions:       "快捷操作",
			ActionsHint:        "一次操作一个 source。JSON 管理接口仍然保留在同名 `/admin/*` 路径下。",
			Source:             "订阅源",
			OpenSourceView:     "打开当前视图",
			RefreshSelected:    "刷新当前订阅",
			ReprocessRecent:    "重处理最近条目",
			RefreshAll:         "刷新全部已启用订阅",
			SourceStatus:       "订阅状态",
			StatusHint:         "当前状态来自 SQLite，并和当前运行时配置合并展示。",
			JSON:               "JSON",
			Mode:               "模式",
			State:              "状态",
			LastSuccess:        "上次成功",
			Counts:             "计数",
			Feed:               "输出",
			Enabled:            "已启用",
			Disabled:           "已禁用",
			Error:              "错误",
			Healthy:            "正常",
			OpenFeed:           "打开 feed",
			SelectedSource:     "当前订阅详情",
			RecentRawItems:     "最近原始条目",
			RecentProcessed:    "最近处理结果",
			FeedOutput:         "Feed 输出地址",
			NoRawItems:         "这个订阅目前还没有存储任何原始条目。",
			NoProcessed:        "这个订阅目前还没有处理结果。",
			Summary:            "摘要",
			ProcessedAt:        "处理时间",
			Model:              "模型",
			Fetched:            "抓取",
			Processed:          "处理",
			RawProcessed:       "原始/输出",
			Never:              "从未",
			RuntimeSettings:    "运行时配置",
			SettingsHint:       "这些配置保存在 SQLite 中，保存后立即生效。",
			Provider:           "Provider",
			APIKey:             "API Key",
			BaseURL:            "Base URL",
			Timeout:            "超时",
			SaveSettings:       "保存 LLM 配置",
			CurrentKey:         "当前 key",
			ModesTitle:         "Modes",
			ModesHint:          "编辑共享 prompt 和输出 schema。",
			Name:               "名称",
			SystemPrompt:       "System Prompt",
			TaskPrompt:         "Task Prompt",
			Temperature:        "Temperature",
			MaxOutputTokens:    "最大输出 Tokens",
			SchemaName:         "Schema 名称",
			TitleField:         "标题字段",
			SummaryField:       "摘要字段",
			ContentField:       "正文字段",
			ExtraFieldsJSON:    "额外字段 JSON",
			SaveMode:           "保存 Mode",
			SelectMode:         "选择 Mode",
			NewMode:            "新建 Mode",
			SourcesTitle:       "Sources",
			SourcesHint:        "编辑抓取源、刷新周期和 source 覆盖参数。",
			URL:                "URL",
			RefreshInterval:    "刷新周期",
			MaxItems:           "最大条目数",
			MaxInputChars:      "最大输入字符数",
			ExtractFullContent: "抓取链接正文",
			SaveSource:         "保存 Source",
			SelectSource:       "选择 Source",
			NewSource:          "新建 Source",
			Language:           "语言",
			Dashboard:          "仪表盘",
			LLMSettingsTitle:   "LLM 设置",
		}
	}

	return AdminText{
		PageTitle:          "rss-gateway admin",
		ControlPlane:       "Control Plane",
		Subtitle:           "Split status, actions, and runtime config into separate pages so the admin UI stays readable.",
		GeneratedAt:        "Generated",
		Sources:            "Sources",
		SelectedFeed:       "Selected Feed",
		StoredItems:        "Stored Items",
		QuickActions:       "Quick Actions",
		ActionsHint:        "Run against one source at a time. The JSON admin APIs stay available under the same /admin/* paths.",
		Source:             "Source",
		OpenSourceView:     "Open Source View",
		RefreshSelected:    "Refresh Selected Source",
		ReprocessRecent:    "Reprocess Recent Items",
		RefreshAll:         "Refresh All Enabled Sources",
		SourceStatus:       "Source Status",
		StatusHint:         "Current state from SQLite, merged with current runtime source config.",
		JSON:               "JSON",
		Mode:               "Mode",
		State:              "State",
		LastSuccess:        "Last Success",
		Counts:             "Counts",
		Feed:               "Feed",
		Enabled:            "enabled",
		Disabled:           "disabled",
		Error:              "error",
		Healthy:            "healthy",
		OpenFeed:           "open feed",
		SelectedSource:     "Selected Source",
		RecentRawItems:     "Recent raw items stored before LLM processing.",
		RecentProcessed:    "Recent processed items emitted to RSS.",
		FeedOutput:         "Feed Output",
		NoRawItems:         "No raw items stored yet for this source.",
		NoProcessed:        "No processed items yet for this source.",
		Summary:            "Summary",
		ProcessedAt:        "Processed At",
		Model:              "Model",
		Fetched:            "fetched",
		Processed:          "processed",
		RawProcessed:       "raw / processed",
		Never:              "never",
		RuntimeSettings:    "Runtime Settings",
		SettingsHint:       "These values live in SQLite and apply immediately after save.",
		Provider:           "Provider",
		APIKey:             "API Key",
		BaseURL:            "Base URL",
		Timeout:            "Timeout",
		SaveSettings:       "Save LLM Settings",
		CurrentKey:         "Current key",
		ModesTitle:         "Modes",
		ModesHint:          "Edit shared prompts and output schema.",
		Name:               "Name",
		SystemPrompt:       "System Prompt",
		TaskPrompt:         "Task Prompt",
		Temperature:        "Temperature",
		MaxOutputTokens:    "Max Output Tokens",
		SchemaName:         "Schema Name",
		TitleField:         "Title Field",
		SummaryField:       "Summary Field",
		ContentField:       "Content Field",
		ExtraFieldsJSON:    "Extra Fields JSON",
		SaveMode:           "Save Mode",
		SelectMode:         "Select Mode",
		NewMode:            "New Mode",
		SourcesTitle:       "Sources",
		SourcesHint:        "Edit feeds, refresh intervals, and source-level overrides.",
		URL:                "URL",
		RefreshInterval:    "Refresh Interval",
		MaxItems:           "Max Items",
		MaxInputChars:      "Max Input Chars",
		ExtractFullContent: "Extract Full Content",
		SaveSource:         "Save Source",
		SelectSource:       "Select Source",
		NewSource:          "New Source",
		Language:           "Language",
		Dashboard:          "Dashboard",
		LLMSettingsTitle:   "LLM Settings",
	}
}

func buildSourceForm(sources []model.Source, selected string) AdminSourceForm {
	for _, source := range sources {
		if source.ID == selected {
			return AdminSourceForm{
				ID:              source.ID,
				Name:            source.Name,
				URL:             source.URL,
				RefreshInterval: source.RefreshInterval.String(),
				Enabled:         source.Enabled,
				MaxItems:        source.MaxItems,
				PipelineMode:    source.PipelineMode,
				SystemPrompt:    source.SystemPrompt,
				TaskPrompt:      source.TaskPrompt,
				MaxInputChars:   source.MaxInputChars,
				ExtractFull:     source.ExtractFull,
				Temperature:     floatValue(source.Temperature),
				MaxOutputTokens: source.MaxOutputTokens,
			}
		}
	}
	return AdminSourceForm{
		Enabled:         true,
		RefreshInterval: "30m",
		MaxItems:        20,
		MaxInputChars:   8000,
	}
}

func buildModeForm(modes []model.Mode, selected string) AdminModeForm {
	for _, mode := range modes {
		if mode.Name == selected {
			return AdminModeForm{
				Name:            mode.Name,
				SystemPrompt:    mode.SystemPrompt,
				TaskPrompt:      mode.TaskPrompt,
				Temperature:     floatValue(mode.Temperature),
				MaxOutputTokens: mode.MaxOutputTokens,
				SchemaName:      mode.OutputSchema.Name,
				TitleField:      mode.OutputSchema.TitleField,
				SummaryField:    mode.OutputSchema.SummaryField,
				ContentField:    mode.OutputSchema.ContentField,
				ExtraFieldsJSON: extraFieldsJSON(mode.OutputSchema),
			}
		}
	}
	return AdminModeForm{
		SchemaName:   "rss_output",
		TitleField:   "title",
		SummaryField: "summary",
		ContentField: "content",
	}
}

func extraFieldsJSON(schema model.OutputSchema) string {
	extras := make([]model.OutputField, 0)
	for _, field := range schema.Fields {
		if field.Name == schema.TitleField || field.Name == schema.SummaryField || field.Name == schema.ContentField {
			continue
		}
		extras = append(extras, field)
	}
	data, err := json.MarshalIndent(extras, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(data)
}

func floatValue(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%g", *value)
}

func maskAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "******"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func buildNavItems(r *http.Request, text AdminText, selectedSource, selectedMode, section string) []AdminNavItem {
	return []AdminNavItem{
		{Label: text.Dashboard, Href: adminURL("/admin", selectedSource, selectedMode, detectLanguage(r)), Active: section == AdminSectionDashboard},
		{Label: text.LLMSettingsTitle, Href: adminURL("/admin/settings/llm", selectedSource, selectedMode, detectLanguage(r)), Active: section == AdminSectionLLM},
		{Label: text.ModesTitle, Href: adminURL("/admin/modes", selectedSource, selectedMode, detectLanguage(r)), Active: section == AdminSectionModes},
		{Label: text.SourcesTitle, Href: adminURL("/admin/sources", selectedSource, selectedMode, detectLanguage(r)), Active: section == AdminSectionSources},
	}
}

func navPath(vm AdminPageView) string {
	switch vm.Section {
	case AdminSectionLLM:
		return "/admin/settings/llm"
	case AdminSectionModes:
		return "/admin/modes"
	case AdminSectionSources:
		return "/admin/sources"
	default:
		return "/admin"
	}
}

func adminURL(path, selectedSource, selectedMode, lang string) string {
	values := url.Values{}
	if selectedSource != "" {
		values.Set("source", selectedSource)
	}
	if selectedMode != "" {
		values.Set("mode", selectedMode)
	}
	if lang != "" {
		values.Set("lang", lang)
	}
	if encoded := values.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func normalizeSection(section string) string {
	switch section {
	case AdminSectionLLM, AdminSectionModes, AdminSectionSources:
		return section
	default:
		return AdminSectionDashboard
	}
}
