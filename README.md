# linkaudit

A CLI tool that audits all links from a website's sitemap using [lychee](https://github.com/lycheeverse/lychee).

## How it works

1. Fetches the sitemap (or sitemap index) from the provided URL
2. Extracts all page URLs from the sitemap(s)
3. Passes all URLs to lychee for link checking
4. Reports broken links

## Installation

### Prerequisites

You need [lychee](https://github.com/lycheeverse/lychee) installed:

```bash
# macOS
brew install lychee

# Windows (scoop)
scoop install lychee

# Windows (winget)
winget install --id lycheeverse.lychee

# Cargo (any platform)
cargo install lychee
```

### Install linkaudit

```bash
go install github.com/Devon-White/linkaudit/cmd/linkaudit@latest
```

## Usage

```bash
# Basic usage - provide the sitemap URL directly
linkaudit https://example.com/sitemap.xml

# Works with sitemap index files too
linkaudit https://example.com/sitemaps-1-sitemap.xml

# With custom output
linkaudit --output report.json --format json https://example.com/sitemap.xml

# Exclude certain patterns
linkaudit --exclude "linkedin.com" --exclude "twitter.com" https://example.com/sitemap.xml

# Custom concurrency and timeout
linkaudit --concurrency 10 --timeout 60 https://example.com/sitemap.xml

# Use custom lychee config
linkaudit --config ./lychee.toml https://example.com/sitemap.xml

# CI mode (no progress bar)
linkaudit --no-progress https://example.com/sitemap.xml
```

## Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--output` | `-o` | Write report to file | stdout |
| `--format` | `-f` | Output format: compact, json, markdown | compact |
| `--concurrency` | `-c` | Maximum concurrent requests | 20 |
| `--timeout` | `-t` | Request timeout in seconds | 30 |
| `--exclude` | `-e` | Exclude URLs matching pattern (repeatable) | - |
| `--config` | - | Path to lychee config file | - |
| `--no-progress` | - | Disable progress output | false |
| `--verbose` | `-v` | Verbose output | false |

## Exit Codes

- `0` - All links are valid
- `1` - Error (couldn't fetch sitemap, lychee not installed, etc.)
- `2` - Broken links found

## Example Output

```
Fetching sitemap from https://example.com...
Found 42 pages in sitemap

[lychee progress output]

Pages checked: 42 | Passed: 156 | Failed: 3 | Excluded: 12
```

## License

MIT
