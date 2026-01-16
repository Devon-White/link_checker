package sitemap

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

// URLSet represents the sitemap XML structure
type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []URL    `xml:"url"`
}

// URL represents a single URL entry in the sitemap
type URL struct {
	Loc string `xml:"loc"`
}

// SitemapIndex represents a sitemap index file
type SitemapIndex struct {
	XMLName  xml.Name  `xml:"sitemapindex"`
	Sitemaps []Sitemap `xml:"sitemap"`
}

// Sitemap represents a single sitemap entry in a sitemap index
type Sitemap struct {
	Loc string `xml:"loc"`
}

// FetchResult contains URLs grouped by their source sitemap
type FetchResult struct {
	// Sitemaps maps sitemap URL to the page URLs it contains
	Sitemaps map[string][]string
	// AllURLs is a flat list of all URLs (for merged mode)
	AllURLs []string
}



// Fetch retrieves and parses a sitemap, returning all page URLs (merged)
func Fetch(sitemapURL string) ([]string, error) {
	result, err := FetchGrouped(sitemapURL)
	if err != nil {
		return nil, err
	}
	return result.AllURLs, nil
}

// FetchGrouped retrieves and parses a sitemap, returning URLs grouped by source sitemap
func FetchGrouped(sitemapURL string) (*FetchResult, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", sitemapURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Request XML explicitly to avoid getting HTML rendering
	req.Header.Set("Accept", "application/xml, text/xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sitemap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read sitemap body: %w", err)
	}

	result := &FetchResult{
		Sitemaps: make(map[string][]string),
	}

	// Try parsing as sitemap index first
	var sitemapIndex SitemapIndex
	if err := xml.Unmarshal(body, &sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		return fetchSitemapIndexGrouped(client, sitemapIndex)
	}

	// Parse as regular sitemap
	var urlSet URLSet
	if err := xml.Unmarshal(body, &urlSet); err != nil {
		return nil, fmt.Errorf("failed to parse sitemap XML: %w", err)
	}

	urls := make([]string, 0, len(urlSet.URLs))
	for _, u := range urlSet.URLs {
		urls = append(urls, u.Loc)
	}

	result.Sitemaps[sitemapURL] = urls
	result.AllURLs = dedupe(urls)

	return result, nil
}

// fetchSitemapIndexGrouped fetches all sitemaps and groups URLs by source
func fetchSitemapIndexGrouped(client *http.Client, index SitemapIndex) (*FetchResult, error) {
	result := &FetchResult{
		Sitemaps: make(map[string][]string),
	}

	for _, sm := range index.Sitemaps {
		req, err := http.NewRequest("GET", sm.Loc, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/xml, text/xml")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var urlSet URLSet
		if err := xml.Unmarshal(body, &urlSet); err != nil {
			continue
		}

		urls := make([]string, 0, len(urlSet.URLs))
		for _, u := range urlSet.URLs {
			urls = append(urls, u.Loc)
		}

		result.Sitemaps[sm.Loc] = urls
		result.AllURLs = append(result.AllURLs, urls...)
	}

	result.AllURLs = dedupe(result.AllURLs)
	return result, nil
}

// fetchSitemapIndex fetches all sitemaps from a sitemap index
func fetchSitemapIndex(client *http.Client, index SitemapIndex) ([]string, error) {
	var allURLs []string

	for _, sm := range index.Sitemaps {
		req, err := http.NewRequest("GET", sm.Loc, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/xml, text/xml")

		resp, err := client.Do(req)
		if err != nil {
			continue // Skip failed sitemaps
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var urlSet URLSet
		if err := xml.Unmarshal(body, &urlSet); err != nil {
			continue
		}

		for _, u := range urlSet.URLs {
			allURLs = append(allURLs, u.Loc)
		}
	}

	return dedupe(allURLs), nil
}

// dedupe removes duplicate URLs
func dedupe(urls []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(urls))

	for _, u := range urls {
		if !seen[u] {
			seen[u] = true
			result = append(result, u)
		}
	}

	return result
}
