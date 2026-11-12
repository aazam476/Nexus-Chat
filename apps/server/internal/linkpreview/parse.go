package linkpreview

import (
	"io"
	"net/url"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// Limits on what a preview carries.
const (
	maximumTitle       = 300
	maximumDescription = 1000
)

// Page is what a preview shows of a page.
type Page struct {
	Title       string
	Description string
	// ImageURL is absolute, or empty.
	ImageURL string
}

// meta collects the tags a preview uses, by their property or name.
type meta map[string]string

func attribute(token html.Token, name string) string {
	for _, attribute := range token.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val
		}
	}
	return ""
}

// headReader collects the head's meta tags and title as tokens arrive.
type headReader struct {
	tags    meta
	title   strings.Builder
	inTitle bool
}

// start handles an opening tag; it reports false once the body begins.
func (reader *headReader) start(token html.Token) bool {
	switch token.Data {
	case "meta":
		key := strings.ToLower(attribute(token, "property") + attribute(token, "name"))
		if _, seen := reader.tags[key]; !seen && key != "" {
			reader.tags[key] = attribute(token, "content")
		}
	case "title":
		reader.inTitle = true
	case "body":
		return false
	}
	return true
}

// end handles a closing tag; it reports false once the head ends.
func (reader *headReader) end(token html.Token) bool {
	if token.Data == "title" {
		reader.inTitle = false
	}
	return token.Data != "head"
}

// readHead reads tags until the head ends; the body never holds them.
func readHead(body io.Reader) (meta, string) {
	tokenizer := html.NewTokenizer(body)
	reader := &headReader{tags: meta{}}
	for more := true; more; {
		switch tokenizer.Next() {
		case html.ErrorToken:
			more = false
		case html.StartTagToken, html.SelfClosingTagToken:
			more = reader.start(tokenizer.Token())
		case html.EndTagToken:
			more = reader.end(tokenizer.Token())
		case html.TextToken:
			if reader.inTitle {
				reader.title.Write(tokenizer.Text())
			}
		}
	}
	return reader.tags, reader.title.String()
}

func first(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// shorten cuts text to limit characters, on a character boundary.
func shorten(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit-1]) + "…"
}

// parsePage reads a page's preview from its HTML, resolving the image
// against base. Only http and https images are kept.
func parsePage(body io.Reader, base *url.URL) Page {
	tags, title := readHead(body)
	page := Page{
		Title:       shorten(first(tags["og:title"], tags["twitter:title"], title), maximumTitle),
		Description: shorten(first(tags["og:description"], tags["twitter:description"], tags["description"]), maximumDescription),
	}
	if image := first(tags["og:image"], tags["og:image:url"], tags["twitter:image"]); image != "" {
		if resolved, err := base.Parse(image); err == nil && (resolved.Scheme == "https" || resolved.Scheme == "http") {
			page.ImageURL = resolved.String()
		}
	}
	return page
}
