package search

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/yourusername/linkedin-automation/internal/config"
	"github.com/yourusername/linkedin-automation/internal/logger"
	"github.com/yourusername/linkedin-automation/internal/stealth"
	"github.com/yourusername/linkedin-automation/internal/storage"
)

// SearchCriteria represents search parameters
type SearchCriteria struct {
	JobTitles []string
	Companies []string
	Locations []string
	Keywords  []string
}

// ProfileInfo contains basic profile information
type ProfileInfo struct {
	URL      string
	Name     string
	Headline string
}

// SearchProfiles searches LinkedIn for profiles matching the criteria
func SearchProfiles(page *rod.Page, criteria SearchCriteria, maxPages int) ([]ProfileInfo, error) {
	logger.Info("Starting profile search", "max_pages", maxPages)

	var allProfiles []ProfileInfo

	// Search by job titles
	for _, jobTitle := range criteria.JobTitles {
		logger.Debug("Searching by job title", "title", jobTitle)
		
		profiles, err := searchByJobTitle(page, jobTitle, criteria, maxPages)
		if err != nil {
			logger.Error("Failed to search by job title", "title", jobTitle, "error", err)
			continue
		}

		allProfiles = append(allProfiles, profiles...)

		// Delay between searches
		time.Sleep(stealth.RandomDelay(10*time.Second, 20*time.Second))
	}

	// Deduplicate profiles
	uniqueProfiles := deduplicateProfiles(allProfiles)

	logger.Info("Profile search completed", "total_found", len(uniqueProfiles))
	return uniqueProfiles, nil
}

// searchByJobTitle searches profiles by job title
func searchByJobTitle(page *rod.Page, jobTitle string, criteria SearchCriteria, maxPages int) ([]ProfileInfo, error) {
	// Build search URL
	searchURL := buildSearchURL(jobTitle, criteria)
	logger.Info("Navigating to search URL", "url", searchURL, "job_title", jobTitle)

	// Navigate to search page
	if err := page.Navigate(searchURL); err != nil {
		return nil, fmt.Errorf("failed to navigate to search page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("failed to wait for page load: %w", err)
	}

	// Wait for search results to load - try multiple indicators
	logger.Debug("Waiting for search results to appear...")
	
	// Wait for either the results container or the main element with a reasonable timeout
	_, waitErr := page.Timeout(15 * time.Second).Race().
		Element("ul.reusable-search__entity-result-list").MustHandle(func(e *rod.Element) {
			logger.Debug("Found reusable-search entity result list")
		}).
		Element(".search-results-container").MustHandle(func(e *rod.Element) {
			logger.Debug("Found search results container")
		}).
		Element("main").MustHandle(func(e *rod.Element) {
			logger.Debug("Found main element")
		}).
		Do()
	
	if waitErr != nil {
		logger.Warn("Timeout waiting for search results elements", "error", waitErr)
	}

	// Additional wait for dynamic content
	time.Sleep(stealth.RandomDelay(3*time.Second, 5*time.Second))

	// Natural scrolling to load results - MORE aggressive to ensure all links load
	logger.Debug("Scrolling to load dynamic content...")
	if err := stealth.ScrollFeed(page, 5); err != nil {
		logger.Warn("Failed to scroll feed", "error", err)
	}
	
	// Wait after scrolling for content to stabilize
	time.Sleep(stealth.RandomDelay(3*time.Second, 5*time.Second))

	var profiles []ProfileInfo

	// Extract profiles from current page
	for pageNum := 1; pageNum <= maxPages; pageNum++ {
		logger.Debug("Processing search page", "page", pageNum)

		// Extract profile URLs with retry logic
		var pageProfiles []ProfileInfo
		var err error
		for attempt := 1; attempt <= 3; attempt++ {
			pageProfiles, err = ExtractProfileURLs(page)
			if err == nil && len(pageProfiles) > 0 {
				break
			}
			logger.Warn("Extraction attempt failed, retrying...", "attempt", attempt, "error", err)
			// Additional scroll and wait before retry
			if attempt < 3 {
				stealth.ScrollFeed(page, 3)
				time.Sleep(stealth.RandomDelay(2*time.Second, 4*time.Second))
			}
		}

		if err != nil || len(pageProfiles) == 0 {
			logger.Error("Failed to extract profiles after retries", "page", pageNum, "error", err)
			break
		}

		profiles = append(profiles, pageProfiles...)
		logger.Debug("Extracted profiles from page", "page", pageNum, "count", len(pageProfiles))

		// Check if there's a next page
		if pageNum < maxPages {
			hasNext, err := goToNextPage(page)
			if err != nil || !hasNext {
				logger.Info("No more pages available", "page", pageNum)
				break
			}

			// Wait for next page to load
			time.Sleep(stealth.RandomDelay(3*time.Second, 6*time.Second))
		}
	}

	return profiles, nil
}

// ExtractProfileURLs extracts profile information from the current search results page
// Uses robust direct link extraction with stability checks and better waiting
func ExtractProfileURLs(page *rod.Page) ([]ProfileInfo, error) {
	// Step 1: Ensure page is fully loaded
	page.MustWaitLoad()
	time.Sleep(2 * time.Second) // Let dynamic content settle

	// Step 2: Wait for search results container to be present
	logger.Debug("Waiting for search results container...")
	_, err := page.Timeout(15 * time.Second).Race().
		Element("ul.reusable-search__entity-result-list").
		Element(".search-results-container").
		Element("div.search-results__list").
		Do()
	if err != nil {
		logger.Warn("Search results container not found, continuing anyway", "error", err)
	}

	// Step 3: Scroll to ensure all visible content is loaded
	logger.Debug("Scrolling to load all visible profile links...")
	for i := 0; i < 3; i++ {
		page.Mouse.Scroll(0, 300, 5)
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(2 * time.Second)

	// Step 4: Wait for DOM to stabilize - check link count stays consistent
	logger.Debug("Waiting for DOM to stabilize...")
	stableCount := 0
	lastCount := 0
	for attempt := 0; attempt < 5; attempt++ {
		linkCountJS := page.MustEval(`() => {
			const links = document.querySelectorAll("a[href*='/in/']");
			const uniqueLinks = new Set();
			links.forEach(link => {
				const href = link.href;
				if (href && href.includes('linkedin.com/in/')) {
					uniqueLinks.add(href.split('?')[0]);
				}
			});
			return uniqueLinks.size;
		}`)
		currentCount := int(linkCountJS.Num())
		
		if currentCount == lastCount && currentCount > 0 {
			stableCount++
			if stableCount >= 2 {
				logger.Debug("DOM stabilized", "link_count", currentCount)
				break
			}
		} else {
			stableCount = 0
		}
		lastCount = currentCount
		logger.Debug("DOM stability check", "attempt", attempt+1, "count", currentCount)
		time.Sleep(1 * time.Second)
	}

	// Step 5: Final check - ensure we have profile links
	linkCountJS := page.MustEval(`() => document.querySelectorAll("a[href*='/in/']").length`)
	logger.Debug("Profile links visible to JavaScript", "count", linkCountJS.Num())
	
	if linkCountJS.Num() == 0 {
		logger.Error("No profile links found after all checks")
		// Save debug screenshot
		screenshotPath := filepath.Join("logs", fmt.Sprintf("no-links-%d.png", time.Now().Unix()))
		os.MkdirAll("logs", 0755)
		page.MustScreenshot(screenshotPath)
		logger.Info("Saved debug screenshot", "path", screenshotPath)
		return nil, fmt.Errorf("no profile links found on page")
	}

	// Step 6: Extract ALL profile links from page
	// Use prefix match for cleaner selection
	links, err := page.Elements("a[href^='https://www.linkedin.com/in/']")
	if err != nil || len(links) == 0 {
		return nil, fmt.Errorf("no profile links found: %w", err)
	}

	logger.Info("Found profile links on page", "total_links", len(links))

	seen := make(map[string]bool)
	var profiles []ProfileInfo
	const maxProfiles = 10 // Profile extraction limit

	logger.Debug("Starting profile extraction from links", "total_links", len(links))

	for i, link := range links {
		// Stop if we've reached the limit
		if len(profiles) >= maxProfiles {
			logger.Info("Reached profile extraction limit", "limit", maxProfiles, "processed", i+1)
			break
		}
		
		// Use Property() instead of Attribute() - LinkedIn hides href from attributes
		hrefProp, err := link.Property("href")
		if err != nil {
			if i < 10 { // Log first few failures only
				logger.Debug("Failed to get href property", "index", i, "error", err)
			}
			continue
		}

		profileURL := cleanProfileURL(hrefProp.String())
		
		// Debug log first successful extraction to verify fix
		if i == 0 {
			logger.Info("Sample href property", "href", profileURL)
		}

		// STRICT validation - must be a proper LinkedIn profile URL
		if !strings.HasPrefix(profileURL, "https://www.linkedin.com/in/") {
			logger.Debug("Skipping invalid profile URL", "url", profileURL, "index", i)
			continue
		}

		// Skip duplicates
		if seen[profileURL] {
			logger.Debug("Skipping duplicate profile URL", "url", profileURL, "index", i)
			continue
		}
		seen[profileURL] = true

		// Check if already in database
		exists, err := storage.ProfileExists(profileURL)
		if err != nil {
			logger.Warn("Failed to check profile existence", "url", profileURL, "error", err)
			// Don't skip on error - continue to add the profile
		} else if exists {
			logger.Info("Profile already exists in database, skipping", "url", profileURL, "index", i)
			continue
		}

		logger.Info("Adding new profile", "url", profileURL, "index", i)

		profiles = append(profiles, ProfileInfo{
			URL: profileURL,
		})

		// Save to database
		err = storage.SaveProfile(storage.Profile{
			ProfileURL:   profileURL,
			DiscoveredAt: time.Now(),
		})
		if err != nil {
			logger.Warn("Failed to save profile", "url", profileURL, "error", err)
		}
	}

	logger.Info("Extracted unique profile URLs from page", "count", len(profiles))
	return profiles, nil
}

// extractFromLinks is a fallback method to extract profiles from raw links
func extractFromLinks(links rod.Elements) ([]ProfileInfo, error) {
	var profiles []ProfileInfo
	seen := make(map[string]bool)

	for _, link := range links {
		href, err := link.Property("href")
		if err != nil {
			continue
		}

		profileURL := href.String()
		profileURL = cleanProfileURL(profileURL)

		// Validate it's a proper LinkedIn profile URL
		if !strings.Contains(profileURL, "linkedin.com/in/") {
			continue
		}

		// Skip duplicates
		if seen[profileURL] {
			continue
		}
		seen[profileURL] = true

		// Check if already in database
		exists, err := storage.ProfileExists(profileURL)
		if err != nil {
			logger.Warn("Failed to check profile existence", "url", profileURL, "error", err)
		} else if exists {
			logger.Debug("Profile already exists in database", "url", profileURL)
			continue
		}

		// Try to extract name from link text
		name := ""
		nameText, err := link.Text()
		if err == nil {
			name = strings.TrimSpace(nameText)
		}

		logger.Debug("Extracted profile from link", "url", profileURL, "name", name)

		profile := ProfileInfo{
			URL:      profileURL,
			Name:     name,
			Headline: "", // Can't extract headline without container
		}

		profiles = append(profiles, profile)

		// Save to database
		err = storage.SaveProfile(storage.Profile{
			ProfileURL:   profileURL,
			Name:         name,
			Headline:     "",
			DiscoveredAt: time.Now(),
		})
		if err != nil {
			logger.Warn("Failed to save profile", "url", profileURL, "error", err)
		}
	}

	return profiles, nil
}

// goToNextPage navigates to the next page of search results
func goToNextPage(page *rod.Page) (bool, error) {
	// Look for "Next" button
	nextButtonSelectors := []string{
		"button[aria-label='Next']",
		".artdeco-pagination__button--next",
		"button.artdeco-pagination__button--next",
	}

	var nextButton *rod.Element
	var err error

	for _, selector := range nextButtonSelectors {
		nextButton, err = page.Element(selector)
		if err == nil {
			break
		}
	}

	if err != nil {
		return false, fmt.Errorf("next button not found")
	}

	// Check if button is disabled
	disabled, err := nextButton.Property("disabled")
	if err == nil && disabled.Bool() {
		return false, nil
	}

	// Scroll to button
	if err := stealth.ScrollToElement(page, nextButtonSelectors[0]); err != nil {
		logger.Warn("Failed to scroll to next button", "error", err)
	}

	time.Sleep(stealth.ShortDelay())

	// Click next button with human-like behavior
	shape, err := nextButton.Shape()
	if err != nil {
		return false, fmt.Errorf("failed to get button shape: %w", err)
	}
	box := shape.Box()

	// Move mouse to button
	if err := stealth.MoveMouse(page, box.X+box.Width/2, box.Y+box.Height/2); err != nil {
		logger.Warn("Failed to move mouse to button", "error", err)
	}

	time.Sleep(stealth.ShortDelay())

	// Click
	if err := nextButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return false, fmt.Errorf("failed to click next button: %w", err)
	}

	return true, nil
}

// buildSearchURL constructs a LinkedIn search URL
func buildSearchURL(jobTitle string, criteria SearchCriteria) string {
	baseURL := "https://www.linkedin.com/search/results/people/"

	params := url.Values{}
	
	// Keywords (job title + additional keywords)
	keywords := []string{jobTitle}
	keywords = append(keywords, criteria.Keywords...)
	params.Add("keywords", strings.Join(keywords, " "))

	// Location
	if len(criteria.Locations) > 0 {
		// Use first location (can be enhanced to search multiple)
		params.Add("geoUrn", criteria.Locations[0])
	}

	// Origin (people search)
	params.Add("origin", "FACETED_SEARCH")

	return baseURL + "?" + params.Encode()
}

// cleanProfileURL removes query parameters and normalizes profile URL
// LinkedIn appends garbage query params like miniProfileUrn that must be stripped
func cleanProfileURL(rawURL string) string {
	// Simple and fast: strip everything after '?'
	if i := strings.Index(rawURL, "?"); i != -1 {
		rawURL = rawURL[:i]
	}
	// Remove trailing slashes for consistency
	return strings.TrimRight(rawURL, "/")
}

// deduplicateProfiles removes duplicate profiles based on URL
func deduplicateProfiles(profiles []ProfileInfo) []ProfileInfo {
	seen := make(map[string]bool)
	var unique []ProfileInfo

	for _, profile := range profiles {
		if !seen[profile.URL] {
			seen[profile.URL] = true
			unique = append(unique, profile)
		}
	}

	return unique
}

// SearchAndCollect performs a search and collects profile URLs
func SearchAndCollect(page *rod.Page, cfg *config.Config) ([]ProfileInfo, error) {
	criteria := SearchCriteria{
		JobTitles: cfg.Search.JobTitles,
		Companies: cfg.Search.Companies,
		Locations: cfg.Search.Locations,
		Keywords:  cfg.Search.Keywords,
	}

	profiles, err := SearchProfiles(page, criteria, cfg.Search.MaxPages)
	if err != nil {
		return nil, fmt.Errorf("failed to search profiles: %w", err)
	}

	// Limit to max results
	if len(profiles) > cfg.Search.MaxResults {
		profiles = profiles[:cfg.Search.MaxResults]
	}

	return profiles, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
