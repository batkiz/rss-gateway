package ui

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/batkiz/rss-gateway/internal/model"
)

type ItemPageView struct {
	Lang      string
	Text      ItemText
	NavItems  []AdminNavItem
	SourceID  string
	GUID      string
	Message   string
	Error     string
	Raw       model.RawItem
	Processed *model.ProcessedItem
	Preview   *model.ItemProcessPreview
	Form      ItemPromptForm
}

type ItemPromptForm struct {
	Mode            string
	SystemPrompt    string
	TaskPrompt      string
	MaxInputChars   int
	Temperature     string
	MaxOutputTokens int
}

type ItemText struct {
	Title            string
	BackToDashboard  string
	Source           string
	GUID             string
	PublishedAt      string
	RawContent       string
	RawHTML          string
	ProcessedOutput  string
	NoProcessed      string
	PromptLab        string
	Mode             string
	SystemPrompt     string
	TaskPrompt       string
	Temperature      string
	MaxOutputTokens  string
	MaxInputChars    string
	Preview          string
	ReprocessAndSave string
	PreviewOutput    string
	Model            string
	Summary          string
	Content          string
	Request          string
	OutputJSON       string
}

func BuildItemPageView(r *http.Request, source model.Source, raw model.RawItem, processed *model.ProcessedItem, preview *model.ItemProcessPreview, modes []model.Mode, message, errText string) ItemPageView {
	lang := detectLanguage(r)
	text := itemTexts(lang)
	form := ItemPromptForm{
		Mode:          source.PipelineMode,
		SystemPrompt:  source.SystemPrompt,
		TaskPrompt:    source.TaskPrompt,
		MaxInputChars: source.MaxInputChars,
	}
	if source.Temperature != nil {
		form.Temperature = floatValue(source.Temperature)
	}
	form.MaxOutputTokens = source.MaxOutputTokens
	if preview != nil {
		form.Mode = preview.Request.Mode
		form.SystemPrompt = preview.Request.SystemPrompt
		form.TaskPrompt = preview.Request.TaskPrompt
		form.MaxInputChars = preview.Request.MaxInputChars
		form.Temperature = floatValue(preview.Request.Temperature)
		form.MaxOutputTokens = preview.Request.MaxOutputTokens
	} else {
		for _, mode := range modes {
			if mode.Name == source.PipelineMode {
				if form.SystemPrompt == "" {
					form.SystemPrompt = mode.SystemPrompt
				}
				if form.TaskPrompt == "" {
					form.TaskPrompt = mode.TaskPrompt
				}
				if form.Temperature == "" {
					form.Temperature = floatValue(mode.Temperature)
				}
				if form.MaxOutputTokens == 0 {
					form.MaxOutputTokens = mode.MaxOutputTokens
				}
				break
			}
		}
	}

	return ItemPageView{
		Lang:      lang,
		Text:      text,
		NavItems:  buildNavItems(r, textsFor(lang), source.ID, source.PipelineMode, AdminSectionDashboard),
		SourceID:  source.ID,
		GUID:      raw.GUID,
		Message:   message,
		Error:     errText,
		Raw:       raw,
		Processed: processed,
		Preview:   preview,
		Form:      form,
	}
}

func itemLink(sourceID, guid, lang string) string {
	values := url.Values{}
	values.Set("source", sourceID)
	values.Set("guid", guid)
	if lang != "" {
		values.Set("lang", lang)
	}
	return "/items?" + values.Encode()
}

func itemTexts(lang string) ItemText {
	if lang == "zh" {
		return ItemText{
			Title:            "单条条目",
			BackToDashboard:  "返回仪表盘",
			Source:           "订阅源",
			GUID:             "GUID",
			PublishedAt:      "发布时间",
			RawContent:       "原始内容",
			RawHTML:          "原始 HTML",
			ProcessedOutput:  "已保存处理结果",
			NoProcessed:      "这条 item 还没有保存处理结果。",
			PromptLab:        "Prompt 调试",
			Mode:             "模式",
			SystemPrompt:     "System Prompt",
			TaskPrompt:       "Task Prompt",
			Temperature:      "Temperature",
			MaxOutputTokens:  "最大输出 Tokens",
			MaxInputChars:    "最大输入字符数",
			Preview:          "仅预览",
			ReprocessAndSave: "重跑并保存",
			PreviewOutput:    "预览输出",
			Model:            "模型",
			Summary:          "摘要",
			Content:          "正文",
			Request:          "请求参数",
			OutputJSON:       "原始 JSON",
		}
	}
	return ItemText{
		Title:            "Item",
		BackToDashboard:  "Back to dashboard",
		Source:           "Source",
		GUID:             "GUID",
		PublishedAt:      "Published At",
		RawContent:       "Raw Content",
		RawHTML:          "Raw HTML",
		ProcessedOutput:  "Saved Processed Output",
		NoProcessed:      "This item has no saved processed result yet.",
		PromptLab:        "Prompt Lab",
		Mode:             "Mode",
		SystemPrompt:     "System Prompt",
		TaskPrompt:       "Task Prompt",
		Temperature:      "Temperature",
		MaxOutputTokens:  "Max Output Tokens",
		MaxInputChars:    "Max Input Chars",
		Preview:          "Preview Only",
		ReprocessAndSave: "Reprocess and Save",
		PreviewOutput:    "Preview Output",
		Model:            "Model",
		Summary:          "Summary",
		Content:          "Content",
		Request:          "Request",
		OutputJSON:       "Raw JSON",
	}
}

func itemMetaTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

func intValue(value int) string {
	return fmt.Sprintf("%d", value)
}
