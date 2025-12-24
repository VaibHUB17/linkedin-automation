package messaging

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

// CheckNewConnections checks for newly accepted connection requests
func CheckNewConnections(page *rod.Page) ([]storage.ConnectionRequest, error) {
	logger.Info("Checking for new connections")

	// Get pending connection requests from database
	pendingRequests, err := storage.GetPendingRequests()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending requests: %w", err)
	}

	if len(pendingRequests) == 0 {
		logger.Debug("No pending connection requests")
		return []storage.ConnectionRequest{}, nil
	}

	logger.Debug("Found pending requests", "count", len(pendingRequests))

	// Navigate to My Network page to check connections
	logger.Debug("Navigating to My Network")
	if err := page.Navigate("https://www.linkedin.com/mynetwork/"); err != nil {
		return nil, fmt.Errorf("failed to navigate to My Network: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("failed to wait for page load: %w", err)
	}

	time.Sleep(stealth.RandomDelay(2*time.Second, 4*time.Second))

	// Check each pending request to see if it was accepted
	var newConnections []storage.ConnectionRequest

	for _, request := range pendingRequests {
		// Check if connection exists (you would typically verify by visiting profile
		// or checking the connections list, but this is simplified)
		
		// For demonstration, we'll check if the request is older than the acceptance delay
		// In a real implementation, you'd verify the actual connection status
		timeSinceSent := time.Since(request.SentAt)
		
		// If request is recent (< 24 hours), skip for now
		if timeSinceSent < 24*time.Hour {
			continue
		}

		// In a production implementation, you would:
		// 1. Navigate to the person's profile
		// 2. Check if the "Connect" button changed to "Message" or shows "Connected"
		// 3. Or check your connections list
		
		// For this POC, we'll mark it as a new connection if it's old enough
		// This is a simplified approach for demonstration
		
		logger.Debug("Potential new connection detected", "profile_url", request.ProfileURL)
		newConnections = append(newConnections, request)
	}

	logger.Info("New connections found", "count", len(newConnections))
	return newConnections, nil
}

// SendMessage sends a message to a connection
func SendMessage(page *rod.Page, profileURL, message string, cfg *config.Config) error {
	logger.Info("Sending message", "profile_url", profileURL)

	// Check if we've already sent a message to this profile
	alreadySent, err := storage.HasSentMessage(profileURL)
	if err != nil {
		return fmt.Errorf("failed to check message history: %w", err)
	}

	if alreadySent {
		logger.Info("Message already sent to this profile, skipping", "profile_url", profileURL)
		return nil
	}

	// Navigate to profile
	logger.Debug("Navigating to profile", "url", profileURL)
	if err := page.Navigate(profileURL); err != nil {
		return fmt.Errorf("failed to navigate to profile: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("failed to wait for page load: %w", err)
	}

	time.Sleep(stealth.RandomDelay(2*time.Second, 4*time.Second))

	// Find and click Message button
	if err := clickMessageButton(page); err != nil {
		return fmt.Errorf("failed to click message button: %w", err)
	}

	// Wait for message dialog/thread to open
	time.Sleep(stealth.RandomDelay(2*time.Second, 3*time.Second))

	// Type the message
	if err := typeMessage(page, message); err != nil {
		return fmt.Errorf("failed to type message: %w", err)
	}

	// Send the message
	if err := clickSendMessageButton(page); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Record the sent message
	err = storage.RecordMessage(storage.Message{
		ProfileURL: profileURL,
		Message:    message,
		SentAt:     time.Now(),
	})
	if err != nil {
		logger.Error("Failed to record message", "error", err)
	}

	// Record action for rate limiting
	err = storage.RecordAction("message_sent")
	if err != nil {
		logger.Error("Failed to record action", "error", err)
	}

	logger.Info("Message sent successfully", "profile_url", profileURL)
	return nil
}

// clickMessageButton finds and clicks the Message button on a profile
func clickMessageButton(page *rod.Page) error {
	messageSelectors := []string{
		"button[aria-label*='Message']",
		"button:has-text('Message')",
		".pvs-profile-actions__action:has-text('Message')",
	}

	var messageButton *rod.Element
	var err error

	for _, selector := range messageSelectors {
		messageButton, err = page.Element(selector)
		if err == nil {
			break
		}
	}

	if err != nil {
		return fmt.Errorf("message button not found (profile may not be a connection)")
	}

	// Get button position
	shape, err := messageButton.Shape()
	if err != nil {
		return fmt.Errorf("failed to get button shape: %w", err)
	}
	box := shape.Box()

	// Move mouse to button
	if err := stealth.MoveMouse(page, box.X+box.Width/2, box.Y+box.Height/2); err != nil {
		logger.Warn("Failed to move mouse to button", "error", err)
	}

	time.Sleep(stealth.RandomDelay(300*time.Millisecond, 800*time.Millisecond))

	// Click
	if err := messageButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click message button: %w", err)
	}

	logger.Debug("Message button clicked")
	return nil
}

// typeMessage types a message in the message input field
func typeMessage(page *rod.Page, message string) error {
	logger.Debug("Typing message")

	// Find message input field
	messageSelectors := []string{
		"div[role='textbox'][aria-label*='Write a message']",
		"div.msg-form__contenteditable",
		"div[contenteditable='true'][role='textbox']",
	}

	var messageField *rod.Element
	var err error

	for _, selector := range messageSelectors {
		messageField, err = page.Element(selector)
		if err == nil {
			break
		}
	}

	if err != nil {
		return fmt.Errorf("message field not found")
	}

	// Click the field
	if err := messageField.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click message field: %w", err)
	}

	time.Sleep(stealth.ShortDelay())

	// Type message with realistic timing
	words := strings.Split(message, " ")
	for i, word := range words {
		// Type word character by character
		for _, char := range word {
			messageField.Input(string(char))
			
			// Variable delay between keystrokes
			delay := stealth.RandomDelay(100*time.Millisecond, 250*time.Millisecond)
			time.Sleep(delay)
		}

		// Add space after word (except last word)
		if i < len(words)-1 {
			messageField.Input(" ")
			time.Sleep(stealth.RandomDelay(100*time.Millisecond, 300*time.Millisecond))
		}

		// Occasional pause between words (thinking)
		if i > 0 && i%5 == 0 { // Every 5 words
			time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1500*time.Millisecond))
		}
	}

	logger.Debug("Message typed successfully")
	return nil
}

// clickSendMessageButton clicks the Send button to send the message
func clickSendMessageButton(page *rod.Page) error {
	sendSelectors := []string{
		"button[type='submit'][aria-label*='Send']",
		"button.msg-form__send-button",
		"button:has-text('Send')",
	}

	var sendButton *rod.Element
	var err error

	for _, selector := range sendSelectors {
		sendButton, err = page.Element(selector)
		if err == nil {
			break
		}
	}

	if err != nil {
		return fmt.Errorf("send button not found")
	}

	// Brief delay before sending
	time.Sleep(stealth.RandomDelay(500*time.Millisecond, 1500*time.Millisecond))

	// Click send button
	if err := sendButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click send button: %w", err)
	}

	logger.Debug("Send button clicked")

	// Wait for message to be sent
	time.Sleep(stealth.RandomDelay(1*time.Second, 2*time.Second))

	return nil
}

// RenderTemplate replaces template variables with actual values
func RenderTemplate(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// ProcessNewConnections checks for new connections and sends follow-up messages
func ProcessNewConnections(page *rod.Page, cfg *config.Config) error {
	// Check for new connections
	newConnections, err := CheckNewConnections(page)
	if err != nil {
		return fmt.Errorf("failed to check new connections: %w", err)
	}

	if len(newConnections) == 0 {
		logger.Info("No new connections to message")
		return nil
	}

	logger.Info("Processing new connections", "count", len(newConnections))

	// Send follow-up messages to new connections
	for _, conn := range newConnections {
		// Check if enough time has passed since acceptance
		timeSinceAcceptance := time.Since(conn.SentAt)
		minDelay := time.Duration(cfg.Messaging.DelayAfterAcceptanceHours) * time.Hour
		
		if timeSinceAcceptance < minDelay {
			logger.Debug("Skipping - too soon after connection", "profile_url", conn.ProfileURL)
			continue
		}

		// Extract name from profile URL or database
		// In a real implementation, you'd get the name from the profile
		name := "there" // Fallback

		// Render message template
		message := RenderTemplate(cfg.Messaging.FollowUpTemplate, map[string]string{
			"name": name,
		})

		// Send message
		err := SendMessage(page, conn.ProfileURL, message, cfg)
		if err != nil {
			logger.Error("Failed to send message", "profile_url", conn.ProfileURL, "error", err)
			continue
		}

		// Mark request as accepted in database
		err = storage.MarkRequestAccepted(conn.ProfileURL)
		if err != nil {
			logger.Error("Failed to mark request as accepted", "profile_url", conn.ProfileURL, "error", err)
		}

		// Delay between messages
		delay := stealth.RandomDelay(
			time.Duration(cfg.Messaging.MinDelaySeconds)*time.Second,
			time.Duration(cfg.Messaging.MaxDelaySeconds)*time.Second,
		)
		logger.Debug("Waiting before next message", "delay", delay)
		time.Sleep(delay)
	}

	return nil
}
