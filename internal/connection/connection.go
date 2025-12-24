package connection

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/yourusername/linkedin-automation/internal/config"
	"github.com/yourusername/linkedin-automation/internal/logger"
	"github.com/yourusername/linkedin-automation/internal/stealth"
	"github.com/yourusername/linkedin-automation/internal/storage"
)

const (
	MaxNoteLength = 300 // LinkedIn's character limit for connection notes
)

// SendConnectionRequest sends a connection request to a profile
func SendConnectionRequest(page *rod.Page, profileURL, note string, cfg *config.Config) error {
	logger.Info("Sending connection request", "profile_url", profileURL)

	// Check daily limit
	requestsToday, err := storage.GetConnectionRequestsSentToday()
	if err != nil {
		return fmt.Errorf("failed to check daily limit: %w", err)
	}

	if requestsToday >= cfg.Connection.DailyLimit {
		return fmt.Errorf("daily connection request limit reached: %d/%d", requestsToday, cfg.Connection.DailyLimit)
	}

	// Navigate to profile
	logger.Debug("Navigating to profile", "url", profileURL)
	if err := page.Navigate(profileURL); err != nil {
		return fmt.Errorf("failed to navigate to profile: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("failed to wait for page load: %w", err)
	}

	// Wait and scroll naturally
	time.Sleep(stealth.RandomDelay(2*time.Second, 4*time.Second))

	// Scroll to view profile sections (human behavior)
	if err := stealth.ScrollFeed(page, 2); err != nil {
		logger.Warn("Failed to scroll profile", "error", err)
	}

	// Scroll back to top where Connect button usually is
	_, err = page.Eval(`() => window.scrollTo({top: 0, behavior: 'smooth'})`)
	if err != nil {
		logger.Warn("Failed to scroll to top", "error", err)
	}

	time.Sleep(stealth.RandomDelay(1*time.Second, 2*time.Second))

	// Find and click Connect button (or Add from More menu)
	if err := ClickConnectButton(page); err != nil {
		return fmt.Errorf("failed to click connect button: %w", err)
	}

	// Wait for modal/dialog to appear
	time.Sleep(stealth.RandomDelay(1*time.Second, 2*time.Second))

	// Check if personalized note dialog appeared
	hasNoteOption, err := hasAddNoteOption(page)
	if err != nil {
		logger.Warn("Failed to check for note option", "error", err)
	}

	if hasNoteOption && note != "" {
		// Click "Add a note" button
		if err := clickAddNoteButton(page); err != nil {
			logger.Warn("Failed to click add note button", "error", err)
		} else {
			time.Sleep(stealth.ShortDelay())

			// Type personalized note
			if err := TypePersonalizedNote(page, note); err != nil {
				logger.Error("Failed to type personalized note", "error", err)
			}
		}
	}

	// Click Send button
	if err := clickSendButton(page); err != nil {
		return fmt.Errorf("failed to click send button: %w", err)
	}

	// Record the connection request
	err = storage.RecordConnectionRequest(storage.ConnectionRequest{
		ProfileURL: profileURL,
		Note:       note,
		SentAt:     time.Now(),
	})
	if err != nil {
		logger.Error("Failed to record connection request", "error", err)
	}

	// Record action for rate limiting
	err = storage.RecordAction("connection_request")
	if err != nil {
		logger.Error("Failed to record action", "error", err)
	}

	logger.Info("Connection request sent successfully", "profile_url", profileURL)
	return nil
}

// ClickConnectButton finds and clicks the Connect button or Add option from More menu
func ClickConnectButton(page *rod.Page) error {
	// Detect which connection flow is available
	connectBtn, flowType := detectConnectFlow(page)
	
	switch flowType {
	case "direct":
		logger.Info("Using direct Connect button")
		return clickDirectConnect(connectBtn)
		
	case "more":
		logger.Info("Using More menu → Connect flow")
		return clickConnectFromMoreMenu(page, connectBtn)
		
	case "none":
		return fmt.Errorf("no connect option available (profile may require Follow first or connection locked)")
		
	default:
		return fmt.Errorf("unknown connection flow type: %s", flowType)
	}
}

// detectConnectFlow detects which connection flow is available on the profile page
func detectConnectFlow(page *rod.Page) (*rod.Element, string) {
	// Try to find direct Connect or Add button first
	connectSelectors := []string{
		"button:has-text('Add')",
		"button[aria-label*='Add']",
		"button:has-text('Connect')",
		"button[aria-label='Connect']",
		"button.artdeco-button--primary:has-text('Connect')",
		"button.artdeco-button--primary:has-text('Add')",
		"button.pvs-profile-actions__action:has-text('Connect')",
		"button.pvs-profile-actions__action:has-text('Add')",
	}
	
	for _, selector := range connectSelectors {
		logger.Debug("Checking for direct Connect/Add button", "selector", selector)
		element, err := page.Timeout(3 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found direct Connect/Add button")
			return element, "direct"
		}
	}
	
	// Try to find More button (for profiles where Connect is in More menu)
	moreSelectors := []string{
		"button[aria-label='More actions']",
		"button[aria-label*='More']",
		"button:has-text('More')",
		"button.artdeco-dropdown__trigger--placement-bottom",
	}
	
	for _, selector := range moreSelectors {
		logger.Debug("Checking for More button", "selector", selector)
		element, err := page.Timeout(3 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found More button")
			return element, "more"
		}
	}
	
	logger.Warn("No connect option found on profile")
	return nil, "none"
}

// clickDirectConnect clicks the direct Connect button
func clickDirectConnect(connectBtn *rod.Element) error {
	if connectBtn == nil {
		return fmt.Errorf("connect button is nil")
	}
	
	// Small delay before clicking (human behavior)
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
	
	if err := connectBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click connect button: %w", err)
	}
	
	logger.Debug("Direct Connect button clicked")
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
	
	return nil
}

// clickConnectFromMoreMenu clicks the More button and then the Connect option from dropdown
func clickConnectFromMoreMenu(page *rod.Page, moreBtn *rod.Element) error {
	if moreBtn == nil {
		return fmt.Errorf("more button is nil")
	}
	
	// Small delay before clicking (human behavior)
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
	
	// Click More button
	if err := moreBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click More button: %w", err)
	}
	
	logger.Debug("More button clicked, waiting for dropdown")
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
	
	// Find and click Connect or Add option from dropdown
	connectSelectors := []string{
		"div[role='menu'] span:has-text('Connect')",
		"div[role='menu'] span:has-text('Add')",
		"li:has-text('Connect')",
		"li:has-text('Add')",
		"div.artdeco-dropdown__item:has-text('Connect')",
		"div.artdeco-dropdown__item:has-text('Add')",
		"button:has-text('Connect')",
		"button:has-text('Add')",
		"span.display-flex:has-text('Connect')",
		"span.display-flex:has-text('Add')",
	}
	
	var connectOption *rod.Element
	var err error
	
	for _, selector := range connectSelectors {
		logger.Debug("Trying Connect/Add option selector", "selector", selector)
		connectOption, err = page.Timeout(3 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found Connect/Add option", "selector", selector)
			break
		}
	}
	
	if err != nil {
		return fmt.Errorf("Connect/Add option not found in dropdown")
	}
	
	// Click Connect/Add option
	if err := connectOption.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click Connect/Add option: %w", err)
	}
	
	logger.Debug("Connect/Add option clicked")
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
	
	return nil
}

// hasAddNoteOption checks if the "Add a note" option is available
func hasAddNoteOption(page *rod.Page) (bool, error) {
	noteSelectors := []string{
		"button:has-text('Add a note')",
		"button[aria-label='Add a note']",
		"button.artdeco-button--secondary:has-text('Add a note')",
	}

	for _, selector := range noteSelectors {
		logger.Debug("Checking for Add a note button", "selector", selector)
		has, _, err := page.Has(selector)
		if err == nil && has {
			logger.Debug("Found Add a note button")
			return true, nil
		}
	}

	logger.Debug("Add a note button not found")
	return false, nil
}

// clickAddNoteButton clicks the "Add a note" button
func clickAddNoteButton(page *rod.Page) error {
	noteSelectors := []string{
		"button:has-text('Add a note')",
		"button[aria-label='Add a note']",
		"button.artdeco-button--secondary:has-text('Add a note')",
	}

	var noteButton *rod.Element
	var err error

	for _, selector := range noteSelectors {
		logger.Debug("Trying Add a note button selector", "selector", selector)
		noteButton, err = page.Timeout(5 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found Add a note button", "selector", selector)
			break
		}
	}

	if err != nil {
		return fmt.Errorf("add note button not found")
	}

	if err := noteButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click add note button: %w", err)
	}

	logger.Info("Add note button clicked, waiting for textarea")
	return nil
}

// TypePersonalizedNote types a personalized note with realistic behavior
func TypePersonalizedNote(page *rod.Page, note string) error {
	logger.Debug("Typing personalized note")

	// Truncate note if too long
	if len(note) > MaxNoteLength {
		note = note[:MaxNoteLength-3] + "..."
		logger.Warn("Note truncated to fit character limit", "max_length", MaxNoteLength)
	}

	// Find note textarea
	noteSelectors := []string{
		"textarea[name='message']",
		"textarea[id*='custom-message']",
		"textarea.send-invite__custom-message",
		"textarea#custom-message",
		"textarea[aria-label*='note']",
	}

	var noteField *rod.Element
	var err error

	for _, selector := range noteSelectors {
		logger.Debug("Trying note textarea selector", "selector", selector)
		noteField, err = page.Timeout(5 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found note textarea", "selector", selector)
			break
		}
	}

	if err != nil {
		return fmt.Errorf("note field not found")
	}

	// Click the field
	if err := noteField.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click note field: %w", err)
	}

	time.Sleep(stealth.ShortDelay())

	// Type the note with realistic timing and occasional typos
	words := strings.Split(note, " ")
	for i, word := range words {
		// Type word
		for j, char := range word {
			// Occasionally introduce typo
			if stealth.RandomDelay(0, 100*time.Millisecond) < 2*time.Millisecond { // 2% chance
				// Type wrong character
				wrongChar := randomTypo(char)
				noteField.Input(string(wrongChar))
				time.Sleep(stealth.RandomDelay(100*time.Millisecond, 200*time.Millisecond))

				// Backspace
				noteField.Input(string(input.Backspace))
				time.Sleep(stealth.RandomDelay(100*time.Millisecond, 200*time.Millisecond))
			}

			// Type correct character
			noteField.Input(string(char))

			// Variable delay between keystrokes
			delay := calculateKeystrokeDelay(j, len(word))
			time.Sleep(delay)
		}

		// Add space after word (except last word)
		if i < len(words)-1 {
			noteField.Input(" ")
			time.Sleep(stealth.RandomDelay(100*time.Millisecond, 300*time.Millisecond))
		}

		// Occasional pause between words (thinking)
		if stealth.RandomDelay(0, 100*time.Millisecond) < 10*time.Millisecond { // 10% chance
			time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1500*time.Millisecond))
		}
	}

	logger.Debug("Personalized note typed successfully")
	return nil
}

// clickSendButton clicks the Send button to submit the connection request
func clickSendButton(page *rod.Page) error {
	sendSelectors := []string{
		"button:has-text('Send without a note')",
		"button.artdeco-button--primary:has-text('Send')",
		"button[aria-label='Send invitation']",
		"button[aria-label='Send now']",
		"button:has-text('Send')",
		"button.ml1[aria-label*='Send']",
	}

	var sendButton *rod.Element
	var err error

	for _, selector := range sendSelectors {
		logger.Debug("Trying send button selector", "selector", selector)
		sendButton, err = page.Timeout(5 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found send button", "selector", selector)
			break
		}
	}

	if err != nil {
		return fmt.Errorf("send button not found after trying all selectors")
	}

	// Small delay before clicking
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1500*time.Millisecond))

	// Click send button
	if err := sendButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click send button: %w", err)
	}

	logger.Debug("Send button clicked")

	// Wait for modal to close
	time.Sleep(stealth.RandomDelay(2*time.Second, 3*time.Second))

	return nil
}

// clickAddFromMoreMenu clicks the More button and then the Add option from dropdown
func clickAddFromMoreMenu(page *rod.Page) error {
	// Find and click More button
	moreSelectors := []string{
		"button[aria-label='More actions']",
		"button[aria-label*='More']",
		"button:has-text('More')",
		"button.artdeco-dropdown__trigger--placement-bottom",
	}

	var moreButton *rod.Element
	var err error

	for _, selector := range moreSelectors {
		logger.Debug("Trying More button selector", "selector", selector)
		moreButton, err = page.Timeout(3 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found More button", "selector", selector)
			break
		}
	}

	if err != nil {
		return fmt.Errorf("More button not found")
	}

	// Click More button
	if err := moreButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click More button: %w", err)
	}

	logger.Debug("More button clicked, waiting for dropdown")
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))

	// Find and click Add option from dropdown
	addSelectors := []string{
		"div[role='menu'] span:has-text('Add')",
		"li:has-text('Add')",
		"div.artdeco-dropdown__item:has-text('Add')",
		"button:has-text('Add')",
		"span.display-flex:has-text('Add')",
	}

	var addOption *rod.Element

	for _, selector := range addSelectors {
		logger.Debug("Trying Add option selector", "selector", selector)
		addOption, err = page.Timeout(3 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found Add option", "selector", selector)
			break
		}
	}

	if err != nil {
		return fmt.Errorf("Add option not found in dropdown")
	}

	// Click Add option
	if err := addOption.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click Add option: %w", err)
	}

	logger.Debug("Add option clicked")
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))

	return nil
}

// CheckDailyLimit checks if the daily connection request limit has been reached
func CheckDailyLimit(cfg *config.Config) (bool, int, error) {
	requestsToday, err := storage.GetConnectionRequestsSentToday()
	if err != nil {
		return false, 0, fmt.Errorf("failed to check daily limit: %w", err)
	}

	remaining := cfg.Connection.DailyLimit - requestsToday
	allowed := remaining > 0

	return allowed, remaining, nil
}

// GeneratePersonalizedNote generates a personalized note from template
func GeneratePersonalizedNote(template string, name, interest string) string {
	note := strings.ReplaceAll(template, "{{name}}", name)
	note = strings.ReplaceAll(note, "{{interest}}", interest)
	return note
}

// calculateKeystrokeDelay calculates realistic delay between keystrokes
func calculateKeystrokeDelay(position, totalLength int) time.Duration {
	baseDelay := 150 * time.Millisecond

	// Slower at the beginning
	if position < 3 {
		baseDelay = 200 * time.Millisecond
	}

	// Occasional longer pauses
	if stealth.RandomDelay(0, 100*time.Millisecond) < 10*time.Millisecond { // 10% chance
		baseDelay = stealth.RandomDelay(300*time.Millisecond, 800*time.Millisecond)
	}

	// Add randomness (±40%)
	variation := 0.4
	randomFactor := 1.0 + (stealth.RandomDelay(0, time.Second).Seconds()*2-1)*variation

	return time.Duration(float64(baseDelay) * randomFactor)
}

// randomTypo returns a typo for a character
func randomTypo(char rune) rune {
	typoMap := map[rune][]rune{
		'a': {'s', 'q', 'w'},
		'b': {'v', 'n'},
		'c': {'x', 'v'},
		'd': {'s', 'f'},
		'e': {'w', 'r'},
		'f': {'d', 'g'},
		'g': {'f', 'h'},
		'h': {'g', 'j'},
		'i': {'u', 'o'},
		'j': {'h', 'k'},
		'k': {'j', 'l'},
		'l': {'k', 'o'},
		'm': {'n'},
		'n': {'b', 'm'},
		'o': {'i', 'p'},
		'p': {'o'},
		'r': {'e', 't'},
		's': {'a', 'd'},
		't': {'r', 'y'},
		'u': {'y', 'i'},
		'v': {'c', 'b'},
		'w': {'q', 'e'},
		'x': {'z', 'c'},
		'y': {'t', 'u'},
		'z': {'x'},
	}

	lowerChar := rune(strings.ToLower(string(char))[0])
	if typos, ok := typoMap[lowerChar]; ok && len(typos) > 0 {
		return typos[int(stealth.RandomDelay(0, time.Duration(len(typos))*time.Millisecond).Milliseconds())%len(typos)]
	}

	return char
}

// ShouldAddNote determines if a personalized note should be added
func ShouldAddNote(cfg *config.Config) bool {
	// Use personalization rate from config
	randomValue := stealth.RandomDelay(0, 100*time.Millisecond).Milliseconds()
	threshold := int64(cfg.Connection.PersonalizationRate * 100)
	return randomValue < threshold
}
