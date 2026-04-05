package content

import (
	"bytes"
	"io"
	"math"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var whitespaceRE = regexp.MustCompile(`\s+`)

var noiseSelectors = []string{
	"script", "style", "noscript", "svg", "form", "nav", "header", "footer", "aside", "iframe",
	".comments", ".comment", ".comment-list", ".share", ".sharing", ".social", ".related",
	".advert", ".ads", ".promo", ".newsletter", ".sidebar", ".breadcrumbs", ".breadcrumb",
	"[aria-hidden='true']", "[hidden]", ".sr-only",
}

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

	cleanDocument(doc)

	var buf bytes.Buffer
	renderSelectionText(&buf, doc.Selection)
	return normalizeWhitespace(buf.String())
}

func ExtractReadableHTML(body io.Reader) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return "", "", err
	}

	cleanDocument(doc)
	selection := selectReadableRoot(doc)
	if selection == nil || selection.Length() == 0 {
		selection = doc.Find("body").First()
	}
	if selection == nil || selection.Length() == 0 {
		selection = doc.Selection
	}

	htmlText, err := selection.Html()
	if err != nil {
		htmlText = ""
	}

	var buf bytes.Buffer
	renderSelectionText(&buf, selection)
	text := normalizeWhitespace(buf.String())

	if len(text) < 160 {
		bodySelection := doc.Find("body").First()
		if bodySelection.Length() > 0 {
			var fallback bytes.Buffer
			renderSelectionText(&fallback, bodySelection)
			fallbackText := normalizeWhitespace(fallback.String())
			if len(fallbackText) > len(text) {
				text = fallbackText
				if fallbackHTML, err := bodySelection.Html(); err == nil {
					htmlText = fallbackHTML
				}
			}
		}
	}

	return strings.TrimSpace(htmlText), text, nil
}

func cleanDocument(doc *goquery.Document) {
	for _, selector := range noiseSelectors {
		doc.Find(selector).Each(func(_ int, sel *goquery.Selection) {
			sel.Remove()
		})
	}
	doc.Find("[class], [id]").Each(func(_ int, sel *goquery.Selection) {
		classValue, _ := sel.Attr("class")
		idValue, _ := sel.Attr("id")
		combined := strings.ToLower(classValue + " " + idValue)
		if hasNoiseToken(combined, "comments", "comment-list", "share", "sharing", "footer", "sidebar", "breadcrumbs", "breadcrumb") {
			sel.Remove()
		}
	})
}

func hasNoiseToken(value string, tokens ...string) bool {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		default:
			return true
		}
	})
	for _, part := range parts {
		for _, token := range tokens {
			if part == token {
				return true
			}
		}
	}
	return false
}

func selectReadableRoot(doc *goquery.Document) *goquery.Selection {
	candidates := []*goquery.Selection{
		doc.Find("article").First(),
		doc.Find("main").First(),
		doc.Find("[role='main']").First(),
	}

	doc.Find("section, div").Each(func(i int, sel *goquery.Selection) {
		if i < 80 {
			candidates = append(candidates, sel)
		}
	})

	bestScore := math.Inf(-1)
	var best *goquery.Selection
	seen := map[*html.Node]struct{}{}
	for _, candidate := range candidates {
		if candidate == nil || candidate.Length() == 0 || len(candidate.Nodes) == 0 {
			continue
		}
		if _, ok := seen[candidate.Nodes[0]]; ok {
			continue
		}
		seen[candidate.Nodes[0]] = struct{}{}
		score := readabilityScore(candidate)
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	return best
}

func readabilityScore(sel *goquery.Selection) float64 {
	var buf bytes.Buffer
	renderSelectionText(&buf, sel)
	text := normalizeWhitespace(buf.String())
	if len(text) == 0 {
		return math.Inf(-1)
	}

	paragraphs := sel.Find("p").Length()
	links := sel.Find("a").Length()
	headings := sel.Find("h1, h2, h3").Length()
	textLen := float64(len(text))
	linkDensity := float64(links) / math.Max(1, float64(paragraphs+1))
	score := textLen
	score += float64(paragraphs) * 120
	score += float64(headings) * 40
	score -= linkDensity * 140
	if textLen < 200 {
		score -= 250
	}
	return score
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
		case "p", "div", "section", "article", "li", "br", "h1", "h2", "h3", "h4", "blockquote", "pre":
			buf.WriteString("\n")
		}
	}
}

func normalizeWhitespace(value string) string {
	value = html.UnescapeString(value)
	value = whitespaceRE.ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}
