package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"devorch/internal/tool"
)

const (
	websearchTimeout    = 30 * time.Second
	websearchMaxResults = 10
)

// WebSearchParams defines parameters for web search
type WebSearchParams struct {
	Query      string `json:"query"`                 // search query
	MaxResults int    `json:"max_results,omitempty"` // max results (default: 5)
	Engine     string `json:"engine,omitempty"`      // search engine (default: duckduckgo)
}

// WebSearchTool performs web searches
type WebSearchTool struct {
	// Timeout for search requests
	Timeout time.Duration

	// DefaultEngine is the default search engine
	DefaultEngine string

	// UserAgent for HTTP requests
	UserAgent string

	// SerpAPIKey for Google search (optional)
	SerpAPIKey string

	// BraveAPIKey for Brave search (optional)
	BraveAPIKey string
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Info returns tool metadata
func (t *WebSearchTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "websearch",
		Description: `Search the web for information.
Returns search results with titles, URLs, and snippets.
Useful for finding current information, documentation, or solutions.

Supported engines:
- duckduckgo (default, no API key needed)
- brave (requires BRAVE_API_KEY)
- google (requires SERP_API_KEY)

Examples:
  - {"query": "golang error handling best practices"}
  - {"query": "python pandas dataframe tutorial", "max_results": 5}
  - {"query": "kubernetes deployment yaml", "engine": "duckduckgo"}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "Search query"
				},
				"max_results": {
					"type": "integer",
					"description": "Maximum number of results (default: 5, max: 10)"
				},
				"engine": {
					"type": "string",
					"enum": ["duckduckgo", "brave", "google"],
					"description": "Search engine to use (default: duckduckgo)"
				}
			},
			"required": ["query"]
		}`),
	}
}

// Execute performs a web search
func (t *WebSearchTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p WebSearchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if strings.TrimSpace(p.Query) == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Set defaults
	maxResults := p.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > websearchMaxResults {
		maxResults = websearchMaxResults
	}

	engine := p.Engine
	if engine == "" {
		engine = t.DefaultEngine
	}
	if engine == "" {
		engine = "duckduckgo"
	}

	// Perform search
	var results []SearchResult
	var err error

	switch engine {
	case "duckduckgo":
		results, err = t.searchDuckDuckGo(p.Query, maxResults)
	case "brave":
		if t.BraveAPIKey == "" {
			return nil, fmt.Errorf("Brave search requires BRAVE_API_KEY")
		}
		results, err = t.searchBrave(p.Query, maxResults)
	case "google":
		if t.SerpAPIKey == "" {
			return nil, fmt.Errorf("Google search requires SERP_API_KEY")
		}
		results, err = t.searchGoogle(p.Query, maxResults)
	default:
		return nil, fmt.Errorf("unsupported search engine: %s", engine)
	}

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Format results
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search: %s\n", p.Query))
	sb.WriteString(fmt.Sprintf("Engine: %s | Results: %d\n\n", engine, len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", r.URL))
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", r.Snippet))
		}
		sb.WriteString("\n")
	}

	if len(results) == 0 {
		sb.WriteString("No results found.\n")
	}

	return &tool.Result{
		Output: sb.String(),
		Title:  fmt.Sprintf("websearch: %s", truncateString(p.Query, 30)),
		Metadata: map[string]any{
			"query":        p.Query,
			"engine":       engine,
			"result_count": len(results),
			"results":      results,
		},
	}, nil
}

// searchDuckDuckGo uses DuckDuckGo HTML for search (no API key needed)
func (t *WebSearchTool) searchDuckDuckGo(query string, maxResults int) ([]SearchResult, error) {
	// Use DuckDuckGo lite version for simpler parsing
	searchURL := fmt.Sprintf("https://lite.duckduckgo.com/lite/?q=%s", url.QueryEscape(query))

	timeout := t.Timeout
	if timeout == 0 {
		timeout = websearchTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	userAgent := t.UserAgent
	if userAgent == "" {
		userAgent = "DevOrch/1.0 (AI Agent Web Search)"
	}
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse DuckDuckGo Lite results
	results := t.parseDuckDuckGoLite(string(body), maxResults)
	return results, nil
}

// parseDuckDuckGoLite extracts results from DuckDuckGo Lite HTML
func (t *WebSearchTool) parseDuckDuckGoLite(html string, maxResults int) []SearchResult {
	var results []SearchResult

	// Simple regex-based parsing for DuckDuckGo Lite
	// Look for result links and snippets

	// Split by result entries
	parts := strings.Split(html, "<tr>")

	for _, part := range parts {
		if len(results) >= maxResults {
			break
		}

		// Look for result link
		if !strings.Contains(part, "result-link") && !strings.Contains(part, "class=\"result") {
			// Alternative: look for typical result patterns
			if !strings.Contains(part, "uddg=") {
				continue
			}
		}

		// Extract URL
		urlStart := strings.Index(part, "uddg=")
		if urlStart == -1 {
			continue
		}
		urlStart += 5 // len("uddg=")
		urlEnd := strings.Index(part[urlStart:], "\"")
		if urlEnd == -1 {
			urlEnd = strings.Index(part[urlStart:], "'")
		}
		if urlEnd == -1 {
			continue
		}

		resultURL, err := url.QueryUnescape(part[urlStart : urlStart+urlEnd])
		if err != nil {
			continue
		}

		// Clean URL
		resultURL = strings.TrimPrefix(resultURL, "//duckduckgo.com/l/?uddg=")
		if ampIdx := strings.Index(resultURL, "&"); ampIdx != -1 {
			resultURL = resultURL[:ampIdx]
		}
		resultURL, _ = url.QueryUnescape(resultURL)

		if resultURL == "" || !strings.HasPrefix(resultURL, "http") {
			continue
		}

		// Extract title (text inside the link)
		title := extractTextBetween(part, ">", "</a>")
		title = cleanHTMLText(title)
		if title == "" {
			title = resultURL
		}

		// Extract snippet
		snippet := ""
		if snippetStart := strings.Index(part, "result-snippet"); snippetStart != -1 {
			snippet = extractTextBetween(part[snippetStart:], ">", "</td>")
			snippet = cleanHTMLText(snippet)
		} else {
			// Try alternative snippet location
			for _, marker := range []string{"<td class=\"result-snippet\">", "<span class=\"result-snippet\">"} {
				if idx := strings.Index(part, marker); idx != -1 {
					snippet = extractTextBetween(part[idx:], ">", "</")
					snippet = cleanHTMLText(snippet)
					break
				}
			}
		}

		results = append(results, SearchResult{
			Title:   truncateString(title, 100),
			URL:     resultURL,
			Snippet: truncateString(snippet, 200),
		})
	}

	// If no results found, try alternative parsing
	if len(results) == 0 {
		results = t.parseDuckDuckGoAlternative(html, maxResults)
	}

	return results
}

// parseDuckDuckGoAlternative alternative parsing method
func (t *WebSearchTool) parseDuckDuckGoAlternative(htmlContent string, maxResults int) []SearchResult {
	var results []SearchResult

	// Look for links with class containing "result"
	lines := strings.Split(htmlContent, "\n")
	var currentResult SearchResult

	for _, line := range lines {
		if len(results) >= maxResults {
			break
		}

		// Look for links
		if strings.Contains(line, "href=") && strings.Contains(line, "http") {
			hrefStart := strings.Index(line, "href=\"")
			if hrefStart == -1 {
				hrefStart = strings.Index(line, "href='")
			}
			if hrefStart == -1 {
				continue
			}
			hrefStart += 6

			hrefEnd := strings.Index(line[hrefStart:], "\"")
			if hrefEnd == -1 {
				hrefEnd = strings.Index(line[hrefStart:], "'")
			}
			if hrefEnd == -1 {
				continue
			}

			href := line[hrefStart : hrefStart+hrefEnd]

			// Skip internal DDG links
			if strings.Contains(href, "duckduckgo.com") && !strings.Contains(href, "uddg=") {
				continue
			}

			// Extract uddg parameter if present
			if uddgIdx := strings.Index(href, "uddg="); uddgIdx != -1 {
				href = href[uddgIdx+5:]
				if ampIdx := strings.Index(href, "&"); ampIdx != -1 {
					href = href[:ampIdx]
				}
				href, _ = url.QueryUnescape(href)
			}

			if !strings.HasPrefix(href, "http") {
				continue
			}

			// Get title from link text
			title := extractTextBetween(line, ">", "</a>")
			title = cleanHTMLText(title)

			if title != "" && href != "" {
				currentResult = SearchResult{
					Title: truncateString(title, 100),
					URL:   href,
				}
				results = append(results, currentResult)
			}
		}
	}

	return results
}

// searchBrave uses Brave Search API
func (t *WebSearchTool) searchBrave(query string, maxResults int) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), maxResults)

	timeout := t.Timeout
	if timeout == 0 {
		timeout = websearchTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Subscription-Token", t.BraveAPIKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Brave API error: %d", resp.StatusCode)
	}

	var braveResp struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&braveResp); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, r := range braveResp.Web.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
		})
	}

	return results, nil
}

// searchGoogle uses SerpAPI for Google search
func (t *WebSearchTool) searchGoogle(query string, maxResults int) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("https://serpapi.com/search.json?q=%s&num=%d&api_key=%s",
		url.QueryEscape(query), maxResults, t.SerpAPIKey)

	timeout := t.Timeout
	if timeout == 0 {
		timeout = websearchTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SerpAPI error: %d", resp.StatusCode)
	}

	var serpResp struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&serpResp); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, r := range serpResp.OrganicResults {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Snippet: r.Snippet,
		})
	}

	return results, nil
}

// Helper functions

func extractTextBetween(s, start, end string) string {
	startIdx := strings.Index(s, start)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(start)

	endIdx := strings.Index(s[startIdx:], end)
	if endIdx == -1 {
		return ""
	}

	return s[startIdx : startIdx+endIdx]
}

func cleanHTMLText(s string) string {
	// Remove HTML tags
	for {
		tagStart := strings.Index(s, "<")
		if tagStart == -1 {
			break
		}
		tagEnd := strings.Index(s[tagStart:], ">")
		if tagEnd == -1 {
			break
		}
		s = s[:tagStart] + s[tagStart+tagEnd+1:]
	}

	// Decode HTML entities
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")

	// Clean whitespace
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Ensure WebSearchTool implements Tool interface
var _ tool.Tool = (*WebSearchTool)(nil)
