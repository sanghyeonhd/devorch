// Package tools provides web integration capabilities
package tools

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// WebTools provides web fetching and searching capabilities
type WebTools struct {
	client    *http.Client
	userAgent string
}

// NewWebTools creates a new web tools instance
func NewWebTools() *WebTools {
	return &WebTools{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
}

// WebFetchOptions configures web fetching behavior
type WebFetchOptions struct {
	Format    string // "text", "markdown", "html"
	Timeout   int    // seconds
	MaxSize   int    // bytes
	UserAgent string
	Headers   map[string]string
}

// DefaultWebFetchOptions returns sensible defaults
func DefaultWebFetchOptions() WebFetchOptions {
	return WebFetchOptions{
		Format:    "markdown",
		Timeout:   30,
		MaxSize:   5 * 1024 * 1024, // 5MB
		UserAgent: "Mozilla/5.0 (compatible; DevOrch/1.0)",
		Headers:   make(map[string]string),
	}
}

// WebFetchResult contains the result of a web fetch operation
type WebFetchResult struct {
	URL         string
	StatusCode  int
	ContentType string
	Size        int
	Format      string
	Content     string
	Title       string
	Summary     string
}

// FetchWebPage fetches a web page and converts it to the specified format
func (wt *WebTools) FetchWebPage(targetURL string, options WebFetchOptions) (*WebFetchResult, error) {
	fmt.Printf("🌐 Fetching: %s\n", targetURL)

	// Validate URL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		return nil, fmt.Errorf("URL must start with http:// or https://")
	}

	// Create request
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", options.UserAgent)

	// Set Accept header based on format
	switch options.Format {
	case "markdown":
		req.Header.Set("Accept", "text/markdown;q=1.0, text/x-markdown;q=0.9, text/plain;q=0.8, text/html;q=0.7, */*;q=0.1")
	case "text":
		req.Header.Set("Accept", "text/plain;q=1.0, text/markdown;q=0.9, text/html;q=0.8, */*;q=0.1")
	case "html":
		req.Header.Set("Accept", "text/html;q=1.0, application/xhtml+xml;q=0.9, text/plain;q=0.8, text/markdown;q=0.7, */*;q=0.1")
	default:
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}

	// Add custom headers
	for key, value := range options.Headers {
		req.Header.Set(key, value)
	}

	// Set timeout
	if options.Timeout > 0 {
		wt.client.Timeout = time.Duration(options.Timeout) * time.Second
	}

	// Perform request
	resp, err := wt.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	// Read response with size limit
	limitedReader := io.LimitReader(resp.Body, int64(options.MaxSize))
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	content := string(bodyBytes)
	result := &WebFetchResult{
		URL:         targetURL,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Size:        len(bodyBytes),
		Format:      options.Format,
	}

	// Parse and convert content based on format
	if strings.Contains(result.ContentType, "text/html") {
		result.Title, result.Content, err = wt.parseHTML(content, options.Format)
		if err != nil {
			return nil, fmt.Errorf("failed to parse HTML: %w", err)
		}
	} else {
		result.Content = content
		result.Title = wt.extractTitleFromURL(targetURL)
	}

	// Generate summary
	result.Summary = wt.generateSummary(result.Content, 200)

	fmt.Printf("✅ Fetched %d bytes (%s)\n", result.Size, options.Format)
	if result.Title != "" {
		fmt.Printf("📄 Title: %s\n", result.Title)
	}

	return result, nil
}

// parseHTML parses HTML content and converts it based on format
func (wt *WebTools) parseHTML(content, format string) (title string, converted string, err error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", "", err
	}

	// Extract title
	title = doc.Find("title").Text()
	title = strings.TrimSpace(title)

	switch format {
	case "markdown":
		converted = wt.htmlToMarkdown(doc)
	case "text":
		converted = doc.Text()
	case "html":
		converted = content
	default:
		converted = doc.Text()
	}

	return title, converted, nil
}

// htmlToMarkdown converts HTML to Markdown (basic implementation)
func (wt *WebTools) htmlToMarkdown(doc *goquery.Document) string {
	var markdown strings.Builder

	// Extract main content areas first
	mainContent := doc.Find("main, article, .content, .post, #content").First()
	if mainContent.Length() == 0 {
		mainContent = doc.Find("body")
	}

	// Convert headers
	mainContent.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		tagName := s.Get(0).Data
		level := 1
		if len(tagName) > 1 {
			switch tagName[1] {
			case '1':
				level = 1
			case '2':
				level = 2
			case '3':
				level = 3
			case '4':
				level = 4
			case '5':
				level = 5
			case '6':
				level = 6
			}
		}
		prefix := strings.Repeat("#", level)
		text := strings.TrimSpace(s.Text())
		if text != "" {
			markdown.WriteString(fmt.Sprintf("%s %s\n\n", prefix, text))
		}
	})

	// Convert paragraphs
	mainContent.Find("p").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			markdown.WriteString(text)
			markdown.WriteString("\n\n")
		}
	})

	// Convert lists
	mainContent.Find("ul, ol").Each(func(i int, s *goquery.Selection) {
		s.Find("li").Each(func(j int, li *goquery.Selection) {
			text := strings.TrimSpace(li.Text())
			if text != "" {
				if s.Is("ul") {
					markdown.WriteString("- ")
				} else {
					markdown.WriteString(fmt.Sprintf("%d. ", j+1))
				}
				markdown.WriteString(text)
				markdown.WriteString("\n")
			}
		})
		markdown.WriteString("\n")
	})

	// Convert links
	mainContent.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		href, exists := s.Attr("href")
		if exists && text != "" {
			markdown.WriteString(fmt.Sprintf("[%s](%s)", text, href))
		}
	})

	return markdown.String()
}

// generateSummary creates a summary of the content
func (wt *WebTools) generateSummary(content string, maxLength int) string {
	// Remove excessive whitespace
	re := regexp.MustCompile(`\s+`)
	cleaned := re.ReplaceAllString(strings.TrimSpace(content), " ")

	if len(cleaned) <= maxLength {
		return cleaned
	}

	// Find a good breaking point
	truncated := cleaned[:maxLength]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLength/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}

// extractTitleFromURL extracts a title from URL
func (wt *WebTools) extractTitleFromURL(targetURL string) string {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return targetURL
	}

	// Extract from path
	path := strings.TrimPrefix(parsedURL.Path, "/")
	path = strings.TrimSuffix(path, "/")

	if path != "" {
		parts := strings.Split(path, "/")
		return strings.ReplaceAll(parts[len(parts)-1], "-", " ")
	}

	return parsedURL.Host
}

// SearchWebOptions configures web search behavior
type SearchWebOptions struct {
	NumResults int
	Engine     string // "duckduckgo", "google", "bing"
	SafeSearch bool
	Region     string
	Language   string
}

// DefaultSearchWebOptions returns sensible defaults
func DefaultSearchWebOptions() SearchWebOptions {
	return SearchWebOptions{
		NumResults: 8,
		Engine:     "duckduckgo",
		SafeSearch: true,
		Region:     "us-en",
		Language:   "en",
	}
}

// SearchResult represents a single search result
type SearchResult struct {
	Title       string
	URL         string
	Description string
	Source      string
}

// WebSearchResult contains the results of a web search
type WebSearchResult struct {
	Query      string
	Results    []SearchResult
	TotalFound int
	SearchTime time.Duration
	Engine     string
}

// SearchWeb performs a web search (basic implementation using DuckDuckGo)
func (wt *WebTools) SearchWeb(query string, options SearchWebOptions) (*WebSearchResult, error) {
	fmt.Printf("🔍 Searching: %s\n", query)

	startTime := time.Now()

	// Simple DuckDuckGo search implementation
	searchURL := fmt.Sprintf("https://duckduckgo.com/html/?q=%s&t=devorch", url.QueryEscape(query))

	fetchOptions := DefaultWebFetchOptions()
	fetchOptions.Format = "html"

	result, err := wt.FetchWebPage(searchURL, fetchOptions)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	searchResults, err := wt.parseDuckDuckGoResults(result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	// Limit results
	if len(searchResults) > options.NumResults {
		searchResults = searchResults[:options.NumResults]
	}

	webSearchResult := &WebSearchResult{
		Query:      query,
		Results:    searchResults,
		TotalFound: len(searchResults),
		SearchTime: time.Since(startTime),
		Engine:     options.Engine,
	}

	fmt.Printf("✅ Found %d results in %v\n", webSearchResult.TotalFound, webSearchResult.SearchTime)
	for i, r := range webSearchResult.Results {
		fmt.Printf("  %d. %s\n     %s\n", i+1, r.Title, r.URL)
	}

	return webSearchResult, nil
}

// parseDuckDuckGoResults parses DuckDuckGo HTML results (basic implementation)
func (wt *WebTools) parseDuckDuckGoResults(html string) ([]SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var results []SearchResult

	// Find search result links (this is a simplified parser)
	doc.Find("a[href*='uddg=']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Extract actual URL from DuckDuckGo redirect
		if strings.Contains(href, "uddg=") {
			parts := strings.Split(href, "uddg=")
			if len(parts) > 1 {
				actualURL, err := url.QueryUnescape(parts[1])
				if err == nil {
					title := strings.TrimSpace(s.Text())
					if title != "" && actualURL != "" {
						results = append(results, SearchResult{
							Title:  title,
							URL:    actualURL,
							Source: "DuckDuckGo",
						})
					}
				}
			}
		}
	})

	return results, nil
}

// QuickSearch performs a quick search and returns formatted results
func (wt *WebTools) QuickSearch(query string) (string, error) {
	options := DefaultSearchWebOptions()
	options.NumResults = 5

	result, err := wt.SearchWeb(query, options)
	if err != nil {
		return "", err
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for: %s\n\n", query))

	for i, r := range result.Results {
		output.WriteString(fmt.Sprintf("%d. **%s**\n   %s\n\n", i+1, r.Title, r.URL))
	}

	return output.String(), nil
}
