package main

import (
	"fmt"
	"os"

	"github.com/Devon-White/link_checker/internal/checker"
	"github.com/Devon-White/link_checker/internal/sitemap"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"

	// Flags
	outputFile   string
	outputFormat string
	concurrency  int
	timeout      int
	excludes     []string
	noProgress   bool
	verbose      bool
	lycheeConfig string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "linkaudit <sitemap-url>",
		Short:   "Audit all links from a website's sitemap",
		Long: `linkaudit fetches a sitemap.xml (or sitemap index) and passes all page URLs to lychee for link checking.

Example:
  linkaudit https://example.com/sitemap.xml
  linkaudit https://example.com/sitemaps-1-sitemap.xml`,
		Version: version,
		Args:    cobra.ExactArgs(1),
		RunE:    run,
	}

	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write report to file")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "compact", "Output format: compact, json, markdown")
	rootCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 20, "Maximum concurrent requests for lychee")
	rootCmd.Flags().IntVarP(&timeout, "timeout", "t", 30, "Request timeout in seconds")
	rootCmd.Flags().StringArrayVarP(&excludes, "exclude", "e", nil, "Exclude URLs matching pattern (can be repeated)")
	rootCmd.Flags().BoolVar(&noProgress, "no-progress", false, "Disable progress output")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().StringVar(&lycheeConfig, "config", "", "Path to lychee config file")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	baseURL := args[0]

	// Check lychee is installed
	if !checker.IsLycheeInstalled() {
		return fmt.Errorf("lychee is not installed. Install it from: https://github.com/lycheeverse/lychee")
	}

	// Step 1: Fetch sitemap (supports both sitemap.xml and sitemap index)
	sitemapURL := baseURL
	fmt.Printf("Fetching sitemap from %s...\n", sitemapURL)

	pageURLs, err := sitemap.Fetch(sitemapURL)
	if err != nil {
		return fmt.Errorf("failed to fetch sitemap: %w", err)
	}

	fmt.Printf("Found %d pages in sitemap\n\n", len(pageURLs))

	// Step 2: Run lychee on all page URLs
	// Lychee will crawl each page and check all links it finds
	result, err := checker.CheckURLs(pageURLs, checker.Options{
		Concurrency:  concurrency,
		Timeout:      timeout,
		Excludes:     excludes,
		NoProgress:   noProgress,
		ConfigFile:   lycheeConfig,
		Format:       outputFormat,
		OutputFile:   outputFile,
		Verbose:      verbose,
	})
	if err != nil {
		return fmt.Errorf("link check failed: %w", err)
	}

	// Print summary if not already printed by lychee
	if noProgress || outputFormat == "json" {
		fmt.Printf("\nPages checked: %d | Passed: %d | Failed: %d | Excluded: %d\n",
			len(pageURLs), result.PassedCount, result.FailedCount, result.ExcludedCount)
	}

	// Exit with error code if broken links found
	if result.FailedCount > 0 {
		os.Exit(2)
	}

	return nil
}
