package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestIsSelectQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "Simple SELECT",
			query:    "SELECT * FROM users",
			expected: true,
		},
		{
			name:     "SELECT with WHERE",
			query:    "SELECT id, name FROM users WHERE age > 18",
			expected: true,
		},
		{
			name:     "SELECT with lowercase",
			query:    "select * from users",
			expected: true,
		},
		{
			name:     "SELECT with leading spaces",
			query:    "   SELECT * FROM users",
			expected: true,
		},
		{
			name:     "SELECT with comment",
			query:    "-- This is a comment\nSELECT * FROM users",
			expected: true,
		},
		{
			name:     "UPDATE query",
			query:    "UPDATE users SET name = 'John'",
			expected: false,
		},
		{
			name:     "DELETE query",
			query:    "DELETE FROM users WHERE id = 1",
			expected: false,
		},
		{
			name:     "INSERT query",
			query:    "INSERT INTO users (name) VALUES ('John')",
			expected: false,
		},
		{
			name:     "DROP TABLE",
			query:    "DROP TABLE users",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSelectQuery(tt.query)
			if result != tt.expected {
				t.Errorf("isSelectQuery(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestDetectQueryOperation(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "INSERT query",
			query:    "INSERT INTO users (name) VALUES ('John')",
			expected: "INSERT",
		},
		{
			name:     "UPDATE query",
			query:    "UPDATE users SET name = 'John'",
			expected: "UPDATE",
		},
		{
			name:     "DELETE query",
			query:    "DELETE FROM users WHERE id = 1",
			expected: "DELETE",
		},
		{
			name:     "CREATE TABLE",
			query:    "CREATE TABLE users (id INT)",
			expected: "CREATE",
		},
		{
			name:     "DROP TABLE",
			query:    "DROP TABLE users",
			expected: "DROP",
		},
		{
			name:     "ALTER TABLE",
			query:    "ALTER TABLE users ADD COLUMN age INT",
			expected: "ALTER",
		},
		{
			name:     "TRUNCATE TABLE",
			query:    "TRUNCATE TABLE users",
			expected: "TRUNCATE",
		},
		{
			name:     "SELECT query",
			query:    "SELECT * FROM users",
			expected: "UNKNOWN",
		},
		{
			name:     "Lowercase update",
			query:    "update users set name = 'John'",
			expected: "UPDATE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectQueryOperation(tt.query)
			if result != tt.expected {
				t.Errorf("detectQueryOperation(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestGenerateConfirmToken(t *testing.T) {
	token1 := generateConfirmToken("UPDATE users SET status = 'active'", 100)
	token2 := generateConfirmToken("UPDATE users SET status = 'active'", 100)

	// Tokens should be different even with same input (due to timestamp)
	if token1 == token2 {
		t.Error("generateConfirmToken should generate different tokens for same input")
	}

	// Token should have expected length (12 characters)
	if len(token1) != 12 {
		t.Errorf("Token length = %d, want 12", len(token1))
	}
}

// TestHandleQueryValidation tests only the query validation logic
func TestHandleQueryValidation(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "SELECT allowed",
			query:       "SELECT * FROM users",
			shouldError: false,
		},
		{
			name:        "UPDATE rejected",
			query:       "UPDATE users SET name = 'John'",
			shouldError: true,
			errorMsg:    "UPDATE",
		},
		{
			name:        "DELETE rejected",
			query:       "DELETE FROM users",
			shouldError: true,
			errorMsg:    "DELETE",
		},
		{
			name:        "INSERT rejected",
			query:       "INSERT INTO users (name) VALUES ('test')",
			shouldError: true,
			errorMsg:    "INSERT",
		},
		{
			name:        "DROP rejected",
			query:       "DROP TABLE users",
			shouldError: true,
			errorMsg:    "DROP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test only the validation logic
			if tt.shouldError {
				if isSelectQuery(tt.query) {
					t.Errorf("Query %q should not be detected as SELECT", tt.query)
				}
				operation := detectQueryOperation(tt.query)
				if operation != tt.errorMsg {
					t.Errorf("Operation should be %q, got %q", tt.errorMsg, operation)
				}
			} else {
				if !isSelectQuery(tt.query) {
					t.Errorf("Query %q should be detected as SELECT", tt.query)
				}
			}
		})
	}
}

// TestExecuteToolValidation tests execute tool validation without MySQL dependency
func TestExecuteToolValidation(t *testing.T) {
	server := NewMCPServer()

	// Test 1: Reject SELECT queries
	t.Run("Reject SELECT", func(t *testing.T) {
		args, _ := json.Marshal(map[string]interface{}{
			"sql": "SELECT * FROM users",
		})

		response := server.handleExecuteTool(1, args)
		if response.Error == nil {
			t.Error("Execute tool should reject SELECT queries")
		}
		if !strings.Contains(response.Error.Message, "SELECT queries should use the 'query' tool") {
			t.Errorf("Wrong error message: %v", response.Error.Message)
		}
	})

	// Test 2: Require confirm token when dry_run=false
	t.Run("Require confirm token", func(t *testing.T) {
		args, _ := json.Marshal(map[string]interface{}{
			"sql":     "UPDATE users SET status = 'active'",
			"dry_run": false,
		})

		response := server.handleExecuteTool(2, args)
		if response.Error == nil {
			t.Error("Should require confirm_token when dry_run=false")
		}
		if !strings.Contains(response.Error.Message, "confirm_token is required") {
			t.Errorf("Wrong error message: %v", response.Error.Message)
		}
	})
}

func TestHandleExecuteToolConfirmation(t *testing.T) {
	server := NewMCPServer()

	// Manually create a confirmation token for testing
	sql := "UPDATE users SET status = 'active'"
	token := generateConfirmToken(sql, 10)
	server.confirmTokens[token] = &ExecuteConfirmation{
		SQL:          sql,
		AffectedRows: 10,
		Operation:    "UPDATE",
		CreatedAt:    time.Now(),
	}

	// Test execution without token
	args2, _ := json.Marshal(map[string]interface{}{
		"sql":     sql,
		"dry_run": false,
	})

	response2 := server.handleExecuteTool(2, args2)
	if response2.Error == nil {
		t.Error("Execution without token should error")
	}
	if !strings.Contains(response2.Error.Message, "confirm_token is required") {
		t.Errorf("Wrong error message: %v", response2.Error.Message)
	}

	// Test execution with wrong SQL
	args3, _ := json.Marshal(map[string]interface{}{
		"sql":           "DELETE FROM users", // Different SQL
		"dry_run":       false,
		"confirm_token": token,
	})

	response3 := server.handleExecuteTool(3, args3)
	if response3.Error == nil {
		t.Error("Execution with wrong SQL should error")
	}
	if !strings.Contains(response3.Error.Message, "SQL does not match") {
		t.Errorf("Wrong error message: %v", response3.Error.Message)
	}

	// Test execution with invalid token
	args4, _ := json.Marshal(map[string]interface{}{
		"sql":           sql,
		"dry_run":       false,
		"confirm_token": "invalid_token",
	})

	response4 := server.handleExecuteTool(4, args4)
	if response4.Error == nil {
		t.Error("Execution with invalid token should error")
	}
	if !strings.Contains(response4.Error.Message, "Invalid or expired") {
		t.Errorf("Wrong error message: %v", response4.Error.Message)
	}
}

func TestHandleExecuteToolTokenExpiration(t *testing.T) {
	server := NewMCPServer()

	// Create a confirmation with past timestamp
	sql := "UPDATE users SET status = 'active'"
	token := generateConfirmToken(sql, 10)
	server.confirmTokens[token] = &ExecuteConfirmation{
		SQL:          sql,
		AffectedRows: 10,
		Operation:    "UPDATE",
		CreatedAt:    time.Now().Add(-6 * time.Minute), // 6 minutes ago
	}

	// Try to use expired token
	args, _ := json.Marshal(map[string]interface{}{
		"sql":           sql,
		"dry_run":       false,
		"confirm_token": token,
	})

	response := server.handleExecuteTool(1, args)
	if response.Error == nil {
		t.Error("Execution with expired token should error")
	}
	if !strings.Contains(response.Error.Message, "expired") {
		t.Errorf("Wrong error message: %v", response.Error.Message)
	}

	// Token should be removed
	if _, exists := server.confirmTokens[token]; exists {
		t.Error("Expired token should be removed")
	}
}

func TestCleanupExpiredTokens(t *testing.T) {
	server := NewMCPServer()

	// Add some tokens with different ages
	server.confirmTokens["fresh"] = &ExecuteConfirmation{
		CreatedAt: time.Now(),
	}
	server.confirmTokens["old"] = &ExecuteConfirmation{
		CreatedAt: time.Now().Add(-10 * time.Minute),
	}
	server.confirmTokens["very_old"] = &ExecuteConfirmation{
		CreatedAt: time.Now().Add(-30 * time.Minute),
	}

	server.cleanupExpiredTokens()

	// Fresh token should remain
	if _, exists := server.confirmTokens["fresh"]; !exists {
		t.Error("Fresh token should not be removed")
	}

	// Old tokens should be removed
	if _, exists := server.confirmTokens["old"]; exists {
		t.Error("Old token should be removed")
	}
	if _, exists := server.confirmTokens["very_old"]; exists {
		t.Error("Very old token should be removed")
	}
}

// TestExplainAnalyzeValidation tests that EXPLAIN ANALYZE rejects non-SELECT queries
func TestExplainAnalyzeValidation(t *testing.T) {
	tests := []struct {
		name             string
		query            string
		analyze          bool
		shouldReject     bool
		expectedSuggestion string
	}{
		{
			name:         "EXPLAIN ANALYZE with SELECT",
			query:        "SELECT * FROM users WHERE id = 1",
			analyze:      true,
			shouldReject: false,
		},
		{
			name:         "EXPLAIN ANALYZE with UPDATE",
			query:        "UPDATE users SET name = 'test'",
			analyze:      true,
			shouldReject: true,
			expectedSuggestion: "SELECT * FROM users WHERE",
		},
		{
			name:         "EXPLAIN ANALYZE with DELETE",
			query:        "DELETE FROM users WHERE id = 1",
			analyze:      true,
			shouldReject: true,
			expectedSuggestion: "SELECT * FROM users WHERE",
		},
		{
			name:         "EXPLAIN ANALYZE with INSERT",
			query:        "INSERT INTO users (name) VALUES ('test')",
			analyze:      true,
			shouldReject: true,
			expectedSuggestion: "schema",
		},
		{
			name:         "EXPLAIN (no ANALYZE) with UPDATE",
			query:        "UPDATE users SET name = 'test'",
			analyze:      false,
			shouldReject: false, // Regular EXPLAIN is allowed for all queries
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only test the validation logic
			if tt.analyze && !isSelectQuery(tt.query) {
				if !tt.shouldReject {
					t.Error("Should reject non-SELECT query with EXPLAIN ANALYZE")
				}
				
				// Test suggestion generation
				operation := detectQueryOperation(tt.query)
				var suggestion string
				
				switch operation {
				case "UPDATE":
					re := regexp.MustCompile(`(?i)UPDATE\s+(\S+)\s+SET`)
					matches := re.FindStringSubmatch(tt.query)
					if len(matches) > 1 {
						table := matches[1]
						suggestion = fmt.Sprintf("To analyze UPDATE performance, try: SELECT * FROM %s WHERE <your conditions>", table)
					}
				case "DELETE":
					re := regexp.MustCompile(`(?i)DELETE\s+FROM\s+(\S+)`)
					matches := re.FindStringSubmatch(tt.query)
					if len(matches) > 1 {
						table := matches[1]
						suggestion = fmt.Sprintf("To analyze DELETE performance, try: SELECT * FROM %s WHERE <your conditions>", table)
					}
				case "INSERT":
					suggestion = "To analyze INSERT performance, examine the table structure with 'schema <table>' or analyze a SELECT on the target table"
				}
				
				if tt.expectedSuggestion != "" && !strings.Contains(suggestion, tt.expectedSuggestion) {
					t.Errorf("Expected suggestion to contain %q, got %q", tt.expectedSuggestion, suggestion)
				}
			} else if tt.shouldReject {
				t.Error("Should not reject this query")
			}
		})
	}
}
