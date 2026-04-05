package model

import "time"

type Source struct {
	ID              string
	Name            string
	URL             string
	RefreshInterval time.Duration
	Enabled         bool
	MaxItems        int
	PipelineMode    string
	SystemPrompt    string
	TaskPrompt      string
	MaxInputChars   int
	ExtractFull     bool
	Temperature     *float64
	MaxOutputTokens int
}

type LLMSettings struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
	Timeout  string
}

type Mode struct {
	Name            string
	SystemPrompt    string
	TaskPrompt      string
	Temperature     *float64
	MaxOutputTokens int
	OutputSchema    OutputSchema
}

type RawItem struct {
	SourceID    string
	GUID        string
	Title       string
	Link        string
	Description string
	Content     string
	ContentHTML string
	ContentText string
	Author      string
	PublishedAt time.Time
	Hash        string
	FetchedAt   time.Time
}

type ProcessRequest struct {
	Mode            string
	Title           string
	Link            string
	Content         string
	SystemPrompt    string
	TaskPrompt      string
	MaxInputChars   int
	Temperature     *float64
	MaxOutputTokens int
	OutputSchema    OutputSchema
}

type ProcessResponse struct {
	Title      string
	Summary    string
	Content    string
	Model      string
	OutputJSON string
}

type ProcessedItem struct {
	SourceID      string
	GUID          string
	OriginalTitle string
	OriginalLink  string
	PublishedAt   time.Time
	OutputTitle   string
	OutputSummary string
	OutputContent string
	OutputJSON    string
	Model         string
	InputHash     string
	ProcessedAt   time.Time
}

type OutputSchema struct {
	Name         string
	TitleField   string
	SummaryField string
	ContentField string
	Fields       []OutputField
}

type OutputField struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

type FeedState struct {
	SourceID             string    `json:"source_id"`
	LastSuccessAt        time.Time `json:"last_success_at"`
	LastError            string    `json:"last_error"`
	LastFetchedCount     int       `json:"last_fetched_count"`
	LastProcessedCount   int       `json:"last_processed_count"`
	LastReprocessedCount int       `json:"last_reprocessed_count"`
	RawItemCount         int       `json:"raw_item_count"`
	ProcessedItemCount   int       `json:"processed_item_count"`
}
