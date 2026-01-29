// Package tools provides the fetch tool implementation
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"devorch/internal/tool"

	"golang.org/x/net/html"
)

// FetchParams represents parameters for the fetch tool
type FetchParams struct {
	URL     string `json:"url"`
	Format  string `json:"format"`  // "text", "html", "markdown"
	Timeout int    `json:"timeout"` // milliseconds
}

// FetchTool implements web fetching
type FetchTool struct {
	httpClient *http.Client
}

const (
	FetchToolName       = "fetch"
	defaultFetchTimeout = 30000      // 30 seconds
	maxContentSize      = 500 * 1024 // 500KB
)

// NewFetchTool creates a new fetch tool
func NewFetchTool() *FetchTool {
	return &FetchTool{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Info returns tool information
func (t *FetchTool) Info() tool.ToolInfo {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to fetch",
			},
			"format": map[string]any{
				"type":        "string",
				"enum":        []string{"text", "html", "markdown"},
				"description": "Output format: 'text' extracts readable text, 'html' returns raw HTML, 'markdown' converts to markdown",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in milliseconds (default: 30000)",
			},
		},
		"required": []string{"url", "format"},
	}
	schemaJSON, _ := json.Marshal(schema)

	return tool.ToolInfo{
		Name: FetchToolName,
		Description: `Fetch data from URLs and web pages.

WHEN TO USE THIS TOOL:
- Use when you need to fetch content from a web page
- Helpful for reading documentation, APIs, or online resources
- Good for extracting text content from HTML pages

HOW TO USE:
- Provide the URL to fetch
- Specify the format: 'text' for readable content, 'html' for raw HTML, 'markdown' for markdown

FEATURES:
- Extracts readable text from HTML pages
- Can return raw HTML for parsing
- Converts HTML to markdown for easy reading
- Handles redirects automatically

TIPS:
- Use 'text' format for most web pages to get clean content
- Use 'html' format when you need to see the page structure
- Use 'markdown' format for documentation pages`,
		Parameters: schemaJSON,
	}
}

// Execute runs the fetch tool
func (t *FetchTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p FetchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing parameters: %s", err),
			ExitCode: 1,
		}, nil
	}

	if p.URL == "" {
		return &tool.Result{
			Output:   "url is required",
			ExitCode: 1,
		}, nil
	}

	if p.Format == "" {
		p.Format = "text"
	}

	if p.Format != "text" && p.Format != "html" && p.Format != "markdown" {
		return &tool.Result{
			Output:   "format must be 'text', 'html', or 'markdown'",
			ExitCode: 1,
		}, nil
	}

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultFetchTimeout
	}

	// Create request with timeout
	reqCtx, cancel := context.WithTimeout(ctx.Ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", p.URL, nil)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error creating request: %s", err),
			ExitCode: 1,
		}, nil
	}

	// Set a reasonable user agent
	req.Header.Set("User-Agent", "DevOrch/1.0 (AI Coding Assistant)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error fetching URL: %s", err),
			ExitCode: 1,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &tool.Result{
			Output:   fmt.Sprintf("HTTP error: %d %s", resp.StatusCode, resp.Status),
			ExitCode: 1,
		}, nil
	}

	// Read body with size limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxContentSize))
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error reading response: %s", err),
			ExitCode: 1,
		}, nil
	}

	var content string
	switch p.Format {
	case "html":
		content = string(body)
	case "text":
		content = extractTextContent(string(body))
	case "markdown":
		content = convertToMarkdown(string(body))
	}

	// Truncate if too long
	if len(content) > maxContentSize {
		content = content[:maxContentSize] + "\n\n[Content truncated...]"
	}

	return &tool.Result{
		Output: content,
		Metadata: map[string]any{
			"url":          p.URL,
			"status":       resp.StatusCode,
			"content_type": resp.Header.Get("Content-Type"),
		},
	}, nil
}

func extractTextContent(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	var sb strings.Builder
	var extract func(*html.Node)

	extract = func(n *html.Node) {
		// Skip script, style, and other non-content tags
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "iframe", "svg", "path":
				return
			case "br":
				sb.WriteString("\n")
				return
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr":
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

		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "ul", "ol", "table":
				sb.WriteString("\n")
			}
		}
	}

	extract(doc)

	// Clean up the result
	result := sb.String()
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
	result = regexp.MustCompile(` {2,}`).ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	return result
}

func convertToMarkdown(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return extractTextContent(htmlContent)
	}

	var sb strings.Builder
	var convert func(*html.Node)

	convert = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "iframe", "svg", "path":
				return
			case "h1":
				sb.WriteString("\n# ")
			case "h2":
				sb.WriteString("\n## ")
			case "h3":
				sb.WriteString("\n### ")
			case "h4":
				sb.WriteString("\n#### ")
			case "h5":
				sb.WriteString("\n##### ")
			case "h6":
				sb.WriteString("\n###### ")
			case "p":
				sb.WriteString("\n\n")
			case "br":
				sb.WriteString("\n")
			case "strong", "b":
				sb.WriteString("**")
			case "em", "i":
				sb.WriteString("*")
			case "code":
				sb.WriteString("`")
			case "pre":
				sb.WriteString("\n```\n")
			case "li":
				sb.WriteString("\n- ")
			case "a":
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						sb.WriteString("[")
						// Content will be added by children
						defer func(href string) {
							sb.WriteString("](")
							sb.WriteString(href)
							sb.WriteString(")")
						}(attr.Val)
						break
					}
				}
			case "img":
				for _, attr := range n.Attr {
					if attr.Key == "alt" {
						sb.WriteString(fmt.Sprintf("![%s]", attr.Val))
					}
					if attr.Key == "src" {
						sb.WriteString(fmt.Sprintf("(%s)", attr.Val))
					}
				}
				return
			}
		}

		if n.Type == html.TextNode {
			text := n.Data
			if strings.TrimSpace(text) != "" {
				sb.WriteString(text)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			convert(c)
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				sb.WriteString("\n")
			case "strong", "b":
				sb.WriteString("**")
			case "em", "i":
				sb.WriteString("*")
			case "code":
				sb.WriteString("`")
			case "pre":
				sb.WriteString("\n```\n")
			case "ul", "ol":
				sb.WriteString("\n")
			}
		}
	}

	convert(doc)

	// Clean up
	result := sb.String()
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	return result
}
