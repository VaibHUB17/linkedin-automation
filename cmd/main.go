package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/yourusername/linkedin-automation/internal/auth"
	"github.com/yourusername/linkedin-automation/internal/config"
	"github.com/yourusername/linkedin-automation/internal/connection"
	"github.com/yourusername/linkedin-automation/internal/logger"
	"github.com/yourusername/linkedin-automation/internal/messaging"
	"github.com/yourusername/linkedin-automation/internal/search"
	"github.com/yourusername/linkedin-automation/internal/storage"
	st "github.com/yourusername/linkedin-automation/internal/stealth"
)

const (
	AppVersion = "1.0.0"
)

func main() {
	// Display warning banner
	displayWarningBanner()

	// Load configuration
	logger.Info("Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	err = logger.Init(cfg.Logging.Level, cfg.Logging.ToFile, cfg.Logging.FilePath)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("LinkedIn Automation POC started", "version", AppVersion)
	logger.Warn("This tool is for EDUCATIONAL purposes only and violates LinkedIn's Terms of Service")

	// Initialize database
	logger.Info("Initializing database...", "path", cfg.Database.Path)
	if err := storage.InitDB(cfg.Database.Path); err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}
	defer storage.Close()

	// Print database statistics
	stats, err := storage.GetStats()
	if err == nil {
		logger.Info("Database statistics",
			"total_profiles", stats["total_profiles"],
			"total_requests", stats["total_requests"],
			"accepted_connections", stats["accepted_connections"],
			"total_messages", stats["total_messages"],
			"requests_today", stats["requests_today"],
		)
	}

	// Launch browser with stealth
	logger.Info("Launching browser with stealth mode...")
	browser, cleanup := launchBrowser(cfg)
	defer cleanup()

	// Authenticate
	logger.Info("Authenticating with LinkedIn...")
	if err := auth.Login(browser, cfg.LinkedIn.Email, cfg.LinkedIn.Password); err != nil {
		logger.Fatal("Authentication failed", "error", err)
	}

	logger.Info("Authentication successful")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a page for automation
	page := browser.MustPage()
	defer page.Close()

	// Apply stealth settings to the page
	st.DisableAutomationFlags(page)
	st.SetRealisticViewport(page)

	logger.Info("Starting main automation loop...")

	// Main automation loop
	cycleCount := 0
	for {
		select {
		case <-sigChan:
			logger.Info("Received shutdown signal, cleaning up...")
			return
		default:
			cycleCount++
			logger.Info("Starting automation cycle", "cycle", cycleCount)

			// Check if within business hours
			// if !cfg.IsBusinessHours() {
			// 	waitDuration := st.TimeUntilBusinessHours(
			// 		cfg.Stealth.BusinessHours.Start,
			// 		cfg.Stealth.BusinessHours.End,
			// 		cfg.Stealth.WorkDays,
			// 	)
			// 	logger.Info("Outside business hours, sleeping...", "wait_duration", waitDuration)
			// 	time.Sleep(waitDuration)
			// 	continue
			// }

			// Check if it's lunch time
			if st.IsLunchTime() {
				logger.Info("Lunch time, taking a break...")
				time.Sleep(1 * time.Hour)
				continue
			}

			// Execute automation cycle with error handling
			if err := runAutomationCycle(page, cfg); err != nil {
				logger.Error("Automation cycle failed", "error", err, "cycle", cycleCount)
				
				// Retry with exponential backoff
				backoffSecs := math.Min(float64(uint(1)<<uint(cycleCount%5)), 300)
				backoff := time.Duration(backoffSecs) * time.Second
				logger.Info("Retrying after backoff", "duration", backoff)
				time.Sleep(backoff)
				continue
			}

			// Determine sleep duration until next cycle
			nextCycleDelay := st.RandomDelay(30*time.Minute, 60*time.Minute)
			logger.Info("Cycle completed successfully, sleeping until next cycle", "duration", nextCycleDelay)
			time.Sleep(nextCycleDelay)
		}
	}
}

// runAutomationCycle executes one complete automation cycle
func runAutomationCycle(page *rod.Page, cfg *config.Config) error {
	logger.Info("=== Starting Automation Cycle ===")

	// Phase 1: Search for profiles
	logger.Info("Phase 1: Searching for profiles...")
	profiles, err := search.SearchAndCollect(page, cfg)
	if err != nil {
		return fmt.Errorf("profile search failed: %w", err)
	}

	logger.Info("Profiles collected", "count", len(profiles))

	if len(profiles) == 0 {
		logger.Warn("No profiles found in search")
	}

	// Random delay after search
	time.Sleep(st.RandomDelay(10*time.Second, 30*time.Second))

	// Phase 2: Send connection requests
	logger.Info("Phase 2: Sending connection requests...")
	requestsSent := 0

	for _, profile := range profiles {
		// Check daily limit
		allowed, remaining, err := connection.CheckDailyLimit(cfg)
		if err != nil {
			logger.Error("Failed to check daily limit", "error", err)
			break
		}

		if !allowed {
			logger.Info("Daily connection request limit reached", "limit", cfg.Connection.DailyLimit)
			break
		}

		logger.Info("Processing profile", "name", profile.Name, "url", profile.URL, "remaining_requests", remaining)

		// Determine if we should add a personalized note
		var note string
		if connection.ShouldAddNote(cfg) {
			// Generate personalized note
			name := profile.Name
			if name == "" {
				name = "there"
			}
			note = connection.GeneratePersonalizedNote(cfg.Connection.NoteTemplate, name, "technology")
		}

		// Send connection request with retry logic
		err = retryWithBackoff(func() error {
			return connection.SendConnectionRequest(page, profile.URL, note, cfg)
		}, 3)

		if err != nil {
			logger.Error("Failed to send connection request", "profile_url", profile.URL, "error", err)
			// Continue with next profile instead of failing entire cycle
			continue
		}

		requestsSent++

		// Delay between connection requests
		delay := st.RandomDelay(cfg.GetMinDelay(), cfg.GetMaxDelay())
		logger.Debug("Waiting before next request", "delay", delay)
		time.Sleep(delay)

		// Take breaks periodically
		if st.ShouldTakeBreak(requestsSent) {
			breakDuration := st.GetBreakDuration()
			logger.Info("Taking a break", "duration", breakDuration)
			time.Sleep(breakDuration)
		}
	}

	logger.Info("Connection requests phase completed", "requests_sent", requestsSent)

	// Random delay between phases
	time.Sleep(st.RandomDelay(20*time.Second, 60*time.Second))

	// Phase 3: Check for new connections and send messages
	logger.Info("Phase 3: Checking for new connections and sending messages...")
	err = retryWithBackoff(func() error {
		return messaging.ProcessNewConnections(page, cfg)
	}, 3)

	if err != nil {
		logger.Error("Failed to process new connections", "error", err)
		// Don't fail the entire cycle for this
	}

	logger.Info("=== Automation Cycle Completed ===")
	return nil
}

// launchBrowser launches a Rod browser with stealth configuration
func launchBrowser(cfg *config.Config) (*rod.Browser, func()) {
	// Try to find local Chrome installation first (avoids leakless.exe issue)
	path, exists := launcher.LookPath()
	var l *launcher.Launcher
	
	if exists {
		logger.Info("Using system Chrome browser", "path", path)
		l = launcher.New().Bin(path)
	} else {
		logger.Info("System Chrome not found, using downloaded browser")
		l = launcher.New()
	}
	
	// Configure launcher
	l = l.Headless(cfg.Stealth.Headless).
		Devtools(false).
		Leakless(false) // Disable leakless to avoid antivirus issues

	// Set user agent
	userAgent := st.RandomizeUserAgent()
	l = l.Set("user-agent", userAgent)

	// Launch browser
	url, err := l.Launch()
	if err != nil {
		logger.Fatal("Failed to launch browser", "error", err)
	}

	// Connect to browser
	browser := rod.New().ControlURL(url).MustConnect()

	// Apply stealth plugin
	if err := st.ApplyStealthSettings(browser); err != nil {
		logger.Warn("Failed to apply stealth settings", "error", err)
	}

	// Apply stealth evasions
	browser = browser.MustIncognito()

	logger.Info("Browser launched successfully", "headless", cfg.Stealth.Headless, "user_agent", userAgent)

	// Return browser and cleanup function
	cleanup := func() {
		logger.Info("Closing browser...")
		browser.MustClose()
	}

	return browser, cleanup
}

// retryWithBackoff executes an operation with exponential backoff retry logic
func retryWithBackoff(operation func() error, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < maxRetries {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			logger.Warn("Operation failed, retrying...",
				"attempt", attempt,
				"max_retries", maxRetries,
				"backoff", backoff,
				"error", err,
			)
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, lastErr)
}

// displayWarningBanner displays a warning about the tool's purpose
func displayWarningBanner() {
	banner := `
╔════════════════════════════════════════════════════════════════════════════╗
║                                                                            ║
║                    ⚠️  WARNING - EDUCATIONAL USE ONLY ⚠️                    ║
║                                                                            ║
║  This LinkedIn automation tool is a PROOF-OF-CONCEPT for educational      ║
║  and demonstration purposes ONLY.                                          ║
║                                                                            ║
║  ❌ This tool VIOLATES LinkedIn's Terms of Service                         ║
║  ❌ Using this on real accounts may result in ACCOUNT BAN                  ║
║  ❌ This is NOT intended for production use                                ║
║                                                                            ║
║  ✅ Use ONLY for learning automation techniques                            ║
║  ✅ Use ONLY on test/dummy accounts                                        ║
║  ✅ Demonstrate technical skills responsibly                               ║
║                                                                            ║
║  By continuing, you acknowledge that you understand these warnings and     ║
║  accept full responsibility for any consequences.                          ║
║                                                                            ║
╚════════════════════════════════════════════════════════════════════════════╝

Press Ctrl+C at any time to stop the automation.

`
	fmt.Println(banner)

	// Wait a few seconds to ensure user sees the warning
	fmt.Println("Starting in 5 seconds...")
	for i := 5; i > 0; i-- {
		fmt.Printf("%d...\n", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println()
}
