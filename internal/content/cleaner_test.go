package content

import (
	"strings"
	"testing"
)

func TestExtractTextStripsMarkup(t *testing.T) {
	input := `<article><h1>Hello</h1><p>World <strong>RSS</strong></p><script>alert(1)</script></article>`
	got := ExtractText(input)
	if got != "Hello World RSS" {
		t.Fatalf("unexpected text: %q", got)
	}
}

func TestExtractReadableHTML(t *testing.T) {
	html := `<html><body><article><p>Alpha</p><p>Beta</p></article><footer>Ignore</footer></body></html>`
	raw, text, err := ExtractReadableHTML(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ExtractReadableHTML error: %v", err)
	}
	if !strings.Contains(raw, "Alpha") {
		t.Fatalf("expected raw article html, got %q", raw)
	}
	if text != "Alpha Beta" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestExtractReadableHTMLPrefersMainContentOverSidebar(t *testing.T) {
	html := `<html><body><div class="sidebar"><p>Link</p><p>Nav</p></div><div class="post"><h1>Title</h1><p>Long paragraph one.</p><p>Long paragraph two with details.</p></div></body></html>`
	_, text, err := ExtractReadableHTML(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ExtractReadableHTML error: %v", err)
	}
	if !strings.Contains(text, "Long paragraph two") {
		t.Fatalf("expected main content in extracted text, got %q", text)
	}
	if strings.Contains(text, "Link Nav") {
		t.Fatalf("unexpected sidebar text in extracted text: %q", text)
	}
}
