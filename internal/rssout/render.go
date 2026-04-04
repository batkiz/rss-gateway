package rssout

import (
	"encoding/xml"
	"time"

	"github.com/batkiz/rss-gateway/internal/model"
)

type rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel channel  `xml:"channel"`
}

type channel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	LastBuild   string    `xml:"lastBuildDate"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

func RenderFeed(title, link, description string, items []model.ProcessedItem) ([]byte, error) {
	out := rss{
		Version: "2.0",
		Channel: channel{
			Title:       title,
			Link:        link,
			Description: description,
			LastBuild:   time.Now().UTC().Format(time.RFC1123Z),
			Items:       make([]rssItem, 0, len(items)),
		},
	}

	for _, item := range items {
		description := item.OutputContent
		if description == "" {
			description = item.OutputSummary
		}
		out.Channel.Items = append(out.Channel.Items, rssItem{
			Title:       item.OutputTitle,
			Link:        item.OriginalLink,
			GUID:        item.SourceID + ":" + item.GUID,
			PubDate:     item.PublishedAt.UTC().Format(time.RFC1123Z),
			Description: description,
		})
	}

	data, err := xml.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}
