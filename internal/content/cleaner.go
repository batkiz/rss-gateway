package content

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var whitespaceRE = regexp.MustCompile(`\s+`)

func ExtractText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if !strings.Contains(raw, "<") || !strings.Contains(raw, ">") {
		return normalizeWhitespace(raw)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return normalizeWhitespace(raw)
	}

	doc.Find("script,style,noscript,svg,form,nav,header,footer,aside,iframe").Each(func(_ int, sel *goquery.Selection) {
		sel.Remove()
	})

	var buf bytes.Buffer
	renderSelectionText(&buf, doc.Selection)
	return normalizeWhitespace(buf.String())
}

func ExtractReadableHTML(body io.Reader) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return "", "", err
	}

	candidates := []string{"article", "main", ".post", ".entry-content", ".article-content", "body"}
	var selection *goquery.Selection
	for _, candidate := range candidates {
		selection = doc.Find(candidate).First()
		if selection.Length() > 0 {
			break
		}
	}
	if selection == nil || selection.Length() == 0 {
		selection = doc.Selection
	}

	selection.Find("script,style,noscript,svg,form,nav,header,footer,aside,iframe").Each(func(_ int, sel *goquery.Selection) {
		sel.Remove()
	})

	htmlText, err := selection.Html()
	if err != nil {
		htmlText = ""
	}

	var buf bytes.Buffer
	renderSelectionText(&buf, selection)
	return strings.TrimSpace(htmlText), normalizeWhitespace(buf.String()), nil
}

func renderSelectionText(buf *bytes.Buffer, selection *goquery.Selection) {
	for _, node := range selection.Nodes {
		renderNodeText(buf, node)
	}
}

func renderNodeText(buf *bytes.Buffer, node *html.Node) {
	if node.Type == html.TextNode {
		buf.WriteString(node.Data)
		buf.WriteString(" ")
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		renderNodeText(buf, child)
	}
	if node.Type == html.ElementNode {
		switch node.Data {
		case "p", "div", "section", "article", "li", "br", "h1", "h2", "h3", "h4":
			buf.WriteString("\n")
		}
	}
}

func normalizeWhitespace(value string) string {
	value = html.UnescapeString(value)
	value = whitespaceRE.ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}
