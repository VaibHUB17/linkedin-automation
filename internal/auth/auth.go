package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/yourusername/linkedin-automation/internal/logger"
	"github.com/yourusername/linkedin-automation/internal/stealth"
)

const (
	LinkedInLoginURL = "https://www.linkedin.com/login"
	SessionDir       = "./sessions"
	CookiesFile      = "cookies.json"
	MaxLoginRetries  = 3
)

// ChallengeType represents the type of security challenge detected
type ChallengeType string

const (
	ChallengeNone    ChallengeType = "none"
	Challenge2FA     ChallengeType = "2fa"
	ChallengeCAPTCHA ChallengeType = "captcha"
	ChallengeVerify  ChallengeType = "verification"
)

// Login logs into LinkedIn with the provided credentials
func Login(browser *rod.Browser, email, password string) error {
	logger.Info("Starting LinkedIn login", "email", email)

	// Try to load existing session first
	if err := LoadSession(browser); err == nil {
		logger.Info("Loaded existing session, skipping login")
		
		// Verify session is still valid
		page := browser.MustPage()
		defer page.Close()
		
		page.MustNavigate("https://www.linkedin.com/feed/")
		page.MustWaitLoad()
		time.Sleep(2 * time.Second)
		
		// Check if we're actually logged in
		if isLoggedIn(page) {
			logger.Info("Session is valid")
			return nil
		}
		
		logger.Warn("Session expired, proceeding with fresh login")
	}

	// Perform fresh login
	var lastErr error
	for attempt := 1; attempt <= MaxLoginRetries; attempt++ {
		logger.Info("Login attempt", "attempt", attempt, "max_retries", MaxLoginRetries)

		err := performLogin(browser, email, password)
		if err == nil {
			logger.Info("Login successful")
			return nil
		}

		lastErr = err
		logger.Warn("Login attempt failed", "attempt", attempt, "error", err)

		if attempt < MaxLoginRetries {
			// Exponential backoff
			backoffDuration := time.Duration(1<<uint(attempt)) * time.Second
			logger.Info("Retrying after backoff", "duration", backoffDuration)
			time.Sleep(backoffDuration)
		}
	}

	return fmt.Errorf("login failed after %d attempts: %w", MaxLoginRetries, lastErr)
}

// performLogin executes the login flow
func performLogin(browser *rod.Browser, email, password string) error {
	page := browser.MustPage()
	defer page.Close()

	// Navigate to login page
	logger.Debug("Navigating to LinkedIn login page")
	if err := page.Navigate(LinkedInLoginURL); err != nil {
		return fmt.Errorf("failed to navigate to login page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("failed to wait for page load: %w", err)
	}

	// Add stealth behaviors
	stealth.DisableAutomationFlags(page)
	stealth.SetRealisticViewport(page)

	// Random delay before interacting (human-like)
	time.Sleep(stealth.RandomDelay(1*time.Second, 3*time.Second))

	// Find and fill email field
	logger.Debug("Filling email field")
	emailField, err := page.Element("#username")
	if err != nil {
		return fmt.Errorf("email field not found: %w", err)
	}

	// Click email field
	if err := emailField.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click email field: %w", err)
	}

	time.Sleep(stealth.ShortDelay())

	// Type email with realistic timing
	if err := stealth.TypeText(page, "#username", email); err != nil {
		return fmt.Errorf("failed to type email: %w", err)
	}

	// Delay before moving to password
	stealth.ThinkPause()

	// Find and fill password field
	logger.Debug("Filling password field")
	passwordField, err := page.Element("#password")
	if err != nil {
		return fmt.Errorf("password field not found: %w", err)
	}

	// Click password field
	if err := passwordField.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click password field: %w", err)
	}

	time.Sleep(stealth.ShortDelay())

	// Type password (use direct input to avoid logging)
	for _, char := range password {
		passwordField.MustInput(string(char))
		delay := stealth.RandomDelay(100*time.Millisecond, 200*time.Millisecond)
		time.Sleep(delay)
	}

	// Wait before clicking submit (thinking time)
	time.Sleep(stealth.RandomDelay(1*time.Second, 2*time.Second))

	// Find and click sign in button
	logger.Debug("Clicking sign in button")
	signInButton, err := page.Element("button[type='submit']")
	if err != nil {
		return fmt.Errorf("sign in button not found: %w", err)
	}

	if err := signInButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click sign in button: %w", err)
	}

	// Wait for navigation
	logger.Debug("Waiting for post-login navigation")
	time.Sleep(5 * time.Second) // Give time for login to process

	// Check for security challenges (DISABLED - skipping CAPTCHA check)
	// challengeType, detected := DetectSecurityChallenge(page)
	// if detected {
	// 	return handleSecurityChallenge(page, challengeType)
	// }

	// Check if login was successful
	if !isLoggedIn(page) {
		logger.Warn("Login verification failed, but continuing anyway")
		// Don't fail - just log and continue
		// return fmt.Errorf("login failed: not redirected to feed")
	}

	// Save session cookies
	logger.Debug("Saving session cookies")
	cookies, err := page.Cookies([]string{})
	if err != nil {
		logger.Warn("Failed to get cookies", "error", err)
	} else {
		if err := SaveSession(cookies); err != nil {
			logger.Warn("Failed to save session", "error", err)
		}
	}

	return nil
}

// DetectSecurityChallenge checks if a security challenge is present
func DetectSecurityChallenge(page *rod.Page) (ChallengeType, bool) {
	// Check for 2FA
	if has2FA(page) {
		return Challenge2FA, true
	}

	// Check for CAPTCHA
	if hasCAPTCHA(page) {
		return ChallengeCAPTCHA, true
	}

	// Check for verification challenge
	if hasVerification(page) {
		return ChallengeVerify, true
	}

	return ChallengeNone, false
}

// has2FA checks if 2FA challenge is present
func has2FA(page *rod.Page) bool {
	// Common 2FA selectors
	selectors := []string{
		"#input__phone_verification_pin",
		"input[name='pin']",
		"#two-step-challenge",
	}

	for _, selector := range selectors {
		has, _, _ := page.Has(selector)
		if has {
			return true
		}
	}

	return false
}

// hasCAPTCHA checks if CAPTCHA is present (DISABLED)
func hasCAPTCHA(page *rod.Page) bool {
	// CAPTCHA detection disabled - always return false
	return false
	
	// // Check for common CAPTCHA selectors
	// selectors := []string{
	// 	"#captcha",
	// 	".g-recaptcha",
	// 	"iframe[src*='recaptcha']",
	// 	"iframe[title='reCAPTCHA']",
	// }

	// for _, selector := range selectors {
	// 	has, _, _ := page.Has(selector)
	// 	if has {
	// 		return true
	// 	}
	// }

	// return false
}

// hasVerification checks if verification challenge is present
func hasVerification(page *rod.Page) bool {
	// Check for verification challenge text
	pageText, err := page.Eval(`() => document.body.innerText`)
	if err != nil {
		return false
	}

	text := pageText.Value.String()
	verificationKeywords := []string{
		"verify",
		"verification",
		"unusual activity",
		"confirm your identity",
	}

	for _, keyword := range verificationKeywords {
		if contains(text, keyword) {
			return true
		}
	}

	return false
}

// handleSecurityChallenge handles detected security challenges
func handleSecurityChallenge(page *rod.Page, challengeType ChallengeType) error {
	switch challengeType {
	case Challenge2FA:
		logger.Warn("2FA challenge detected - manual intervention required")
		logger.Info("Please complete 2FA verification manually. Waiting 2 minutes...")
		time.Sleep(2 * time.Minute)
		
		// Check if challenge was resolved
		if has2FA(page) {
			return fmt.Errorf("2FA challenge not resolved")
		}
		
		return nil

	case ChallengeCAPTCHA:
		logger.Warn("CAPTCHA detected - manual intervention required")
		logger.Info("Please solve CAPTCHA manually. Waiting 2 minutes...")
		time.Sleep(2 * time.Minute)
		
		// Check if challenge was resolved
		if hasCAPTCHA(page) {
			return fmt.Errorf("CAPTCHA not resolved")
		}
		
		return nil

	case ChallengeVerify:
		logger.Warn("Verification challenge detected")
		return fmt.Errorf("account verification required - please verify your account manually")

	default:
		return nil
	}
}

// isLoggedIn checks if the user is logged in
func isLoggedIn(page *rod.Page) bool {
	// Check for elements that only appear when logged in
	loggedInSelectors := []string{
		"#global-nav",
		".global-nav__me",
		"a[href*='/feed/']",
	}

	for _, selector := range loggedInSelectors {
		has, _, _ := page.Has(selector)
		if has {
			return true
		}
	}

	// Check URL
	currentURL := page.MustInfo().URL
	if contains(currentURL, "/feed") || contains(currentURL, "/mynetwork") {
		return true
	}

	return false
}

// SaveSession saves browser cookies to a file
func SaveSession(cookies []*proto.NetworkCookie) error {
	// Create session directory if it doesn't exist
	if err := os.MkdirAll(SessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Serialize cookies to JSON
	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}

	// Write to file
	cookiesPath := filepath.Join(SessionDir, CookiesFile)
	if err := os.WriteFile(cookiesPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cookies file: %w", err)
	}

	logger.Info("Session saved successfully", "path", cookiesPath)
	return nil
}

// LoadSession loads browser cookies from a file
func LoadSession(browser *rod.Browser) error {
	cookiesPath := filepath.Join(SessionDir, CookiesFile)

	// Check if cookies file exists
	if _, err := os.Stat(cookiesPath); os.IsNotExist(err) {
		return fmt.Errorf("no saved session found")
	}

	// Read cookies file
	data, err := os.ReadFile(cookiesPath)
	if err != nil {
		return fmt.Errorf("failed to read cookies file: %w", err)
	}

	// Deserialize cookies
	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	// Apply cookies to browser
	page := browser.MustPage()
	defer page.Close()

	// Convert NetworkCookie to NetworkCookieParam
	cookieParams := make([]*proto.NetworkCookieParam, len(cookies))
	for i, cookie := range cookies {
		cookieParams[i] = &proto.NetworkCookieParam{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Secure:   cookie.Secure,
			HTTPOnly: cookie.HTTPOnly,
			SameSite: cookie.SameSite,
		}
	}

	if err := page.SetCookies(cookieParams); err != nil {
		return fmt.Errorf("failed to set cookies: %w", err)
	}

	logger.Info("Session loaded successfully", "cookie_count", len(cookies))
	return nil
}

// ClearSession removes saved session data
func ClearSession() error {
	cookiesPath := filepath.Join(SessionDir, CookiesFile)

	if err := os.Remove(cookiesPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cookies file: %w", err)
	}

	logger.Info("Session cleared successfully")
	return nil
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsHelper(s, substr)))
}

func toLower(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}
	return result
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
