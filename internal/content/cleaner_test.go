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
