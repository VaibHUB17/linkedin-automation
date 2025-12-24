package stealth

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

var (
	rng *rand.Rand
)

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// Point represents a 2D coordinate
type Point struct {
	X float64
	Y float64
}

// ============================================================================
// MANDATORY TECHNIQUE A: Human-like Mouse Movement with Bézier Curves
// ============================================================================

// MoveMouse moves the mouse to target coordinates using a Bézier curve path
func MoveMouse(page *rod.Page, targetX, targetY float64) error {
	// Get current mouse position (approximate)
	currentX := rng.Float64() * 100 // Simplified; in production, track actual position
	currentY := rng.Float64() * 100

	start := Point{X: currentX, Y: currentY}
	end := Point{X: targetX, Y: targetY}

	// Generate control points for cubic Bézier curve
	controlPoints := generateControlPoints(start, end)

	// Generate path points
	pathPoints := CubicBezierCurve(start, end, controlPoints[0], controlPoints[1], 50)

	// Move mouse along the path with variable speed
	for i, point := range pathPoints {
		// Calculate delay based on position (slower at start/end, faster in middle)
		progress := float64(i) / float64(len(pathPoints))
		delay := calculateMouseDelay(progress)

		// Move to point using Dispatch
		_ = proto.InputDispatchMouseEvent{
			Type: proto.InputDispatchMouseEventTypeMouseMoved,
			X:    point.X,
			Y:    point.Y,
		}.Call(page)

		time.Sleep(delay)
	}

	// Add slight overshoot and correction for realism
	if rng.Float64() < 0.3 { // 30% chance of overshoot
		overshootX := targetX + (rng.Float64()-0.5)*5
		overshootY := targetY + (rng.Float64()-0.5)*5
		_ = proto.InputDispatchMouseEvent{
			Type: proto.InputDispatchMouseEventTypeMouseMoved,
			X:    overshootX,
			Y:    overshootY,
		}.Call(page)
		time.Sleep(RandomDelay(10*time.Millisecond, 30*time.Millisecond))
		_ = proto.InputDispatchMouseEvent{
			Type: proto.InputDispatchMouseEventTypeMouseMoved,
			X:    targetX,
			Y:    targetY,
		}.Call(page)
	}

	return nil
}

// CubicBezierCurve generates points along a cubic Bézier curve
func CubicBezierCurve(start, end, control1, control2 Point, steps int) []Point {
	points := make([]Point, steps)

	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		
		// Cubic Bézier formula
		// B(t) = (1-t)³P₀ + 3(1-t)²tP₁ + 3(1-t)t²P₂ + t³P₃
		oneMinusT := 1 - t
		
		points[i] = Point{
			X: math.Pow(oneMinusT, 3)*start.X +
				3*math.Pow(oneMinusT, 2)*t*control1.X +
				3*oneMinusT*math.Pow(t, 2)*control2.X +
				math.Pow(t, 3)*end.X,
			Y: math.Pow(oneMinusT, 3)*start.Y +
				3*math.Pow(oneMinusT, 2)*t*control1.Y +
				3*oneMinusT*math.Pow(t, 2)*control2.Y +
				math.Pow(t, 3)*end.Y,
		}
	}

	return points
}

// generateControlPoints generates natural control points for Bézier curve
func generateControlPoints(start, end Point) []Point {
	// Calculate distance and angle
	dx := end.X - start.X
	dy := end.Y - start.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// Generate control points with some randomness
	angle := math.Atan2(dy, dx)
	perpAngle := angle + math.Pi/2

	// Control point 1: roughly 1/3 of the way with perpendicular offset
	offset1 := (rng.Float64() - 0.5) * distance * 0.3
	cp1 := Point{
		X: start.X + dx/3 + math.Cos(perpAngle)*offset1,
		Y: start.Y + dy/3 + math.Sin(perpAngle)*offset1,
	}

	// Control point 2: roughly 2/3 of the way with perpendicular offset
	offset2 := (rng.Float64() - 0.5) * distance * 0.3
	cp2 := Point{
		X: start.X + 2*dx/3 + math.Cos(perpAngle)*offset2,
		Y: start.Y + 2*dy/3 + math.Sin(perpAngle)*offset2,
	}

	return []Point{cp1, cp2}
}

// calculateMouseDelay calculates delay based on progress (ease-in-ease-out)
func calculateMouseDelay(progress float64) time.Duration {
	// Slower at start and end, faster in the middle
	speed := 1 - math.Abs(2*progress-1) // 0 at edges, 1 in middle
	baseDelay := 10 * time.Millisecond
	return time.Duration(float64(baseDelay) / (speed + 0.5))
}

// ============================================================================
// MANDATORY TECHNIQUE B: Randomized Timing Patterns
// ============================================================================

// RandomDelay returns a random duration between min and max
func RandomDelay(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	delta := max - min
	return min + time.Duration(rng.Int63n(int64(delta)))
}

// ThinkPause simulates human thinking time
func ThinkPause() {
	delay := RandomDelay(2*time.Second, 8*time.Second)
	time.Sleep(delay)
}

// ReadingDelay simulates time spent reading content
func ReadingDelay(contentLength int) time.Duration {
	// Approximate reading speed: 200-250 words per minute
	// Assume ~5 characters per word
	words := contentLength / 5
	readingTimeMs := float64(words) * (60000.0 / 225.0) // 225 words per minute

	// Add some randomness (±30%)
	variation := 0.3
	randomFactor := 1.0 + (rng.Float64()*2-1)*variation

	return time.Duration(readingTimeMs*randomFactor) * time.Millisecond
}

// ActionDelay returns a delay appropriate for between actions
func ActionDelay() time.Duration {
	return RandomDelay(1*time.Second, 5*time.Second)
}

// ShortDelay returns a short random delay
func ShortDelay() time.Duration {
	return RandomDelay(100*time.Millisecond, 500*time.Millisecond)
}

// ============================================================================
// MANDATORY TECHNIQUE C: Browser Fingerprint Masking
// ============================================================================

// ApplyStealthSettings applies all stealth techniques to the browser
func ApplyStealthSettings(browser *rod.Browser) error {
	// Apply go-rod/stealth plugin (handles many fingerprint issues)
	page, err := stealth.Page(browser)
	if err != nil {
		return fmt.Errorf("failed to apply stealth: %w", err)
	}
	page.Close()

	return nil
}

// DisableAutomationFlags disables automation detection flags
func DisableAutomationFlags(page *rod.Page) error {
	// Disable navigator.webdriver flag
	_, err := page.Eval(`() => {
		Object.defineProperty(navigator, 'webdriver', {
			get: () => false
		});
	}`)
	if err != nil {
		return fmt.Errorf("failed to disable webdriver flag: %w", err)
	}

	// Mask automation-related properties
	_, err = page.Eval(`() => {
		// Override the permissions API
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' ?
				Promise.resolve({ state: Notification.permission }) :
				originalQuery(parameters)
		);

		// Mock chrome runtime
		window.chrome = {
			runtime: {}
		};

		// Override plugin detection
		Object.defineProperty(navigator, 'plugins', {
			get: () => [
				{
					0: {type: "application/x-google-chrome-pdf", suffixes: "pdf", description: "Portable Document Format"},
					description: "Portable Document Format",
					filename: "internal-pdf-viewer",
					length: 1,
					name: "Chrome PDF Plugin"
				}
			]
		});
	}`)
	if err != nil {
		return fmt.Errorf("failed to mask automation properties: %w", err)
	}

	return nil
}

// RandomizeUserAgent returns a randomized but realistic user agent
func RandomizeUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	return userAgents[rng.Intn(len(userAgents))]
}

// SetRealisticViewport sets a realistic viewport size
func SetRealisticViewport(page *rod.Page) error {
	viewports := []struct{ Width, Height int }{
		{1920, 1080},
		{1366, 768},
		{1536, 864},
		{1440, 900},
		{1280, 720},
	}

	viewport := viewports[rng.Intn(len(viewports))]
	return page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  viewport.Width,
		Height: viewport.Height,
	})
}

// ============================================================================
// ADDITIONAL TECHNIQUE D: Random Scrolling Behavior
// ============================================================================

// ScrollPage scrolls the page in a natural way
func ScrollPage(page *rod.Page, direction string) error {
	scrollAmount := 50 + rng.Intn(150) // 50-200px

	if direction == "up" {
		scrollAmount = -scrollAmount
	}

	// Sometimes scroll in multiple steps
	steps := 1 + rng.Intn(3) // 1-3 steps
	stepAmount := scrollAmount / steps

	for i := 0; i < steps; i++ {
		_, err := page.Eval(fmt.Sprintf(`() => window.scrollBy(0, %d)`, stepAmount))
		if err != nil {
			return fmt.Errorf("failed to scroll: %w", err)
		}

		// Small delay between scroll steps
		time.Sleep(RandomDelay(50*time.Millisecond, 150*time.Millisecond))
	}

	// Occasionally scroll back slightly (natural correction)
	if rng.Float64() < 0.15 { // 15% chance
		correction := rng.Intn(30)
		page.Eval(fmt.Sprintf(`() => window.scrollBy(0, %d)`, -correction))
		time.Sleep(ShortDelay())
	}

	return nil
}

// ScrollToElement scrolls to bring an element into view naturally
func ScrollToElement(page *rod.Page, selector string) error {
	// Get element position
	element, err := page.Element(selector)
	if err != nil {
		return fmt.Errorf("element not found: %w", err)
	}

	// Get element's position
	shape, err := element.Shape()
	if err != nil {
		return fmt.Errorf("failed to get element shape: %w", err)
	}
	box := shape.Box()

	// Scroll in steps to the element
	currentScroll, err := page.Eval(`() => window.pageYOffset`)
	if err != nil {
		return err
	}

	targetScroll := box.Y - 200 // Leave some space above element
	scrollDistance := int(targetScroll - float64(currentScroll.Value.Int()))

	// Scroll in multiple steps
	steps := 3 + rng.Intn(5) // 3-7 steps
	for i := 0; i < steps; i++ {
		stepScroll := scrollDistance / steps
		page.Eval(fmt.Sprintf(`() => window.scrollBy(0, %d)`, stepScroll))
		time.Sleep(RandomDelay(100*time.Millisecond, 300*time.Millisecond))
	}

	return nil
}

// ScrollFeed scrolls through a feed naturally (for LinkedIn feed)
func ScrollFeed(page *rod.Page, scrollCount int) error {
	for i := 0; i < scrollCount; i++ {
		// Scroll down
		err := ScrollPage(page, "down")
		if err != nil {
			return err
		}

		// Pause to "read" content
		readDelay := RandomDelay(1*time.Second, 4*time.Second)
		time.Sleep(readDelay)

		// Occasionally scroll up slightly
		if rng.Float64() < 0.2 { // 20% chance
			ScrollPage(page, "up")
			time.Sleep(ShortDelay())
		}
	}

	return nil
}

// ============================================================================
// ADDITIONAL TECHNIQUE E: Realistic Typing Simulation
// ============================================================================

// TypeText types text into an input field with human-like characteristics
func TypeText(page *rod.Page, selector, text string) error {
	element, err := page.Element(selector)
	if err != nil {
		return fmt.Errorf("element not found: %w", err)
	}

	// Click the element first
	err = element.Click(proto.InputMouseButtonLeft, 1)
	if err != nil {
		return fmt.Errorf("failed to click element: %w", err)
	}

	time.Sleep(ShortDelay())

	// Type each character with variable delays
	for i, char := range text {
		// Occasionally introduce typo
		if rng.Float64() < 0.02 { // 2% typo rate
			// Type wrong character
			wrongChar := randomTypo(char)
			element.Input(string(wrongChar))
			time.Sleep(RandomDelay(100*time.Millisecond, 200*time.Millisecond))

			// Backspace
			page.Keyboard.Press(input.Backspace)
			time.Sleep(RandomDelay(100*time.Millisecond, 200*time.Millisecond))
		}

		// Type correct character
		element.Input(string(char))

		// Variable keystroke delay
		delay := calculateKeystrokeDelay(i, len(text))
		time.Sleep(delay)
	}

	return nil
}

// calculateKeystrokeDelay calculates realistic delay between keystrokes
func calculateKeystrokeDelay(position, totalLength int) time.Duration {
	baseDelay := 150 * time.Millisecond

	// Slower at the beginning (thinking about what to write)
	if position < 5 {
		baseDelay = 200 * time.Millisecond
	}

	// Occasional longer pauses (thinking mid-sentence)
	if rng.Float64() < 0.1 { // 10% chance
		baseDelay = RandomDelay(300*time.Millisecond, 800*time.Millisecond)
	}

	// Add randomness (±40%)
	variation := 0.4
	randomFactor := 1.0 + (rng.Float64()*2-1)*variation

	return time.Duration(float64(baseDelay) * randomFactor)
}

// randomTypo returns a typo for a character (adjacent key on QWERTY keyboard)
func randomTypo(char rune) rune {
	typoMap := map[rune][]rune{
		'a': {'s', 'q', 'w', 'z'},
		'b': {'v', 'g', 'h', 'n'},
		'c': {'x', 'd', 'f', 'v'},
		'd': {'s', 'e', 'r', 'f', 'c', 'x'},
		'e': {'w', 'r', 'd', 's'},
		'f': {'d', 'r', 't', 'g', 'v', 'c'},
		'g': {'f', 't', 'y', 'h', 'b', 'v'},
		'h': {'g', 'y', 'u', 'j', 'n', 'b'},
		'i': {'u', 'o', 'k', 'j'},
		'j': {'h', 'u', 'i', 'k', 'm', 'n'},
		'k': {'j', 'i', 'o', 'l', 'm'},
		'l': {'k', 'o', 'p'},
		'm': {'n', 'j', 'k'},
		'n': {'b', 'h', 'j', 'm'},
		'o': {'i', 'p', 'l', 'k'},
		'p': {'o', 'l'},
		'q': {'w', 'a'},
		'r': {'e', 't', 'f', 'd'},
		's': {'a', 'w', 'e', 'd', 'x', 'z'},
		't': {'r', 'y', 'g', 'f'},
		'u': {'y', 'i', 'j', 'h'},
		'v': {'c', 'f', 'g', 'b'},
		'w': {'q', 'e', 's', 'a'},
		'x': {'z', 's', 'd', 'c'},
		'y': {'t', 'u', 'h', 'g'},
		'z': {'a', 's', 'x'},
	}

	lowerChar := rune(strings.ToLower(string(char))[0])
	if typos, ok := typoMap[lowerChar]; ok && len(typos) > 0 {
		return typos[rng.Intn(len(typos))]
	}

	return char
}

// ============================================================================
// ADDITIONAL TECHNIQUE F: Mouse Hovering & Movement
// ============================================================================

// RandomHover performs a random hover over elements on the page
func RandomHover(page *rod.Page) error {
	// Get all hoverable elements
	elements, err := page.Elements("a, button, input, div[role='button']")
	if err != nil || len(elements) == 0 {
		return nil // Not critical, skip if no elements
	}

	// Pick random element
	element := elements[rng.Intn(len(elements))]

	// Get element position
	shape, err := element.Shape()
	if err != nil {
		return nil // Not critical
	}
	box := shape.Box()

	// Move mouse to element
	targetX := box.X + box.Width/2
	targetY := box.Y + box.Height/2

	err = MoveMouse(page, targetX, targetY)
	if err != nil {
		return nil // Not critical
	}

	// Hover for random duration
	hoverTime := RandomDelay(500*time.Millisecond, 2*time.Second)
	time.Sleep(hoverTime)

	return nil
}

// HoverElement hovers over a specific element
func HoverElement(page *rod.Page, selector string) error {
	element, err := page.Element(selector)
	if err != nil {
		return fmt.Errorf("element not found: %w", err)
	}

	shape, err := element.Shape()
	if err != nil {
		return fmt.Errorf("failed to get element shape: %w", err)
	}
	box := shape.Box()

	// Move to element center with slight randomization
	targetX := box.X + box.Width/2 + (rng.Float64()-0.5)*10
	targetY := box.Y + box.Height/2 + (rng.Float64()-0.5)*10

	return MoveMouse(page, targetX, targetY)
}

// CursorWander makes the cursor wander naturally during idle time
func CursorWander(page *rod.Page) error {
	// Move to random positions
	wanderCount := 2 + rng.Intn(4) // 2-5 movements

	for i := 0; i < wanderCount; i++ {
		// Get viewport size
		viewport, err := page.Eval(`() => ({width: window.innerWidth, height: window.innerHeight})`)
		if err != nil {
			return err
		}

		width := viewport.Value.Get("width").Int()
		height := viewport.Value.Get("height").Int()

		// Random position within viewport
		targetX := float64(rng.Intn(width))
		targetY := float64(rng.Intn(height))

		err = MoveMouse(page, targetX, targetY)
		if err != nil {
			return err
		}

		time.Sleep(RandomDelay(500*time.Millisecond, 1500*time.Millisecond))
	}

	return nil
}

// ============================================================================
// ADDITIONAL TECHNIQUE G: Activity Scheduling
// ============================================================================

// IsBusinessHours checks if current time is within business hours
func IsBusinessHours(startHour, endHour int, workDays []string) bool {
	now := time.Now()

	// Check day of week
	currentDay := now.Weekday().String()
	isWorkDay := false
	for _, day := range workDays {
		if day == currentDay {
			isWorkDay = true
			break
		}
	}

	if !isWorkDay {
		return false
	}

	// Check hour
	currentHour := now.Hour()
	return currentHour >= startHour && currentHour < endHour
}

// IsLunchTime checks if it's lunch time (12 PM - 1 PM)
func IsLunchTime() bool {
	hour := time.Now().Hour()
	return hour == 12
}

// TimeUntilBusinessHours returns duration until next business hours
func TimeUntilBusinessHours(startHour, endHour int, workDays []string) time.Duration {
	now := time.Now()
	
	// If currently in business hours, return 0
	if IsBusinessHours(startHour, endHour, workDays) {
		return 0
	}

	// Calculate next business hours start time
	next := now
	
	// If after business hours today, try tomorrow
	if now.Hour() >= endHour {
		next = next.Add(24 * time.Hour)
	}

	// Find next work day
	for i := 0; i < 7; i++ {
		dayName := next.Weekday().String()
		isWorkDay := false
		for _, day := range workDays {
			if day == dayName {
				isWorkDay = true
				break
			}
		}

		if isWorkDay {
			// Set to start hour
			next = time.Date(next.Year(), next.Month(), next.Day(), startHour, 0, 0, 0, next.Location())
			if next.After(now) {
				return next.Sub(now)
			}
		}

		next = next.Add(24 * time.Hour)
	}

	// Fallback: 1 hour
	return 1 * time.Hour
}

// ScheduleNextAction determines when the next action should occur
func ScheduleNextAction(minDelay, maxDelay time.Duration) time.Time {
	delay := RandomDelay(minDelay, maxDelay)
	return time.Now().Add(delay)
}

// ============================================================================
// ADDITIONAL TECHNIQUE H: Rate Limiting & Throttling
// ============================================================================

// RateLimiter tracks action counts for rate limiting
type RateLimiter struct {
	DailyLimit  int
	HourlyLimit int
}

// CheckDailyLimit checks if daily limit has been reached (implemented in storage layer)
// This is a helper function that coordinates with storage
func CheckDailyLimit(currentCount, limit int) (allowed bool, remaining int) {
	if currentCount >= limit {
		return false, 0
	}
	return true, limit - currentCount
}

// CheckHourlyLimit checks if hourly limit has been reached
func CheckHourlyLimit(currentCount, limit int) (allowed bool, remaining int) {
	if currentCount >= limit {
		return false, 0
	}
	return true, limit - currentCount
}

// GetCooldownDuration returns cooldown duration based on current activity
func GetCooldownDuration(actionsThisHour int) time.Duration {
	// Increase cooldown as more actions are performed
	baseCooldown := 2 * time.Minute

	if actionsThisHour > 15 {
		baseCooldown = 5 * time.Minute
	} else if actionsThisHour > 10 {
		baseCooldown = 3 * time.Minute
	}

	// Add randomness
	return RandomDelay(baseCooldown, baseCooldown*2)
}

// ShouldTakeBreak determines if a break should be taken
func ShouldTakeBreak(actionCount int) bool {
	// Take break every 20-30 actions
	if actionCount > 0 && actionCount%25 == 0 {
		return rng.Float64() < 0.7 // 70% chance
	}
	return false
}

// GetBreakDuration returns how long to break for
func GetBreakDuration() time.Duration {
	// Break for 10-30 minutes
	return RandomDelay(10*time.Minute, 30*time.Minute)
}
