package connection

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/yourusername/linkedin-automation/internal/config"
	"github.com/yourusername/linkedin-automation/internal/logger"
	"github.com/yourusername/linkedin-automation/internal/stealth"
	"github.com/yourusername/linkedin-automation/internal/storage"
)

const (
	MaxNoteLength = 300 // LinkedIn's character limit for connection notes
	DefaultNote   = "Hi, I came across your profile and was impressed by your background and experience. I'd love to connect and exchange insights about the industry. I believe we could have valuable discussions and potentially explore collaboration opportunities. Looking forward to connecting with you and learning from your expertise."
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

	// Wait for network idle to ensure page is fully loaded (LinkedIn has lots of dynamic JS)
	logger.Debug("Waiting for page to be fully interactive...")
	page.Timeout(15 * time.Second).WaitIdle(1 * time.Second)

	// Wait and scroll naturally
	time.Sleep(stealth.RandomDelay(3*time.Second, 5*time.Second))

	// Scroll to view profile sections (human behavior)
	if err := stealth.ScrollFeed(page, 2); err != nil {
		logger.Warn("Failed to scroll profile", "error", err)
	}

	// Scroll back to top where Connect button usually is
	_, err = page.Eval(`() => window.scrollTo({top: 0, behavior: 'smooth'})`)
	if err != nil {
		logger.Warn("Failed to scroll to top", "error", err)
	}

	time.Sleep(stealth.RandomDelay(2*time.Second, 3*time.Second))

	// Remove overlays BEFORE looking for button
	logger.Debug("Removing overlay blockers...")
	if err := removeOverlayBlockers(page); err != nil {
		logger.Warn("Failed to remove overlay blockers", "error", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Find and click Connect button (or Add from More menu)
	if err := ClickConnectButton(page); err != nil {
		return fmt.Errorf("failed to click connect button: %w", err)
	}

	// Wait for modal/dialog to appear
	time.Sleep(stealth.RandomDelay(1*time.Second, 2*time.Second))

	// Use default note if none provided
	if note == "" {
		note = DefaultNote
		logger.Info("Using default 50-word note")
	}

	// Check if personalized note dialog appeared
	hasNoteOption, err := hasAddNoteOption(page)
	if err != nil {
		logger.Warn("Failed to check for note option", "error", err)
	}

	// Always try to add a note
	if hasNoteOption {
		// Click "Add a note" button
		if err := clickAddNoteButton(page); err != nil {
			logger.Warn("Failed to click add note button, sending without note", "error", err)
		} else {
			// Wait longer for textarea to appear and be ready
			time.Sleep(stealth.RandomDelay(1500*time.Millisecond, 2500*time.Millisecond))

			// Type personalized note
			if err := TypePersonalizedNote(page, note); err != nil {
				logger.Error("Failed to type personalized note, sending without note", "error", err)
			} else {
				// Wait to ensure text is fully entered before clicking Send
				time.Sleep(stealth.RandomDelay(800*time.Millisecond, 1500*time.Millisecond))
				logger.Info("Successfully typed personalized note")
			}
		}
	} else {
		logger.Info("Note dialog not available, sending without note")
	}

	// Click Send button
	if err := clickSendButton(page); err != nil {
		return fmt.Errorf("failed to click send button: %w", err)
	}

	// Handle post-send verification badge prompt (if it appears)
	if err := dismissVerificationPrompt(page); err != nil {
		logger.Warn("Failed to dismiss verification prompt", "error", err)
		// Don't return error - this is optional
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
	_, flowType := detectConnectFlow(page)
	
	switch flowType {
	case "direct":
		logger.Info("Using direct Connect button (main profile)")
		// Scroll to top to ensure main profile button is visible
		logger.Debug("Scrolling to top for main profile button...")
		_, err := page.Eval(`() => window.scrollTo({top: 0, behavior: 'smooth'})`)
		if err != nil {
			logger.Warn("Failed to scroll to top", "error", err)
		}
		time.Sleep(stealth.RandomDelay(1*time.Second, 2*time.Second))
		
		// Re-fetch main profile button selectors
		mainProfileSelectors := []string{
			"div.pv-top-card-profile-picture__container ~ div button[aria-label*='Invite']:not(.pvs-sticky-header-profile-actions__action)",
			"div.ph5 button.artdeco-button--primary[aria-label*='Invite']:not(.pvs-sticky-header-profile-actions__action)",
			"section.artdeco-card button.artdeco-button--primary[aria-label*='Invite']:not(.pvs-sticky-header-profile-actions__action)",
			"div.pv-top-card--list button.artdeco-button--primary[aria-label*='Invite']",
		}
		
		var btn *rod.Element
		for i, selector := range mainProfileSelectors {
			logger.Debug("Attempting to find main profile Connect button", "selector_index", i, "selector", selector)
			btn, err = page.Timeout(5 * time.Second).Element(selector)
			
			if err == nil {
				logger.Info("✅ Successfully found main profile Connect button", "selector_index", i, "selector", selector)
				break
			} else {
				logger.Debug("❌ Selector failed", "selector_index", i, "error", err.Error())
			}
		}
		
		if err != nil {
			return fmt.Errorf("failed to find main profile connect button: %w", err)
		}
		
		if err := clickDirectConnect(btn, page); err != nil {
			return err
		}
		
		return verifyConnectDialogAppeared(page)
		
	case "sticky":
		logger.Info("Using sticky header Connect button")
		// Scroll down to make sticky header visible
		logger.Debug("Scrolling down to activate sticky header...")
		_, err := page.Eval(`() => window.scrollTo({top: 400, behavior: 'smooth'})`)
		if err != nil {
			logger.Warn("Failed to scroll for sticky header", "error", err)
		}
		time.Sleep(stealth.RandomDelay(1*time.Second, 2*time.Second))
		
		// Re-fetch sticky header button
		stickySelectors := []string{
			"button.pvs-sticky-header-profile-actions__action[aria-label*='Invite']",
			"button[id^='ember'].pvs-sticky-header-profile-actions__action.artdeco-button--primary[aria-label*='Invite']",
		}
		
		var btn *rod.Element
		for i, selector := range stickySelectors {
			logger.Debug("Attempting to find sticky header Connect button", "selector_index", i, "selector", selector)
			btn, err = page.Timeout(5 * time.Second).Element(selector)
			
			if err == nil {
				logger.Info("✅ Successfully found sticky header Connect button", "selector_index", i, "selector", selector)
				break
			} else {
				logger.Debug("❌ Selector failed", "selector_index", i, "error", err.Error())
			}
		}
		
		if err != nil {
			return fmt.Errorf("failed to find sticky header connect button: %w", err)
		}
		
		if err := clickDirectConnect(btn, page); err != nil {
			return err
		}
		
		return verifyConnectDialogAppeared(page)
		
	case "more":
		logger.Info("Using More menu → Connect flow")
		// Re-fetch More button
		moreSelectors := []string{
			"button[aria-label*='More']",
			"button.artdeco-dropdown__trigger",
			"button.artdeco-dropdown__trigger--placement-bottom",
		}
		
		var moreBtn *rod.Element
		var err error
		for i, selector := range moreSelectors {
			logger.Debug("Attempting to find More button", "selector_index", i, "selector", selector)
			moreBtn, err = page.Timeout(5 * time.Second).Element(selector)
			if err == nil {
				logger.Info("✅ Successfully found More button", "selector_index", i, "selector", selector)
				
				// Log button attributes for debugging
				if ariaLabel, err := moreBtn.Attribute("aria-label"); err == nil && ariaLabel != nil {
					logger.Debug("More button aria-label", "value", *ariaLabel)
				}
				if class, err := moreBtn.Attribute("class"); err == nil && class != nil {
					logger.Debug("More button class", "value", *class)
				}
				
				break
			} else {
				logger.Debug("❌ Selector failed", "selector_index", i, "error", err.Error())
			}
		}
		
		if err != nil {
			return fmt.Errorf("failed to re-fetch more button: %w", err)
		}
		
		if err := clickConnectFromMoreMenu(page, moreBtn); err != nil {
			return err
		}
		
		// Verify the dialog appeared
		return verifyConnectDialogAppeared(page)
		
	case "none":
		logger.Warn("No connect option available - profile may require Follow first or connection locked")
		return fmt.Errorf("no connect option available (profile may require Follow first or connection locked)")
		
	default:
		logger.Error("Unknown connection flow type", "flow_type", flowType)
		return fmt.Errorf("unknown connection flow type: %s", flowType)
	}
}

// detectConnectFlow detects which connection flow is available on the profile page
func detectConnectFlow(page *rod.Page) (*rod.Element, string) {
	// PRIORITY 1: Look for main profile Connect button (NOT the sticky header)
	// These buttons are in the main profile actions section and are visible at the top
	mainProfileSelectors := []string{
		// Main profile section button (excluding sticky header)
		"div.pv-top-card-profile-picture__container ~ div button[aria-label*='Invite']:not(.pvs-sticky-header-profile-actions__action)",
		"div.ph5 button.artdeco-button--primary[aria-label*='Invite']:not(.pvs-sticky-header-profile-actions__action)",
		"section.artdeco-card button.artdeco-button--primary[aria-label*='Invite']:not(.pvs-sticky-header-profile-actions__action)",
		// More specific: main profile actions container
		"div.pv-top-card--list button.artdeco-button--primary[aria-label*='Invite']",
	}
	
	for _, selector := range mainProfileSelectors {
		logger.Debug("Checking for main profile Connect button", "selector", selector)
		element, err := page.Timeout(3 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found main profile Connect button (non-sticky)")
			return element, "direct"
		}
	}
	
	// PRIORITY 2: If main profile button not found, try sticky header button
	// This requires scrolling down to make it visible
	stickyHeaderSelectors := []string{
		"button.pvs-sticky-header-profile-actions__action[aria-label*='Invite']",
		"button[id^='ember'].pvs-sticky-header-profile-actions__action.artdeco-button--primary[aria-label*='Invite']",
	}
	
	for _, selector := range stickyHeaderSelectors {
		logger.Debug("Checking for sticky header Connect button", "selector", selector)
		element, err := page.Timeout(3 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found sticky header Connect button (requires scroll)")
			return element, "sticky"
		}
	}
	
	// PRIORITY 3: Generic fallback for any primary button with Invite
	connectSelectors := []string{
		"button[id^='ember'].artdeco-button--primary[aria-label*='Invite']",
		"button.artdeco-button--primary[aria-label*='Invite'][aria-label*='connect']",
		"button.artdeco-button--primary:has-text('Add')",
		"button[data-control-name='connect']",
	}
	
	for _, selector := range connectSelectors {
		logger.Debug("Checking for Connect/Invite button (fallback)", "selector", selector)
		element, err := page.Timeout(3 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found Connect/Invite button (generic)")
			return element, "direct"
		}
	}
	
	// PRIORITY 4: Try to find More button (for profiles where Connect is in More menu)
	moreSelectors := []string{
		"button[aria-label*='More']",
		"button.artdeco-dropdown__trigger",
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

// removeOverlayBlockers removes common LinkedIn overlay elements that can block clicks
func removeOverlayBlockers(page *rod.Page) error {
	_, err := page.Eval(`() => {
	// Specifically target the messaging bottom bar which often blocks the 'Add' button area
	const msgOverlay = document.querySelector('.msg-overlay-list-bubble--is-minimized, .msg-overlay-container, .msg-overlay-bubble-header');
	if (msgOverlay) msgOverlay.remove();
	
	// Remove messaging overlay entirely
	const msgOverlays = document.querySelectorAll('.msg-overlay-list-bubble, .msg-overlay-container, aside.msg-overlay-list-bubble');
	msgOverlays.forEach(m => m.remove());
	
	// Remove toasts, modals, and other common blockers
	const blockers = document.querySelectorAll('.artdeco-toast-item, .artdeco-modal:not([role="dialog"]), .artdeco-toasts, .artdeco-hoverable-trigger');
	blockers.forEach(b => b.remove());
	
	// Remove any element with high z-index that might be blocking
	const highZIndex = Array.from(document.querySelectorAll('*')).filter(el => {
		const zIndex = window.getComputedStyle(el).zIndex;
		return !isNaN(zIndex) && parseInt(zIndex) > 1000 && !el.matches('[role="dialog"]');
	});
	highZIndex.forEach(el => {
		if (!el.matches('button, [role="dialog"]')) {
			el.style.zIndex = '-1';
		}
	});
}`)
	return err
}

// verifyConnectDialogAppeared checks that the connection dialog appeared after clicking
func verifyConnectDialogAppeared(page *rod.Page) error {
	_, err := page.Timeout(5 * time.Second).Element("div[role='dialog']")
	if err != nil {
		return fmt.Errorf("connect dialog did not appear after click: %w", err)
	}
	logger.Debug("Connect dialog appeared successfully")
	return nil
}

// clickDirectConnect clicks the direct Connect button with robust error handling
func clickDirectConnect(btn *rod.Element, page *rod.Page) error {
	if btn == nil {
		return fmt.Errorf("connect button is nil")
	}
	
	// Small delay before clicking (human behavior)
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
	
	// 1️⃣ Remove overlays FIRST before checking visibility
	logger.Debug("Removing overlays preemptively...")
	removeOverlayBlockers(page)
	time.Sleep(800 * time.Millisecond) // Give time for overlays to be removed
	
	// 2️⃣ Scroll into view BEFORE visibility check (VERY IMPORTANT for LinkedIn)
	logger.Debug("Scrolling button into view...")
	if err := btn.ScrollIntoView(); err != nil {
		logger.Warn("Failed to scroll button into view", "error", err)
	}
	time.Sleep(500 * time.Millisecond)
	
	// 3️⃣ Ensure element is attached and visible WITH INCREASED TIMEOUT
	logger.Debug("Waiting for button to be visible...")
	if err := btn.Timeout(15 * time.Second).WaitVisible(); err != nil {
		// Try more aggressive overlay removal
		logger.Warn("Button not visible, trying aggressive overlay removal...")
		removeOverlayBlockers(page)
		
		// Try scrolling again
		if scrollErr := btn.ScrollIntoView(); scrollErr == nil {
			time.Sleep(1 * time.Second)
		}
		
		// One more visibility check with extended timeout
		if err := btn.Timeout(10 * time.Second).WaitVisible(); err != nil {
			logger.Warn("Button still not visible after retry, attempting click anyway", "error", err)
			// Don't return error yet - try clicking anyway
		}
	}
	
	logger.Debug("Proceeding to click...")
	time.Sleep(300 * time.Millisecond)
	
	// 4️⃣ Try normal click with increased timeout
	err := btn.Timeout(8 * time.Second).Click(proto.InputMouseButtonLeft, 1)
	if err == nil {
		logger.Debug("Direct Connect button clicked (normal click)")
		time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
		return nil
	}
	
	logger.Warn("Normal click failed, trying JS click", "error", err)
	
	// 5️⃣ Fallback to JS click (LinkedIn-safe)
	_, jsErr := page.Eval(`(el) => { el.click(); }`, btn)
	if jsErr != nil {
		// Last resort: try to click using coordinates
		logger.Warn("JS click also failed, trying click by coordinates", "js_error", jsErr)
		
		shape, shapeErr := btn.Shape()
		if shapeErr == nil && len(shape.Quads) > 0 {
			quad := shape.Quads[0]
			x := (quad[0] + quad[2]) / 2
			y := (quad[1] + quad[5]) / 2
			
			// Move mouse to coordinates then click
			if moveErr := page.Mouse.MoveLinear(proto.NewPoint(x, y), 5); moveErr == nil {
				time.Sleep(200 * time.Millisecond)
				if clickErr := page.Mouse.Click(proto.InputMouseButtonLeft, 1); clickErr == nil {
					logger.Debug("Direct Connect button clicked (coordinate click)")
					time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
					return nil
				}
			}
		}
		
		return fmt.Errorf("all click methods failed: normal=%v, js=%v", err, jsErr)
	}
	
	logger.Debug("Direct Connect button clicked (JS click)")
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
	
	// Wait for dropdown to stabilize
	time.Sleep(stealth.RandomDelay(800*time.Millisecond, 1200*time.Millisecond))
	
	// Find and click Connect option from dropdown
	// Use text matching for dropdown items as they render actual text
	connectSelectors := []string{
		"div[role='menu'] span:has-text('Connect')",
		"div.artdeco-dropdown__content span:has-text('Connect')",
		"div.artdeco-dropdown__item:has-text('Connect')",
		"li[role='menuitem']:has-text('Connect')",
		"button[aria-label*='connect']",
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
		// Wait for modal dialog first
		"div[role='dialog'] button[aria-label='Add a note']",
		"div.send-invite button:has-text('Add a note')",
		"button.artdeco-button--secondary:has-text('Add a note')",
		"button:has-text('Add a note')",
		"button[aria-label='Add a note']",
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
	// First, ensure we're in the connection modal
	logger.Debug("Waiting for connection invitation modal")
	_, err := page.Timeout(5 * time.Second).Element("div[role='dialog']")
	if err != nil {
		logger.Warn("Connection modal not found, continuing anyway")
	}
	
	noteSelectors := []string{
		// Most specific: within the dialog
		"div[role='dialog'] button[aria-label='Add a note']",
		"div.send-invite button[aria-label='Add a note']",
		"button.artdeco-button--secondary[aria-label='Add a note']",
		"button:has-text('Add a note')",
		"button[aria-label='Add a note']",
	}

	var noteButton *rod.Element

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

	// Find note textarea with better timeout and wait for visibility
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
		noteField, err = page.Timeout(8 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found note textarea", "selector", selector)
			break
		}
	}

	if err != nil {
		return fmt.Errorf("note field not found after trying all selectors")
	}

	// Wait for the field to be visible and interactable
	logger.Debug("Waiting for textarea to be visible and ready")
	if err := noteField.WaitVisible(); err != nil {
		logger.Warn("Textarea not visible, continuing anyway", "error", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Scroll textarea into view
	if err := noteField.ScrollIntoView(); err != nil {
		logger.Warn("Failed to scroll textarea into view", "error", err)
	}
	time.Sleep(300 * time.Millisecond)

	// Click the field to focus it
	logger.Debug("Clicking textarea to focus")
	if err := noteField.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click note field: %w", err)
	}

	time.Sleep(stealth.RandomDelay(800*time.Millisecond, 1200*time.Millisecond))

	// Try multiple methods to enter the text (LinkedIn can be finicky)
	logger.Debug("Attempting to enter note text", "length", len(note))
	
	// Method 1: Use Rod's InputTime method for more reliable input
	err = noteField.Timeout(10 * time.Second).Input(note)
	if err != nil {
		logger.Warn("Input method failed, trying JS setValue", "error", err)
		
		// Method 2: Fallback to JavaScript setValue
		_, jsErr := page.Eval(`(el, text) => {
			el.value = text;
			el.dispatchEvent(new Event('input', { bubbles: true }));
			el.dispatchEvent(new Event('change', { bubbles: true }));
		}`, noteField, note)
		
		if jsErr != nil {
			logger.Error("JS setValue also failed", "error", jsErr)
			return fmt.Errorf("failed to enter note text: input failed=%v, js failed=%v", err, jsErr)
		}
		logger.Debug("Note text entered via JavaScript")
	} else {
		logger.Debug("Note text entered via Input method")
	}
	
	// Add realistic delay to simulate typing completion
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
	
	// Verify the text was actually entered
	logger.Debug("Verifying note text was entered")
	actualValue, err := noteField.Eval(`el => el.value`)
	if err != nil {
		logger.Warn("Could not verify note text", "error", err)
	} else {
		if actualValue.Value.Str() == "" {
			logger.Error("Note textarea is empty after input attempt!")
			return fmt.Errorf("note text was not entered successfully")
		}
		logger.Debug("Note text verified", "length", len(actualValue.Value.Str()))
	}

	logger.Info("Personalized note typed successfully", "length", len(note))
	return nil
}

// clickSendButton clicks the Send button to submit the connection request
func clickSendButton(page *rod.Page) error {
	sendSelectors := []string{
		// Priority order: Send with note > Send without note
		"div[role='dialog'] button.artdeco-button--primary[aria-label*='Send']",
		"button.artdeco-button--primary:has-text('Send')",
		"button[aria-label='Send invitation']",
		"button[aria-label='Send now']",
		"button:has-text('Send without a note')",
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

// dismissVerificationPrompt dismisses the "Verify now" modal that appears after sending invitations
func dismissVerificationPrompt(page *rod.Page) error {
	logger.Debug("Checking for verification badge prompt")
	
	// Wait a bit for the modal to appear (if it's going to)
	time.Sleep(stealth.RandomDelay(1500*time.Millisecond, 2500*time.Millisecond))
	
	// Check if the verification prompt modal is present
	verificationModalSelectors := []string{
		"div[role='dialog']:has-text('Your invitation is sent')",
		"div[role='dialog']:has-text('Verify now')",
		"div.artdeco-modal:has-text('verification')",
	}
	
	var hasModal bool
	for _, selector := range verificationModalSelectors {
		has, _, err := page.Timeout(2 * time.Second).Has(selector)
		if err == nil && has {
			hasModal = true
			logger.Debug("Verification prompt modal detected", "selector", selector)
			break
		}
	}
	
	if !hasModal {
		logger.Debug("No verification prompt detected, continuing")
		return nil
	}
	
	// Find and click "Not now" button
	notNowSelectors := []string{
		"button:has-text('Not now')",
		"button[aria-label='Not now']",
		"div[role='dialog'] button.artdeco-button--secondary:has-text('Not now')",
		"button.artdeco-button--muted:has-text('Not now')",
	}
	
	var notNowButton *rod.Element
	var err error
	
	for _, selector := range notNowSelectors {
		logger.Debug("Trying Not now button selector", "selector", selector)
		notNowButton, err = page.Timeout(3 * time.Second).Element(selector)
		if err == nil {
			logger.Debug("Found Not now button", "selector", selector)
			break
		}
	}
	
	if err != nil {
		logger.Warn("Not now button not found in verification prompt")
		// Try to close modal with X button as fallback
		closeButton, closeErr := page.Timeout(2 * time.Second).Element("button[aria-label='Dismiss']")
		if closeErr == nil {
			if clickErr := closeButton.Click(proto.InputMouseButtonLeft, 1); clickErr == nil {
				logger.Debug("Closed verification prompt with X button")
				time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1000*time.Millisecond))
				return nil
			}
		}
		return fmt.Errorf("could not find way to dismiss verification prompt")
	}
	
	// Small delay before clicking (human behavior)
	time.Sleep(stealth.RandomDelay(800*time.Millisecond, 1500*time.Millisecond))
	
	// Click "Not now" button
	if err := notNowButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		// Try JS click as fallback
		_, jsErr := page.Eval(`(el) => { el.click(); }`, notNowButton)
		if jsErr != nil {
			return fmt.Errorf("failed to click Not now button: normal=%v, js=%v", err, jsErr)
		}
		logger.Debug("Not now button clicked (JS)")
	} else {
		logger.Debug("Not now button clicked (normal)")
	}
	
	// Wait for modal to close
	time.Sleep(stealth.RandomDelay(1000*time.Millisecond, 1500*time.Millisecond))
	
	logger.Info("Successfully dismissed verification prompt")
	return nil
}
