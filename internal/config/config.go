package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	LinkedIn  LinkedInConfig  `yaml:"linkedin"`
	Search    SearchConfig    `yaml:"search"`
	Connection ConnectionConfig `yaml:"connection"`
	Messaging MessagingConfig  `yaml:"messaging"`
	Stealth   StealthConfig    `yaml:"stealth"`
	Database  DatabaseConfig   `yaml:"database"`
	Logging   LoggingConfig    `yaml:"logging"`
}

// LinkedInConfig contains LinkedIn credentials
type LinkedInConfig struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
}

// SearchConfig contains search parameters
type SearchConfig struct {
	JobTitles  []string `yaml:"job_titles"`
	Companies  []string `yaml:"companies"`
	Locations  []string `yaml:"locations"`
	Keywords   []string `yaml:"keywords"`
	MaxResults int      `yaml:"max_results"`
	MaxPages   int      `yaml:"max_pages"`
}

// ConnectionConfig contains connection request settings
type ConnectionConfig struct {
	DailyLimit          int     `yaml:"daily_limit"`
	NoteTemplate        string  `yaml:"note_template"`
	MinDelaySeconds     int     `yaml:"min_delay_seconds"`
	MaxDelaySeconds     int     `yaml:"max_delay_seconds"`
	PersonalizationRate float64 `yaml:"personalization_rate"`
}

// MessagingConfig contains messaging settings
type MessagingConfig struct {
	FollowUpTemplate             string `yaml:"follow_up_template"`
	DelayAfterAcceptanceHours    int    `yaml:"delay_after_acceptance_hours"`
	MinDelaySeconds              int    `yaml:"min_delay_seconds"`
	MaxDelaySeconds              int    `yaml:"max_delay_seconds"`
}

// StealthConfig contains anti-detection settings
type StealthConfig struct {
	MinActionDelayMs   int      `yaml:"min_action_delay_ms"`
	MaxActionDelayMs   int      `yaml:"max_action_delay_ms"`
	ScrollSpeedPx      int      `yaml:"scroll_speed_px"`
	TypingSpeedMs      int      `yaml:"typing_speed_ms"`
	BusinessHoursOnly  bool     `yaml:"business_hours_only"`
	BusinessHours      BusinessHours `yaml:"business_hours"`
	WorkDays           []string `yaml:"work_days"`
	TypoRate           float64  `yaml:"typo_rate"`
	Headless           bool     `yaml:"headless"`
}

// BusinessHours defines the operating hours
type BusinessHours struct {
	Start int `yaml:"start"`
	End   int `yaml:"end"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level    string `yaml:"level"`
	ToFile   bool   `yaml:"to_file"`
	FilePath string `yaml:"file_path"`
}

var (
	// Global configuration instance
	globalConfig *Config
)

// Load loads configuration from YAML file and environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (ignore errors if not present)
	_ = godotenv.Load()

	// Read config file
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./config/config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in YAML
	expandedData := expandEnvVars(string(data))

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal([]byte(expandedData), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	globalConfig = &cfg
	return &cfg, nil
}

// Get returns the global configuration instance
func Get() *Config {
	if globalConfig == nil {
		panic("configuration not loaded, call Load() first")
	}
	return globalConfig
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate LinkedIn credentials
	if c.LinkedIn.Email == "" {
		return fmt.Errorf("LinkedIn email is required")
	}
	if c.LinkedIn.Password == "" {
		return fmt.Errorf("LinkedIn password is required")
	}

	// Validate search config
	if c.Search.MaxResults <= 0 {
		return fmt.Errorf("max_results must be positive")
	}
	if c.Search.MaxPages <= 0 {
		return fmt.Errorf("max_pages must be positive")
	}

	// Validate connection config
	if c.Connection.DailyLimit <= 0 {
		return fmt.Errorf("daily_limit must be positive")
	}
	if c.Connection.MinDelaySeconds < 0 {
		return fmt.Errorf("min_delay_seconds must be non-negative")
	}
	if c.Connection.MaxDelaySeconds < c.Connection.MinDelaySeconds {
		return fmt.Errorf("max_delay_seconds must be >= min_delay_seconds")
	}

	// Validate stealth config
	if c.Stealth.BusinessHours.Start < 0 || c.Stealth.BusinessHours.Start > 23 {
		return fmt.Errorf("business hours start must be between 0 and 23")
	}
	if c.Stealth.BusinessHours.End < 0 || c.Stealth.BusinessHours.End > 23 {
		return fmt.Errorf("business hours end must be between 0 and 23")
	}

	// Validate logging config
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	return nil
}

// IsBusinessHours checks if the current time is within business hours
func (c *Config) IsBusinessHours() bool {
	if !c.Stealth.BusinessHoursOnly {
		return true
	}

	now := time.Now()
	
	// Check if it's a work day
	currentDay := now.Weekday().String()
	isWorkDay := false
	for _, day := range c.Stealth.WorkDays {
		if day == currentDay {
			isWorkDay = true
			break
		}
	}
	if !isWorkDay {
		return false
	}

	// Check if it's within business hours
	currentHour := now.Hour()
	return currentHour >= c.Stealth.BusinessHours.Start && currentHour < c.Stealth.BusinessHours.End
}

// expandEnvVars expands environment variables in the format ${VAR} or ${VAR:default}
func expandEnvVars(s string) string {
	// Pattern matches ${VAR} or ${VAR:default}
	pattern := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)
	
	return pattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name and default value
		parts := pattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		
		varName := parts[1]
		defaultValue := ""
		if len(parts) > 2 {
			defaultValue = parts[2]
		}
		
		// Get environment variable value
		value := os.Getenv(varName)
		if value == "" {
			return defaultValue
		}
		return value
	})
}

// GetMinDelay returns the minimum delay duration for actions
func (c *Config) GetMinDelay() time.Duration {
	return time.Duration(c.Connection.MinDelaySeconds) * time.Second
}

// GetMaxDelay returns the maximum delay duration for actions
func (c *Config) GetMaxDelay() time.Duration {
	return time.Duration(c.Connection.MaxDelaySeconds) * time.Second
}

// GetTypingSpeed returns the typing speed as a duration
func (c *Config) GetTypingSpeed() time.Duration {
	return time.Duration(c.Stealth.TypingSpeedMs) * time.Millisecond
}

// ParseTemplate replaces template variables with actual values
func ParseTemplate(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}
