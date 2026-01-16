package main

import (
	"encoding/json"
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
	dryRun       bool
	perSitemap   bool
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
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Fetch sitemap and list URLs without checking links")
	rootCmd.Flags().BoolVar(&perSitemap, "per-sitemap", false, "Report results grouped by source sitemap")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	baseURL := args[0]

	// Check lychee is installed (skip for dry-run)
	if !dryRun && !checker.IsLycheeInstalled() {
		return fmt.Errorf("lychee is not installed. Install it from: https://github.com/lycheeverse/lychee")
	}

	// Step 1: Fetch sitemap (supports both sitemap.xml and sitemap index)
	sitemapURL := baseURL
	fmt.Printf("Fetching sitemap from %s...\n", sitemapURL)

	fetchResult, err := sitemap.FetchGrouped(sitemapURL)
	if err != nil {
		return fmt.Errorf("failed to fetch sitemap: %w", err)
	}

	fmt.Printf("Found %d pages in %d sitemap(s)\n\n", len(fetchResult.AllURLs), len(fetchResult.Sitemaps))

	// Dry run: just list URLs and exit
	if dryRun {
		if perSitemap {
			for smURL, urls := range fetchResult.Sitemaps {
				fmt.Printf("=== %s (%d URLs) ===\n", smURL, len(urls))
				for _, u := range urls {
					fmt.Println(u)
				}
				fmt.Println()
			}
		} else {
			for _, u := range fetchResult.AllURLs {
				fmt.Println(u)
			}
		}
		return nil
	}

	// Per-sitemap mode: check each sitemap separately
	if perSitemap {
		return runPerSitemap(fetchResult)
	}

	// Merged mode: check all URLs together
	result, err := checker.CheckURLs(fetchResult.AllURLs, checker.Options{
		Concurrency: concurrency,
		Timeout:     timeout,
		Excludes:    excludes,
		NoProgress:  noProgress,
		ConfigFile:  lycheeConfig,
		Format:      outputFormat,
		OutputFile:  outputFile,
		Verbose:     verbose,
	})
	if err != nil {
		return fmt.Errorf("link check failed: %w", err)
	}

	// Print summary if not already printed by lychee
	if noProgress || outputFormat == "json" {
		fmt.Printf("\nPages checked: %d | Passed: %d | Failed: %d | Excluded: %d\n",
			len(fetchResult.AllURLs), result.PassedCount, result.FailedCount, result.ExcludedCount)
	}

	// Exit with error code if broken links found
	if result.FailedCount > 0 {
		os.Exit(2)
	}

	return nil
}

// SitemapReport represents the check results for a single sitemap
type SitemapReport struct {
	SitemapURL string          `json:"sitemap_url"`
	PageCount  int             `json:"page_count"`
	Result     *checker.Result `json:"result"`
}

// FullReport represents the complete audit report
type FullReport struct {
	Sitemaps    []SitemapReport `json:"sitemaps"`
	TotalPages  int             `json:"total_pages"`
	TotalPassed int             `json:"total_passed"`
	TotalFailed int             `json:"total_failed"`
}

func runPerSitemap(fetchResult *sitemap.FetchResult) error {
	report := FullReport{
		Sitemaps: make([]SitemapReport, 0, len(fetchResult.Sitemaps)),
	}

	hasFailures := false

	for smURL, urls := range fetchResult.Sitemaps {
		fmt.Printf("Checking %s (%d pages)...\n", smURL, len(urls))

		result, err := checker.CheckURLs(urls, checker.Options{
			Concurrency: concurrency,
			Timeout:     timeout,
			Excludes:    excludes,
			NoProgress:  true, // Always quiet in per-sitemap mode
			ConfigFile:  lycheeConfig,
			Format:      "json",
			Verbose:     verbose,
		})
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		report.Sitemaps = append(report.Sitemaps, SitemapReport{
			SitemapURL: smURL,
			PageCount:  len(urls),
			Result:     result,
		})

		report.TotalPages += len(urls)
		report.TotalPassed += result.PassedCount
		report.TotalFailed += result.FailedCount

		if result.FailedCount > 0 {
			hasFailures = true
			fmt.Printf("  FAILED: %d broken links\n", result.FailedCount)
		} else {
			fmt.Printf("  OK: %d links passed\n", result.PassedCount)
		}
	}

	// Output report
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total pages: %d | Passed: %d | Failed: %d\n",
		report.TotalPages, report.TotalPassed, report.TotalFailed)

	if outputFile != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal report: %w", err)
		}
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Printf("Report written to %s\n", outputFile)
	}

	if hasFailures {
		os.Exit(2)
	}

	return nil
}
