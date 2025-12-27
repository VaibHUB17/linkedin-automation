# LinkedIn Connection Request Flow

## Overview
This document describes how the automation handles sending connection requests with personalized notes based on the LinkedIn profile HTML structure.

## HTML Structure Analysis

Based on the provided LinkedIn profile HTML, the key elements are:

### 1. Add/Connect Button
```html
<button aria-label="Invite Harish B. to connect" 
        class="artdeco-button artdeco-button--2 artdeco-button--primary">
    <svg data-test-icon="connect-small"></svg>
    <span class="artdeco-button__text">Add</span>
</button>
```

**Key Identifiers:**
- `aria-label`: Contains "Invite" and "connect"
- Class: `artdeco-button--2 artdeco-button--primary`
- Text content: "Add"
- SVG icon: `data-test-icon="connect-small"`

### 2. Message Button (Sibling Element)
```html
<button aria-label="Message Harish" 
        class="artdeco-button artdeco-button--2 artdeco-button--secondary">
    <span class="artdeco-button__text">Message</span>
</button>
```

### 3. More Actions Menu
```html
<div class="artdeco-dropdown">
    <button aria-label="More actions" 
            class="artdeco-dropdown__trigger">
        <span>More</span>
    </button>
</div>
```

## Implementation Flow

### Phase 1: Click the "Add" Button

The implementation uses multiple selector strategies to find the correct button:

```go
connectSelectors := []string{
    // Exact match for LinkedIn's Add/Invite button structure
    "button.artdeco-button.artdeco-button--2.artdeco-button--primary[aria-label*='Invite'][aria-label*='connect']",
    // Alternative with text content
    "button.artdeco-button--primary.artdeco-button--2:has-text('Add')",
    // With SVG icon for Connect
    "button.artdeco-button--primary[aria-label*='Invite'] svg[data-test-icon='connect-small']",
    // Primary button near Message button
    "button.artdeco-button--primary[aria-label*='Invite']:has-text('Add')",
    // Fallback
    "button[data-control-name='connect']",
}
```

**Key Features:**
- Removes overlay blockers (messaging bubbles, toasts) before clicking
- Scrolls button into view
- Uses human-like timing delays
- Falls back to JavaScript click if normal click fails
- Verifies the connection dialog appeared after clicking

### Phase 2: Add a Note (Optional)

After clicking "Add", LinkedIn shows a modal dialog with options:
1. **Send without a note** (direct send)
2. **Add a note** (opens textarea for personalized message)

The implementation checks for the "Add a note" button:

```go
noteSelectors := []string{
    "div[role='dialog'] button[aria-label='Add a note']",
    "div.send-invite button:has-text('Add a note')",
    "button.artdeco-button--secondary:has-text('Add a note')",
    "button:has-text('Add a note')",
    "button[aria-label='Add a note']",
}
```

### Phase 3: Write Personalized Note

Once the textarea appears, the implementation:

1. **Finds the textarea:**
   ```go
   noteSelectors := []string{
       "textarea[name='message']",
       "textarea[id*='custom-message']",
       "textarea.send-invite__custom-message",
       "textarea#custom-message",
       "textarea[aria-label*='note']",
   }
   ```

2. **Types with realistic behavior:**
   - Variable delays between keystrokes (150-200ms base)
   - Occasional typos (2% chance) followed by backspace
   - Random pauses between words (10% chance for 500-1500ms)
   - Slower typing at the beginning
   - Respects 300-character limit

3. **Default message (50 words):**
   ```
   "Hi, I came across your profile and was impressed by your background 
   and experience. I'd love to connect and exchange insights about the 
   industry. I believe we could have valuable discussions and potentially 
   explore collaboration opportunities. Looking forward to connecting with 
   you and learning from your expertise."
   ```

### Phase 4: Send Connection Request

The implementation looks for the Send button:

```go
sendSelectors := []string{
    "div[role='dialog'] button.artdeco-button--primary[aria-label*='Send']",
    "button.artdeco-button--primary:has-text('Send')",
    "button[aria-label='Send invitation']",
    "button[aria-label='Send now']",
    "button:has-text('Send without a note')",
    "button:has-text('Send')",
}
```

**Safety Features:**
- Checks daily limit before sending
- Records all sent requests in database
- Adds random delays to mimic human behavior
- Waits for modal to close after sending

## Stealth Features

The implementation includes multiple stealth mechanisms:

1. **Natural scrolling** - Scrolls profile before clicking connect
2. **Random delays** - Variable timing between actions
3. **Human-like typing** - Realistic keystroke timing with occasional typos
4. **Overlay removal** - Clears blocking elements before interactions
5. **Rate limiting** - Respects daily connection limits

## Error Handling

The implementation handles various scenarios:

- **Button not found** - Tries multiple selectors and detection methods
- **Daily limit reached** - Stops sending and returns error
- **Modal didn't appear** - Verifies dialog presence after click
- **Note option unavailable** - Sends without note if "Add a note" isn't present
- **Profile requires Follow first** - Detects when connect isn't available

## Configuration

Relevant configuration options:

```yaml
connection:
  daily_limit: 50          # Max connections per day
  personalization_rate: 0.8 # 80% of requests include personalized note
  default_note: "..."       # Default 50-word message
```

## Usage Example

```go
// Send connection request with custom note
note := "Hi John, I noticed we both work in AI/ML. Would love to connect!"
err := connection.SendConnectionRequest(page, profileURL, note, cfg)

// Send with default 50-word note
err := connection.SendConnectionRequest(page, profileURL, "", cfg)
```

## Testing Considerations

When testing this flow:

1. **Test with various profile types:**
   - 1st degree connections (should show "Message")
   - 2nd degree connections (should show "Add")
   - 3rd degree connections (may require "Follow" first)

2. **Test modal variations:**
   - Profiles that allow immediate send
   - Profiles that require email verification
   - Profiles that only allow "Follow"

3. **Monitor rate limits:**
   - Track daily request counts
   - Respect LinkedIn's limits to avoid restrictions

## Important Notes

⚠️ **LinkedIn Detection:** LinkedIn has sophisticated bot detection. The implementation includes stealth features, but users should:
- Use reasonable daily limits (20-50 connections/day)
- Add random delays between profiles
- Personalize messages when possible
- Avoid repetitive patterns
- Use authenticated sessions with cookies

⚠️ **Terms of Service:** Automating LinkedIn actions may violate their Terms of Service. This tool is for educational purposes only.
