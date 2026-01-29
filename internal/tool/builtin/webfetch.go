package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"devorch/internal/tool"

	"golang.org/x/net/html"
)

const (
	webfetchMaxSize    = 1024 * 1024 // 1MB
	webfetchTimeout    = 30 * time.Second
	webfetchMaxContent = 50000 // characters
)

// WebFetchParams defines parameters for web page fetching
type WebFetchParams struct {
	URL      string `json:"url"`                // URL to fetch
	Selector string `json:"selector,omitempty"` // CSS selector to extract (optional)
	Format   string `json:"format,omitempty"`   // "text", "html", "markdown" (default: text)
}

// WebFetchTool fetches and extracts content from web pages
type WebFetchTool struct {
	// UserAgent for HTTP requests
	UserAgent string

	// Timeout for HTTP requests
	Timeout time.Duration

	// AllowedDomains restricts fetching to specific domains (empty = all)
	AllowedDomains []string
}

// Info returns tool metadata
func (t *WebFetchTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "webfetch",
		Description: `Fetch content from a web page.
Retrieves the main text content from a URL.
Useful for reading documentation, articles, or API references.

Best for:
- Reading documentation pages
- Fetching API references
- Getting content from articles

Note: JavaScript-rendered content may not be available.
For dynamic sites, consider using websearch instead.

Examples:
  - {"url": "https://example.com/docs"}
  - {"url": "https://example.com/api", "format": "text"}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "URL to fetch content from"
				},
				"selector": {
					"type": "string",
					"description": "CSS selector to extract specific content (optional)"
				},
				"format": {
					"type": "string",
					"enum": ["text", "html", "markdown"],
					"description": "Output format (default: text)"
				}
			},
			"required": ["url"]
		}`),
	}
}

// Execute fetches web page content
func (t *WebFetchTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p WebFetchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate URL
	parsedURL, err := url.Parse(p.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("only http/https URLs are supported")
	}

	// Check allowed domains
	if len(t.AllowedDomains) > 0 {
		allowed := false
		for _, domain := range t.AllowedDomains {
			if strings.HasSuffix(parsedURL.Host, domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("domain not allowed: %s", parsedURL.Host)
		}
	}

	// Set default format
	format := p.Format
	if format == "" {
		format = "text"
	}

	// Create HTTP client
	timeout := t.Timeout
	if timeout == 0 {
		timeout = webfetchTimeout
	}

	httpCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "GET", p.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	userAgent := t.UserAgent
	if userAgent == "" {
		userAgent = "DevOrch/1.0 (AI Agent Web Fetcher)"
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// Make request
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Read body with size limit
	limitedReader := io.LimitReader(resp.Body, webfetchMaxSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		// Return raw content if HTML parsing fails
		content := string(body)
		if len(content) > webfetchMaxContent {
			content = content[:webfetchMaxContent] + "\n\n[Content truncated...]"
		}
		return &tool.Result{
			Output: content,
			Title:  fmt.Sprintf("webfetch: %s", parsedURL.Host),
		}, nil
	}

	// Extract content
	var content string
	switch format {
	case "html":
		content = string(body)
	case "markdown":
		content = t.extractMarkdown(doc)
	default:
		content = t.extractText(doc)
	}

	// Truncate if needed
	truncated := false
	if len(content) > webfetchMaxContent {
		content = content[:webfetchMaxContent] + "\n\n[Content truncated...]"
		truncated = true
	}

	return &tool.Result{
		Output:    content,
		Title:     fmt.Sprintf("webfetch: %s", parsedURL.Host),
		Truncated: truncated,
		Metadata: map[string]any{
			"url":         p.URL,
			"format":      format,
			"content_len": len(content),
			"truncated":   truncated,
		},
	}, nil
}

// extractText extracts plain text from HTML
func (t *WebFetchTool) extractText(doc *html.Node) string {
	var sb strings.Builder
	var skipTags = map[string]bool{
		"script": true, "style": true, "noscript": true,
		"nav": true, "header": true, "footer": true,
	}

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if skipTags[n.Data] {
				return
			}
			// Add newline for block elements
			if isBlockElement(n.Data) {
				sb.WriteString("\n")
			}
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}

		if n.Type == html.ElementNode && isBlockElement(n.Data) {
			sb.WriteString("\n")
		}
	}

	extract(doc)

	// Clean up whitespace
	content := sb.String()
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")
	content = regexp.MustCompile(` +`).ReplaceAllString(content, " ")
	content = strings.TrimSpace(content)

	return content
}

// extractMarkdown converts HTML to simple markdown
func (t *WebFetchTool) extractMarkdown(doc *html.Node) string {
	var sb strings.Builder
	var skipTags = map[string]bool{
		"script": true, "style": true, "noscript": true,
		"nav": true, "header": true, "footer": true,
	}

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if skipTags[n.Data] {
				return
			}

			switch n.Data {
			case "h1":
				sb.WriteString("\n# ")
			case "h2":
				sb.WriteString("\n## ")
			case "h3":
				sb.WriteString("\n### ")
			case "h4", "h5", "h6":
				sb.WriteString("\n#### ")
			case "p":
				sb.WriteString("\n\n")
			case "br":
				sb.WriteString("\n")
			case "li":
				sb.WriteString("\n- ")
			case "code":
				sb.WriteString("`")
			case "pre":
				sb.WriteString("\n```\n")
			case "strong", "b":
				sb.WriteString("**")
			case "em", "i":
				sb.WriteString("*")
			case "a":
				// Handle links
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						sb.WriteString("[")
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							extract(c)
						}
						sb.WriteString("](")
						sb.WriteString(attr.Val)
						sb.WriteString(")")
						return
					}
				}
			}
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				sb.WriteString("\n")
			case "code":
				sb.WriteString("`")
			case "pre":
				sb.WriteString("\n```\n")
			case "strong", "b":
				sb.WriteString("**")
			case "em", "i":
				sb.WriteString("*")
			}
		}
	}

	extract(doc)

	// Clean up
	content := sb.String()
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")
	content = strings.TrimSpace(content)

	return content
}

func isBlockElement(tag string) bool {
	blockTags := map[string]bool{
		"div": true, "p": true, "h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true, "ul": true, "ol": true,
		"li": true, "table": true, "tr": true, "section": true,
		"article": true, "main": true, "aside": true, "pre": true,
		"blockquote": true, "hr": true,
	}
	return blockTags[tag]
}

// Ensure WebFetchTool implements Tool interface
var _ tool.Tool = (*WebFetchTool)(nil)
