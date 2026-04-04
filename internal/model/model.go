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
}

type RawItem struct {
	SourceID    string
	GUID        string
	Title       string
	Link        string
	Description string
	Content     string
	Author      string
	PublishedAt time.Time
	Hash        string
}

type ProcessRequest struct {
	Mode          string
	Title         string
	Link          string
	Content       string
	SystemPrompt  string
	TaskPrompt    string
	MaxInputChars int
}

type ProcessResponse struct {
	Title   string
	Summary string
	Content string
	Model   string
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
	Model         string
	ProcessedAt   time.Time
}
