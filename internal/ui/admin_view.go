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
	Message        string
	Error          string
	GeneratedAt    time.Time
	SelectedSource string
	Sources        []AdminSourceView
	RawItems       []model.RawItem
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

func BuildAdminPageView(r *http.Request, sources map[string]model.Source, states []model.FeedState, rawItems []model.RawItem, selectedSource, message, errText string) AdminPageView {
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
			LastSuccessLabel:     formatTime(state.LastSuccessAt),
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
		Message:        message,
		Error:          errText,
		GeneratedAt:    time.Now(),
		SelectedSource: selectedSource,
		Sources:        sourceViews,
		RawItems:       rawItems,
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
			return fmt.Sprintf("%d raw / %d processed", source.RawItemCount, source.ProcessedItemCount)
		}
	}
	return ""
}

func formatTime(ts time.Time) string {
	if ts.IsZero() {
		return "never"
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
