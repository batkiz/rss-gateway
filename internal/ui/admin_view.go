package ui

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/templui/templui/components/badge"

	"github.com/batkiz/rss-gateway/internal/model"
)

type AdminPageView struct {
	Lang           string
	Text           AdminText
	Message        string
	Error          string
	GeneratedAt    time.Time
	SelectedSource string
	Sources        []AdminSourceView
	RawItems       []model.RawItem
	ProcessedItems []model.ProcessedItem
}

type AdminText struct {
	PageTitle       string
	ControlPlane    string
	Subtitle        string
	GeneratedAt     string
	Sources         string
	SelectedFeed    string
	StoredItems     string
	QuickActions    string
	ActionsHint     string
	Source          string
	OpenSourceView  string
	RefreshSelected string
	ReprocessRecent string
	RefreshAll      string
	SourceStatus    string
	StatusHint      string
	JSON            string
	Mode            string
	State           string
	LastSuccess     string
	Counts          string
	Feed            string
	Enabled         string
	Disabled        string
	Error           string
	Healthy         string
	OpenFeed        string
	SelectedSource  string
	RecentRawItems  string
	RecentProcessed string
	FeedOutput      string
	NoRawItems      string
	NoProcessed     string
	Summary         string
	ProcessedAt     string
	Model           string
	Fetched         string
	Processed       string
	RawProcessed    string
	Never           string
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

func BuildAdminPageView(r *http.Request, sources map[string]model.Source, states []model.FeedState, rawItems []model.RawItem, processedItems []model.ProcessedItem, selectedSource, message, errText string) AdminPageView {
	lang := detectLanguage(r)
	text := textsFor(lang)

	stateByID := make(map[string]model.FeedState, len(states))
	for _, state := range states {
		stateByID[state.SourceID] = state
	}

	sourceViews := make([]AdminSourceView, 0, len(sources))
	for id, source := range sources {
		state := stateByID[id]
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

	if selectedSource == "" && len(sourceViews) > 0 {
		selectedSource = sourceViews[0].ID
	}

	return AdminPageView{
		Lang:           lang,
		Text:           text,
		Message:        message,
		Error:          errText,
		GeneratedAt:    time.Now(),
		SelectedSource: selectedSource,
		Sources:        sourceViews,
		RawItems:       rawItems,
		ProcessedItems: processedItems,
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

func countsLabel(vm AdminPageView, source AdminSourceView) string {
	return fmt.Sprintf("%d %s, %d %s, %d/%d", source.LastFetchedCount, strings.ToLower(vm.Text.Fetched), source.LastProcessedCount, strings.ToLower(vm.Text.Processed), source.RawItemCount, source.ProcessedItemCount)
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
			PageTitle:       "rss-gateway 管理页",
			ControlPlane:    "控制台",
			Subtitle:        "一个保持简单的管理页面，用于查看 feed 状态、手动刷新、重处理，以及检查输出。",
			GeneratedAt:     "生成时间",
			Sources:         "订阅源",
			SelectedFeed:    "当前订阅",
			StoredItems:     "已存储条目",
			QuickActions:    "快捷操作",
			ActionsHint:     "一次操作一个 source。JSON 管理接口仍然保留在同名 `/admin/*` 路径下。",
			Source:          "订阅源",
			OpenSourceView:  "打开当前视图",
			RefreshSelected: "刷新当前订阅",
			ReprocessRecent: "重处理最近条目",
			RefreshAll:      "刷新全部已启用订阅",
			SourceStatus:    "订阅状态",
			StatusHint:      "当前状态来自 SQLite，并和配置中的 source 元数据合并展示。",
			JSON:            "JSON",
			Mode:            "模式",
			State:           "状态",
			LastSuccess:     "上次成功",
			Counts:          "计数",
			Feed:            "输出",
			Enabled:         "已启用",
			Disabled:        "已禁用",
			Error:           "错误",
			Healthy:         "正常",
			OpenFeed:        "打开 feed",
			SelectedSource:  "当前订阅详情",
			RecentRawItems:  "最近原始条目",
			RecentProcessed: "最近处理结果",
			FeedOutput:      "Feed 输出地址",
			NoRawItems:      "这个订阅目前还没有存储任何原始条目。",
			NoProcessed:     "这个订阅目前还没有处理结果。",
			Summary:         "摘要",
			ProcessedAt:     "处理时间",
			Model:           "模型",
			Fetched:         "抓取",
			Processed:       "处理",
			RawProcessed:    "原始/输出",
			Never:           "从未",
		}
	}

	return AdminText{
		PageTitle:       "rss-gateway admin",
		ControlPlane:    "Control Plane",
		Subtitle:        "A simple admin surface for feed status, manual refresh, reprocessing, and output inspection.",
		GeneratedAt:     "Generated",
		Sources:         "Sources",
		SelectedFeed:    "Selected Feed",
		StoredItems:     "Stored Items",
		QuickActions:    "Quick Actions",
		ActionsHint:     "Run against one source at a time. The JSON admin APIs stay available under the same /admin/* paths.",
		Source:          "Source",
		OpenSourceView:  "Open Source View",
		RefreshSelected: "Refresh Selected Source",
		ReprocessRecent: "Reprocess Recent Items",
		RefreshAll:      "Refresh All Enabled Sources",
		SourceStatus:    "Source Status",
		StatusHint:      "Current state from SQLite, merged with configured source metadata.",
		JSON:            "JSON",
		Mode:            "Mode",
		State:           "State",
		LastSuccess:     "Last Success",
		Counts:          "Counts",
		Feed:            "Feed",
		Enabled:         "enabled",
		Disabled:        "disabled",
		Error:           "error",
		Healthy:         "healthy",
		OpenFeed:        "open feed",
		SelectedSource:  "Selected Source",
		RecentRawItems:  "Recent raw items stored before LLM processing.",
		RecentProcessed: "Recent processed items emitted to RSS.",
		FeedOutput:      "Feed Output",
		NoRawItems:      "No raw items stored yet for this source.",
		NoProcessed:     "No processed items yet for this source.",
		Summary:         "Summary",
		ProcessedAt:     "Processed At",
		Model:           "Model",
		Fetched:         "fetched",
		Processed:       "processed",
		RawProcessed:    "raw / processed",
		Never:           "never",
	}
}
