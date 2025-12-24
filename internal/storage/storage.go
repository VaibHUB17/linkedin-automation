package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

var (
	db *sql.DB
)

// Profile represents a LinkedIn profile
type Profile struct {
	ID           int64
	ProfileURL   string
	Name         string
	Headline     string
	DiscoveredAt time.Time
}

// ConnectionRequest represents a sent connection request
type ConnectionRequest struct {
	ID         int64
	ProfileURL string
	Note       string
	SentAt     time.Time
	Accepted   bool
	AcceptedAt *time.Time
}

// Message represents a sent message
type Message struct {
	ID         int64
	ProfileURL string
	Message    string
	SentAt     time.Time
}

// Action represents any action performed
type Action struct {
	ID         int64
	ActionType string
	Timestamp  time.Time
}

// InitDB initializes the database connection and creates tables
func InitDB(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Create tables
	if err := createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

// createTables creates the required database tables
func createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS profiles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		profile_url TEXT UNIQUE NOT NULL,
		name TEXT,
		headline TEXT,
		discovered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS connection_requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		profile_url TEXT NOT NULL,
		note TEXT,
		sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		accepted BOOLEAN DEFAULT FALSE,
		accepted_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		profile_url TEXT NOT NULL,
		message TEXT NOT NULL,
		sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS actions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action_type TEXT NOT NULL,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_profile_url ON profiles(profile_url);
	CREATE INDEX IF NOT EXISTS idx_connection_requests_profile ON connection_requests(profile_url);
	CREATE INDEX IF NOT EXISTS idx_connection_requests_sent_at ON connection_requests(sent_at);
	CREATE INDEX IF NOT EXISTS idx_messages_profile ON messages(profile_url);
	CREATE INDEX IF NOT EXISTS idx_actions_type ON actions(action_type);
	CREATE INDEX IF NOT EXISTS idx_actions_timestamp ON actions(timestamp);
	`

	_, err := db.Exec(schema)
	return err
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// SaveProfile saves a profile to the database
func SaveProfile(profile Profile) error {
	query := `
		INSERT INTO profiles (profile_url, name, headline, discovered_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(profile_url) DO UPDATE SET
			name = excluded.name,
			headline = excluded.headline
	`

	_, err := db.Exec(query, profile.ProfileURL, profile.Name, profile.Headline, profile.DiscoveredAt)
	if err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	return nil
}

// ProfileExists checks if a profile URL already exists in the database
func ProfileExists(profileURL string) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM profiles WHERE profile_url = ?"
	err := db.QueryRow(query, profileURL).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check profile existence: %w", err)
	}

	return count > 0, nil
}

// RecordConnectionRequest records a sent connection request
func RecordConnectionRequest(req ConnectionRequest) error {
	query := `
		INSERT INTO connection_requests (profile_url, note, sent_at)
		VALUES (?, ?, ?)
	`

	_, err := db.Exec(query, req.ProfileURL, req.Note, req.SentAt)
	if err != nil {
		return fmt.Errorf("failed to record connection request: %w", err)
	}

	return nil
}

// GetConnectionRequestsSentToday returns the number of connection requests sent today
func GetConnectionRequestsSentToday() (int, error) {
	query := `
		SELECT COUNT(*) FROM connection_requests
		WHERE DATE(sent_at) = DATE('now')
	`

	var count int
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get today's request count: %w", err)
	}

	return count, nil
}

// GetPendingRequests returns connection requests that haven't been accepted yet
func GetPendingRequests() ([]ConnectionRequest, error) {
	query := `
		SELECT id, profile_url, note, sent_at, accepted, accepted_at
		FROM connection_requests
		WHERE accepted = FALSE
		ORDER BY sent_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending requests: %w", err)
	}
	defer rows.Close()

	var requests []ConnectionRequest
	for rows.Next() {
		var req ConnectionRequest
		var acceptedAt sql.NullTime

		err := rows.Scan(&req.ID, &req.ProfileURL, &req.Note, &req.SentAt, &req.Accepted, &acceptedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan request: %w", err)
		}

		if acceptedAt.Valid {
			req.AcceptedAt = &acceptedAt.Time
		}

		requests = append(requests, req)
	}

	return requests, nil
}

// MarkRequestAccepted marks a connection request as accepted
func MarkRequestAccepted(profileURL string) error {
	query := `
		UPDATE connection_requests
		SET accepted = TRUE, accepted_at = CURRENT_TIMESTAMP
		WHERE profile_url = ? AND accepted = FALSE
	`

	result, err := db.Exec(query, profileURL)
	if err != nil {
		return fmt.Errorf("failed to mark request accepted: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no pending request found for profile URL: %s", profileURL)
	}

	return nil
}

// RecordMessage records a sent message
func RecordMessage(msg Message) error {
	query := `
		INSERT INTO messages (profile_url, message, sent_at)
		VALUES (?, ?, ?)
	`

	_, err := db.Exec(query, msg.ProfileURL, msg.Message, msg.SentAt)
	if err != nil {
		return fmt.Errorf("failed to record message: %w", err)
	}

	return nil
}

// HasSentMessage checks if a message has already been sent to a profile
func HasSentMessage(profileURL string) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM messages WHERE profile_url = ?"
	err := db.QueryRow(query, profileURL).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check message history: %w", err)
	}

	return count > 0, nil
}

// RecordAction records an action (for rate limiting)
func RecordAction(actionType string) error {
	query := `
		INSERT INTO actions (action_type, timestamp)
		VALUES (?, CURRENT_TIMESTAMP)
	`

	_, err := db.Exec(query, actionType)
	if err != nil {
		return fmt.Errorf("failed to record action: %w", err)
	}

	return nil
}

// GetActionsInLastHour returns the number of actions of a specific type in the last hour
func GetActionsInLastHour(actionType string) (int, error) {
	query := `
		SELECT COUNT(*) FROM actions
		WHERE action_type = ? AND timestamp >= datetime('now', '-1 hour')
	`

	var count int
	err := db.QueryRow(query, actionType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get action count: %w", err)
	}

	return count, nil
}

// GetActionsToday returns the number of actions of a specific type today
func GetActionsToday(actionType string) (int, error) {
	query := `
		SELECT COUNT(*) FROM actions
		WHERE action_type = ? AND DATE(timestamp) = DATE('now')
	`

	var count int
	err := db.QueryRow(query, actionType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get action count: %w", err)
	}

	return count, nil
}

// CleanupOldActions removes actions older than 30 days
func CleanupOldActions() error {
	query := `
		DELETE FROM actions
		WHERE timestamp < datetime('now', '-30 days')
	`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to cleanup old actions: %w", err)
	}

	return nil
}

// GetStats returns statistics about the database
func GetStats() (map[string]int, error) {
	stats := make(map[string]int)

	// Total profiles
	var totalProfiles int
	err := db.QueryRow("SELECT COUNT(*) FROM profiles").Scan(&totalProfiles)
	if err != nil {
		return nil, err
	}
	stats["total_profiles"] = totalProfiles

	// Total connection requests
	var totalRequests int
	err = db.QueryRow("SELECT COUNT(*) FROM connection_requests").Scan(&totalRequests)
	if err != nil {
		return nil, err
	}
	stats["total_requests"] = totalRequests

	// Accepted connections
	var acceptedConnections int
	err = db.QueryRow("SELECT COUNT(*) FROM connection_requests WHERE accepted = TRUE").Scan(&acceptedConnections)
	if err != nil {
		return nil, err
	}
	stats["accepted_connections"] = acceptedConnections

	// Total messages
	var totalMessages int
	err = db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&totalMessages)
	if err != nil {
		return nil, err
	}
	stats["total_messages"] = totalMessages

	// Requests sent today
	var requestsToday int
	err = db.QueryRow("SELECT COUNT(*) FROM connection_requests WHERE DATE(sent_at) = DATE('now')").Scan(&requestsToday)
	if err != nil {
		return nil, err
	}
	stats["requests_today"] = requestsToday

	return stats, nil
}
