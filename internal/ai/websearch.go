package ai

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SearchResult represents a single search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// WebSearchClient performs web searches using DuckDuckGo's HTML-lite API.
type WebSearchClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewWebSearchClient creates a new web search client.
func NewWebSearchClient() *WebSearchClient {
	return &WebSearchClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		baseURL: "https://html.duckduckgo.com/html/",
	}
}

// Search performs a web search and returns results.
func (w *WebSearchClient) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 20 {
		maxResults = 20
	}

	log.Printf("[websearch] searching: %s", query)

	// Build request with DuckDuckGo query parameters
	params := url.Values{}
	params.Set("q", query)

	reqURL := w.baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DevSocial/1.0)")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Parse HTML response
	results, err := w.parseDuckDuckGoHTML(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Limit results
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	log.Printf("[websearch] found %d results", len(results))
	return results, nil
}

// parseDuckDuckGoHTML extracts search results from DuckDuckGo's HTML response.
func (w *WebSearchClient) parseDuckDuckGoHTML(r io.Reader) ([]SearchResult, error) {
	// Read the HTML
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	html := string(body)
	results := []SearchResult{}

	// DuckDuckGo wraps results in <div class="result results_links results_links_deep web-result ">
	// This is a simple parser that looks for patterns in the HTML

	// Split by result container
	resultBlocks := w.splitResults(html)

	for _, block := range resultBlocks {
		if result := w.parseResultBlock(block); result != nil {
			results = append(results, *result)
		}
	}

	return results, nil
}

// splitResults divides the HTML into individual result blocks.
func (w *WebSearchClient) splitResults(html string) []string {
	var blocks []string

	// Find all result blocks using class identifier
	marker := `class="result`
	start := 0

	for {
		idx := strings.Index(html[start:], marker)
		if idx == -1 {
			break
		}
		idx += start

		// Find the end of this div (look for closing </div> or next result)
		// This is a simple heuristic - we look for the next result marker or end
		end := strings.Index(html[idx+100:], marker)
		if end == -1 {
			// Last result - take a reasonable chunk
			endIdx := idx + 2000
			if endIdx > len(html) {
				endIdx = len(html)
			}
			blocks = append(blocks, html[idx:endIdx])
			break
		}
		end += idx + 100

		blocks = append(blocks, html[idx:end])
		start = idx + len(marker)
	}

	return blocks
}

// parseResultBlock extracts title, URL, and snippet from a result block.
func (w *WebSearchClient) parseResultBlock(block string) *SearchResult {
	result := &SearchResult{}

	// Extract URL (look for href="...")
	urlStart := strings.Index(block, `href="`)
	if urlStart == -1 {
		return nil
	}
	urlStart += 6
	urlEnd := strings.Index(block[urlStart:], `"`)
	if urlEnd == -1 {
		return nil
	}
	result.URL = block[urlStart : urlStart+urlEnd]

	// Clean up DuckDuckGo redirect URLs
	if strings.HasPrefix(result.URL, "/l/?uddg=") {
		if parsed, err := url.ParseQuery(strings.TrimPrefix(result.URL, "/l/?uddg=")); err == nil {
			if urls := parsed["uddg"]; len(urls) > 0 {
				if unescaped, err := url.PathUnescape(urls[0]); err == nil {
					result.URL = unescaped
				}
			}
		}
	}

	// Skip internal DuckDuckGo links
	if strings.HasPrefix(result.URL, "/") || strings.Contains(result.URL, "duckduckgo.com") {
		return nil
	}

	// Extract title (usually in <a class="result__a">...</a>)
	titleStart := strings.Index(block, `class="result__a"`)
	if titleStart != -1 {
		titleStart += 17
		titleEnd := strings.Index(block[titleStart:], `</a>`)
		if titleEnd != -1 {
			// Strip HTML tags from title
			title := block[titleStart : titleStart+titleEnd]
			title = w.stripHTML(title)
			title = strings.TrimSpace(title)
			if title != "" {
				result.Title = title
			}
		}
	}

	// Extract snippet (look for class="result__snippet")
	snippetStart := strings.Index(block, `class="result__snippet"`)
	if snippetStart != -1 {
		snippetStart += 23
		snippetEnd := strings.Index(block[snippetStart:], `</a>`)
		if snippetEnd == -1 {
			snippetEnd = strings.Index(block[snippetStart:], `</div>`)
		}
		if snippetEnd != -1 {
			snippet := block[snippetStart : snippetStart+snippetEnd]
			snippet = w.stripHTML(snippet)
			snippet = strings.TrimSpace(snippet)
			// Decode HTML entities
			snippet = w.decodeHTML(snippet)
			result.Snippet = snippet
		}
	}

	// Skip if no title
	if result.Title == "" {
		return nil
	}

	// Truncate snippet if too long
	if len(result.Snippet) > 300 {
		result.Snippet = result.Snippet[:297] + "..."
	}

	return result
}

// stripHTML removes HTML tags from a string.
func (w *WebSearchClient) stripHTML(s string) string {
	var result strings.Builder
	inTag := false

	for _, c := range s {
		if c == '<' {
			inTag = true
		} else if c == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(c)
		}
	}

	return result.String()
}

// decodeHTML handles basic HTML entities.
func (w *WebSearchClient) decodeHTML(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&apos;", "'",
		"&nbsp;", " ",
		"&#8230;", "...",
		"&#8211;", "-",
		"&#8212;", "—",
		"&#8217;", "'",
		"&#8220;", "\"",
		"&#8221;", "\"",
	)
	return replacer.Replace(s)
}

// Health checks if the web search service is accessible.
func (w *WebSearchClient) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", w.baseURL, nil)
	if err != nil {
		return err
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("duckduckgo unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// FormatResults converts search results to a formatted string for LLM consumption.
func FormatResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Found %d result(s):\n\n", len(results)))

	for i, r := range results {
		buf.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, r.Title))
		buf.WriteString(fmt.Sprintf("   URL: %s\n", r.URL))
		if r.Snippet != "" {
			buf.WriteString(fmt.Sprintf("   %s\n", r.Snippet))
		}
		buf.WriteString("\n")
	}

	return buf.String()
}
