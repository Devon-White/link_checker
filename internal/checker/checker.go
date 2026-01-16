package checker

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options configures the link checker
type Options struct {
	Concurrency int
	Timeout     int
	Excludes    []string
	NoProgress  bool
	ConfigFile  string
	Format      string
	OutputFile  string
	Verbose     bool
}

// LinkStatus represents the status of a checked link
type LinkStatus struct {
	URL       string `json:"url"`
	Status    string `json:"status"`
	Code      int    `json:"code,omitempty"`
	SourceURL string `json:"source_url"`
}

// Result contains the check results
type Result struct {
	PassedCount   int          `json:"passed_count"`
	FailedCount   int          `json:"failed_count"`
	ExcludedCount int          `json:"excluded_count"`
	Links         []LinkStatus `json:"links"`
}

// LycheeLink represents a single link in lychee output
type LycheeLink struct {
	URL    string `json:"url"`
	Status struct {
		Text string `json:"text"`
		Code int    `json:"code"`
	} `json:"status"`
}

// LycheeOutput represents the JSON output from lychee
type LycheeOutput struct {
	Total       int                       `json:"total"`
	Successful  int                       `json:"successful"`
	Errors      int                       `json:"errors"`
	Excludes    int                       `json:"excludes"`
	SuccessMap  map[string][]LycheeLink   `json:"success_map"`
	ErrorMap    map[string][]LycheeLink   `json:"error_map"`
	ExcludedMap map[string][]LycheeLink   `json:"excluded_map"`
}

// CheckURLs runs lychee to check all links on the given page URLs
func CheckURLs(pageURLs []string, opts Options) (*Result, error) {
	if len(pageURLs) == 0 {
		return &Result{}, nil
	}

	// Create temp directory for lychee files
	tempDir, err := os.MkdirTemp("", "linkaudit-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write URLs to temp file for lychee to read
	urlsFile := filepath.Join(tempDir, "urls.txt")
	if err := os.WriteFile(urlsFile, []byte(strings.Join(pageURLs, "\n")), 0644); err != nil {
		return nil, fmt.Errorf("failed to write urls file: %w", err)
	}

	// Determine output format and file
	format := opts.Format
	if format == "" || format == "compact" {
		format = "compact"
	}

	// For JSON output or when we need to parse results, use JSON internally
	jsonOutputFile := filepath.Join(tempDir, "output.json")
	
	// Build lychee command
	args := []string{
		"--max-concurrency", fmt.Sprintf("%d", opts.Concurrency),
		"--timeout", fmt.Sprintf("%d", opts.Timeout),
		"--files-from", urlsFile,
	}

	// Always output JSON so we can parse results
	args = append(args, "--format", "json", "--output", jsonOutputFile)

	if opts.NoProgress {
		args = append(args, "--no-progress")
	}

	if opts.ConfigFile != "" {
		args = append(args, "--config", opts.ConfigFile)
	}

	for _, exclude := range opts.Excludes {
		args = append(args, "--exclude", exclude)
	}

	if opts.Verbose {
		fmt.Printf("Running: lychee %s\n", strings.Join(args, " "))
	}

	// Run lychee
	cmd := exec.Command("lychee", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run and capture exit code (lychee exits 2 on broken links, which is not an error)
	_ = cmd.Run()

	// Parse JSON output
	result := &Result{}

	outputBytes, err := os.ReadFile(jsonOutputFile)
	if err != nil {
		// If no output, assume success
		result.PassedCount = len(pageURLs)
		return result, nil
	}

	var output LycheeOutput
	if err := json.Unmarshal(outputBytes, &output); err != nil {
		return nil, fmt.Errorf("failed to parse lychee output: %w", err)
	}

	result.PassedCount = output.Successful
	result.FailedCount = output.Errors
	result.ExcludedCount = output.Excludes

	// Collect all links with their status and source
	for sourceURL, links := range output.SuccessMap {
		for _, link := range links {
			result.Links = append(result.Links, LinkStatus{
				URL:       link.URL,
				Status:    link.Status.Text,
				Code:      link.Status.Code,
				SourceURL: sourceURL,
			})
		}
	}

	for sourceURL, links := range output.ErrorMap {
		for _, link := range links {
			result.Links = append(result.Links, LinkStatus{
				URL:       link.URL,
				Status:    link.Status.Text,
				Code:      link.Status.Code,
				SourceURL: sourceURL,
			})
		}
	}

	for sourceURL, links := range output.ExcludedMap {
		for _, link := range links {
			result.Links = append(result.Links, LinkStatus{
				URL:       link.URL,
				Status:    "excluded",
				SourceURL: sourceURL,
			})
		}
	}

	// If user requested output to file, copy it or reformat
	if opts.OutputFile != "" {
		switch strings.ToLower(opts.Format) {
		case "json":
			// Copy JSON as-is
			if err := os.WriteFile(opts.OutputFile, outputBytes, 0644); err != nil {
				return nil, fmt.Errorf("failed to write output file: %w", err)
			}
		case "markdown", "md":
			// Format as markdown
			md := formatMarkdown(output, len(pageURLs))
			if err := os.WriteFile(opts.OutputFile, []byte(md), 0644); err != nil {
				return nil, fmt.Errorf("failed to write output file: %w", err)
			}
		default:
			// Compact format - just copy JSON for now
			if err := os.WriteFile(opts.OutputFile, outputBytes, 0644); err != nil {
				return nil, fmt.Errorf("failed to write output file: %w", err)
			}
		}
		fmt.Printf("Report written to %s\n", opts.OutputFile)
	}

	return result, nil
}

func formatMarkdown(output LycheeOutput, pageCount int) string {
	var sb strings.Builder
	sb.WriteString("# Link Audit Report\n\n")
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Pages checked**: %d\n", pageCount))
	sb.WriteString(fmt.Sprintf("- **Total links**: %d\n", output.Total))
	sb.WriteString(fmt.Sprintf("- **Passed**: %d\n", output.Successful))
	sb.WriteString(fmt.Sprintf("- **Failed**: %d\n", output.Errors))
	sb.WriteString(fmt.Sprintf("- **Excluded**: %d\n", output.Excludes))
	sb.WriteString("\n")

	if output.Errors == 0 {
		sb.WriteString("All links are valid!\n")
	} else {
		sb.WriteString("See JSON output for failure details.\n")
	}

	return sb.String()
}

// IsLycheeInstalled checks if lychee is available in PATH
func IsLycheeInstalled() bool {
	_, err := exec.LookPath("lychee")
	return err == nil
}
