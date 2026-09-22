package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("Host = %v, want localhost", cfg.Host)
	}

	if cfg.Port != 5432 {
		t.Errorf("Port = %v, want 5432", cfg.Port)
	}

	if cfg.User != "labctl" {
		t.Errorf("User = %v, want labctl", cfg.User)
	}

	if cfg.Database != "labctl" {
		t.Errorf("Database = %v, want labctl", cfg.Database)
	}

	// Must be a mode lib/pq implements. "prefer"/"allow" are libpq-only and
	// make the driver reject the connection outright, which is how a default
	// that could never connect survived a full test suite.
	if cfg.SSLMode != "disable" {
		t.Errorf("SSLMode = %v, want disable", cfg.SSLMode)
	}

	if cfg.MaxOpenConns != 25 {
		t.Errorf("MaxOpenConns = %v, want 25", cfg.MaxOpenConns)
	}

	if cfg.MaxIdleConns != 5 {
		t.Errorf("MaxIdleConns = %v, want 5", cfg.MaxIdleConns)
	}

	if cfg.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 30m", cfg.ConnMaxLifetime)
	}

	if cfg.ConnMaxIdleTime != 5*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 5m", cfg.ConnMaxIdleTime)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid custom config",
			config: Config{
				Host:            "db.example.com",
				Port:            5433,
				User:            "admin",
				Password:        "secret",
				Database:        "myapp",
				SSLMode:         "require",
				MaxOpenConns:    50,
				MaxIdleConns:    10,
				ConnMaxLifetime: time.Hour,
				ConnMaxIdleTime: 10 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "empty host",
			config: Config{
				Host:         "",
				Port:         5432,
				User:         "user",
				Database:     "db",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: true,
			errMsg:  "host cannot be empty",
		},
		{
			name: "port too low",
			config: Config{
				Host:         "localhost",
				Port:         0,
				User:         "user",
				Database:     "db",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name: "port too high",
			config: Config{
				Host:         "localhost",
				Port:         65536,
				User:         "user",
				Database:     "db",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name: "empty user",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "",
				Database:     "db",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: true,
			errMsg:  "user cannot be empty",
		},
		{
			name: "empty database",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: true,
			errMsg:  "database name cannot be empty",
		},
		{
			name: "invalid ssl mode",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				SSLMode:      "invalid",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: true,
			errMsg:  "invalid ssl_mode",
		},
		{
			name: "ssl mode disable",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				SSLMode:      "disable",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: false,
		},
		{
			name: "ssl mode require",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				SSLMode:      "require",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: false,
		},
		{
			name: "ssl mode verify-ca",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				SSLMode:      "verify-ca",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: false,
		},
		{
			name: "ssl mode verify-full",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				SSLMode:      "verify-full",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: false,
		},
		{
			name: "max open conns zero",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				MaxOpenConns: 0,
				MaxIdleConns: 0,
			},
			wantErr: true,
			errMsg:  "max_open_conns must be at least 1",
		},
		{
			name: "max idle conns negative",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				MaxOpenConns: 10,
				MaxIdleConns: -1,
			},
			wantErr: true,
			errMsg:  "max_idle_conns cannot be negative",
		},
		{
			name: "max idle exceeds max open",
			config: Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				MaxOpenConns: 5,
				MaxIdleConns: 10,
			},
			wantErr: true,
			errMsg:  "max_idle_conns cannot exceed max_open_conns",
		},
		{
			name: "multiple errors",
			config: Config{
				Host:         "",
				Port:         0,
				User:         "",
				Database:     "",
				MaxOpenConns: 0,
				MaxIdleConns: -1,
			},
			wantErr: true,
			errMsg:  "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestDefaultConfigSSLModeIsDriverSupported pins the default to what the
// registered driver can actually negotiate. The suite previously asserted the
// default was "prefer" — a value github.com/lib/pq rejects with `unsupported
// sslmode` — so every test passed while the shipped default could not open a
// connection. Validate() alone would not have caught it either, because the
// allowlist admitted "prefer" too; this checks the default against the
// driver's real capability set, independently of Validate().
func TestDefaultConfigSSLModeIsDriverSupported(t *testing.T) {
	// Exactly the set github.com/lib/pq implements (see conn.go ssl()).
	driverSupported := map[string]bool{
		"disable": true, "require": true, "verify-ca": true, "verify-full": true,
	}

	cfg := DefaultConfig()
	if !driverSupported[cfg.SSLMode] {
		t.Fatalf("DefaultConfig().SSLMode = %q, which lib/pq does not support; "+
			"the default must be one of disable/require/verify-ca/verify-full", cfg.SSLMode)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("DefaultConfig() must pass its own validation, got: %v", err)
	}
}

// TestValidateRejectsLibpqOnlySSLModes guards the other half of the same bug:
// validation must not admit modes the driver will refuse at connect time.
func TestValidateRejectsLibpqOnlySSLModes(t *testing.T) {
	for _, mode := range []string{"allow", "prefer"} {
		t.Run(mode, func(t *testing.T) {
			cfg := Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				SSLMode:      mode,
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			}

			if err := cfg.Validate(); err == nil {
				t.Errorf("Validate() accepted SSLMode %q, but lib/pq rejects it at connect time", mode)
			}
		})
	}
}

func TestValidSSLModes(t *testing.T) {
	validModes := []string{"disable", "require", "verify-ca", "verify-full"}

	for _, mode := range validModes {
		t.Run(mode, func(t *testing.T) {
			cfg := Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				SSLMode:      mode,
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			}

			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() with SSLMode %q should not error, got %v", mode, err)
			}
		})
	}
}

func TestConfigValidateEdgeCases(t *testing.T) {
	// Test with empty SSL mode (should be valid - uses default)
	cfg := Config{
		Host:         "localhost",
		Port:         5432,
		User:         "user",
		Database:     "db",
		SSLMode:      "",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with empty SSLMode should not error, got %v", err)
	}

	// Test boundary port values
	cfg.Port = 1
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with port 1 should not error, got %v", err)
	}

	cfg.Port = 65535
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with port 65535 should not error, got %v", err)
	}

	// Test max idle equals max open (valid)
	cfg.Port = 5432
	cfg.MaxOpenConns = 10
	cfg.MaxIdleConns = 10
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with MaxIdleConns == MaxOpenConns should not error, got %v", err)
	}
}

// containsSubstring checks if s contains substr (case-insensitive comparison not needed for this)
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMigrationRecord(t *testing.T) {
	// Test MigrationRecord struct
	now := time.Now()
	record := MigrationRecord{
		Version:   "001_initial.sql",
		AppliedAt: now,
	}

	if record.Version != "001_initial.sql" {
		t.Errorf("Version = %v, want 001_initial.sql", record.Version)
	}

	if !record.AppliedAt.Equal(now) {
		t.Errorf("AppliedAt = %v, want %v", record.AppliedAt, now)
	}
}

func TestMigrationStatusEntry(t *testing.T) {
	// Test unapplied migration
	entry := MigrationStatusEntry{
		Version:   "002_add_users.sql",
		Applied:   false,
		AppliedAt: nil,
	}

	if entry.Version != "002_add_users.sql" {
		t.Errorf("Version = %v, want 002_add_users.sql", entry.Version)
	}

	if entry.Applied {
		t.Error("Applied should be false for unapplied migration")
	}

	if entry.AppliedAt != nil {
		t.Error("AppliedAt should be nil for unapplied migration")
	}

	// Test applied migration
	now := time.Now()
	appliedEntry := MigrationStatusEntry{
		Version:   "001_initial.sql",
		Applied:   true,
		AppliedAt: &now,
	}

	if appliedEntry.Version != "001_initial.sql" {
		t.Errorf("Version = %v, want 001_initial.sql", appliedEntry.Version)
	}

	if !appliedEntry.Applied {
		t.Error("Applied should be true for applied migration")
	}

	if appliedEntry.AppliedAt == nil {
		t.Error("AppliedAt should not be nil for applied migration")
	}

	if !appliedEntry.AppliedAt.Equal(now) {
		t.Errorf("AppliedAt = %v, want %v", *appliedEntry.AppliedAt, now)
	}
}

func TestConfigDSN(t *testing.T) {
	// Test that a valid config can be created with all fields
	cfg := Config{
		Host:            "db.example.com",
		Port:            5433,
		User:            "testuser",
		Password:        "testpass",
		Database:        "testdb",
		SSLMode:         "require",
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
	}

	// Verify all fields are set correctly
	if cfg.Host != "db.example.com" {
		t.Errorf("Host = %v, want db.example.com", cfg.Host)
	}
	if cfg.Port != 5433 {
		t.Errorf("Port = %v, want 5433", cfg.Port)
	}
	if cfg.User != "testuser" {
		t.Errorf("User = %v, want testuser", cfg.User)
	}
	if cfg.Password != "testpass" {
		t.Errorf("Password = %v, want testpass", cfg.Password)
	}
	if cfg.Database != "testdb" {
		t.Errorf("Database = %v, want testdb", cfg.Database)
	}
	if cfg.SSLMode != "require" {
		t.Errorf("SSLMode = %v, want require", cfg.SSLMode)
	}
	if cfg.MaxOpenConns != 50 {
		t.Errorf("MaxOpenConns = %v, want 50", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %v, want 10", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != time.Hour {
		t.Errorf("ConnMaxLifetime = %v, want 1h", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != 10*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 10m", cfg.ConnMaxIdleTime)
	}

	// Should validate successfully
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestConfigValidateAllSSLModes(t *testing.T) {
	tests := []struct {
		mode    string
		wantErr bool
	}{
		{"disable", false},
		{"require", false},
		{"verify-ca", false},
		{"verify-full", false},
		{"", false}, // empty is allowed
		{"invalid", true},
		// libpq has these; lib/pq does not implement them. Validation must
		// reject them rather than admitting a value the driver will refuse.
		{"allow", true},
		{"prefer", true},
		{"DISABLE", true}, // case sensitive
		{"Require", true}, // case sensitive
	}

	for _, tt := range tests {
		t.Run("sslmode_"+tt.mode, func(t *testing.T) {
			cfg := Config{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Database:     "db",
				SSLMode:      tt.mode,
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			}

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() with SSLMode %q: error = %v, wantErr = %v", tt.mode, err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidateConnectionPoolSettings(t *testing.T) {
	tests := []struct {
		name     string
		maxOpen  int
		maxIdle  int
		lifetime time.Duration
		idleTime time.Duration
		wantErr  bool
	}{
		{
			name:     "valid minimal settings",
			maxOpen:  1,
			maxIdle:  0,
			lifetime: 0,
			idleTime: 0,
			wantErr:  false,
		},
		{
			name:     "valid large pool",
			maxOpen:  100,
			maxIdle:  50,
			lifetime: 24 * time.Hour,
			idleTime: time.Hour,
			wantErr:  false,
		},
		{
			name:     "max idle equals max open",
			maxOpen:  25,
			maxIdle:  25,
			lifetime: 30 * time.Minute,
			idleTime: 5 * time.Minute,
			wantErr:  false,
		},
		{
			name:    "max open zero",
			maxOpen: 0,
			maxIdle: 0,
			wantErr: true,
		},
		{
			name:    "max idle negative",
			maxOpen: 10,
			maxIdle: -1,
			wantErr: true,
		},
		{
			name:    "max idle exceeds max open",
			maxOpen: 10,
			maxIdle: 20,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Host:            "localhost",
				Port:            5432,
				User:            "user",
				Database:        "db",
				MaxOpenConns:    tt.maxOpen,
				MaxIdleConns:    tt.maxIdle,
				ConnMaxLifetime: tt.lifetime,
				ConnMaxIdleTime: tt.idleTime,
			}

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidateMultipleErrors(t *testing.T) {
	// Config with multiple validation errors
	cfg := Config{
		Host:         "",
		Port:         0,
		User:         "",
		Database:     "",
		SSLMode:      "invalid",
		MaxOpenConns: 0,
		MaxIdleConns: -1,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for config with multiple issues")
	}

	errStr := err.Error()
	// Check that multiple errors are present
	expectedSubstrings := []string{
		"host",
		"port",
		"user",
		"database",
		"ssl_mode",
		"max_open_conns",
		"max_idle_conns",
	}

	for _, substr := range expectedSubstrings {
		if !containsSubstring(errStr, substr) {
			t.Errorf("Validate() error should mention %q, got: %v", substr, errStr)
		}
	}
}

func TestDefaultConfigImmutability(t *testing.T) {
	// Get two default configs
	cfg1 := DefaultConfig()
	cfg2 := DefaultConfig()

	// Modify cfg1 and verify the modification took effect
	cfg1.Host = "modified"
	cfg1.Port = 9999

	if cfg1.Host != "modified" {
		t.Errorf("cfg1.Host = %v, want modified", cfg1.Host)
	}
	if cfg1.Port != 9999 {
		t.Errorf("cfg1.Port = %v, want 9999", cfg1.Port)
	}

	// cfg2 should still have defaults
	if cfg2.Host != "localhost" {
		t.Errorf("DefaultConfig() returned mutable struct, cfg2.Host = %v", cfg2.Host)
	}

	if cfg2.Port != 5432 {
		t.Errorf("DefaultConfig() returned mutable struct, cfg2.Port = %v", cfg2.Port)
	}
}

// Helper to create a test logger
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestDBHealth(t *testing.T) {
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name    string
		mockFn  func()
		wantErr bool
	}{
		{
			name: "healthy database",
			mockFn: func() {
				mock.ExpectPing()
			},
			wantErr: false,
		},
		{
			name: "unhealthy database",
			mockFn: func() {
				mock.ExpectPing().WillReturnError(errors.New("connection refused"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockFn()
			err := db.Health(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Health() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDBClose(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	mock.ExpectClose()

	if err := db.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDBTransaction_Success(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO test").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = db.Transaction(context.Background(), func(tx *sql.Tx) error {
		_, execErr := tx.Exec("INSERT INTO test VALUES (1)")
		return execErr
	})

	if err != nil {
		t.Errorf("Transaction() unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDBTransaction_FnError_Rollback(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	expectedErr := errors.New("function error")

	mock.ExpectBegin()
	mock.ExpectRollback()

	err = db.Transaction(context.Background(), func(tx *sql.Tx) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Transaction() error = %v, want %v", err, expectedErr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDBTransaction_BeginError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

	err = db.Transaction(context.Background(), func(tx *sql.Tx) error {
		return nil
	})

	if err == nil {
		t.Error("Transaction() expected error for begin failure")
	}

	if !containsSubstring(err.Error(), "beginning transaction") {
		t.Errorf("Transaction() error should mention 'beginning transaction', got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDBTransaction_CommitError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

	err = db.Transaction(context.Background(), func(tx *sql.Tx) error {
		return nil
	})

	if err == nil {
		t.Error("Transaction() expected error for commit failure")
	}

	if !containsSubstring(err.Error(), "committing transaction") {
		t.Errorf("Transaction() error should mention 'committing transaction', got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDBTransaction_RollbackError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	fnErr := errors.New("function error")

	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(errors.New("rollback failed"))

	err = db.Transaction(context.Background(), func(tx *sql.Tx) error {
		return fnErr
	})

	if err == nil {
		t.Error("Transaction() expected error")
	}

	// Error should contain both rollback and original error info
	if !containsSubstring(err.Error(), "rolling back") {
		t.Errorf("Transaction() error should mention 'rolling back', got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetMigrationFiles(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	// Test that getMigrationFiles returns sorted list
	files, err := db.getMigrationFiles()
	if err != nil {
		t.Fatalf("getMigrationFiles() unexpected error: %v", err)
	}

	// Should return at least some migrations
	if len(files) == 0 {
		t.Error("getMigrationFiles() returned empty list, expected at least one migration")
	}

	// Check they're sorted
	for i := 1; i < len(files); i++ {
		if files[i] < files[i-1] {
			t.Errorf("getMigrationFiles() not sorted: %s comes after %s", files[i], files[i-1])
		}
	}

	// Check they all end in .sql
	for _, f := range files {
		if !containsSubstring(f, ".sql") {
			t.Errorf("getMigrationFiles() returned non-SQL file: %s", f)
		}
	}
}

func TestCreateMigrationsTable(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name    string
		mockFn  func()
		wantErr bool
	}{
		{
			name: "success",
			mockFn: func() {
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: false,
		},
		{
			name: "database error",
			mockFn: func() {
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
					WillReturnError(errors.New("permission denied"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockFn()
			err := db.createMigrationsTable(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("createMigrationsTable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetAppliedMigrations(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	now := time.Now()

	tests := []struct {
		name    string
		mockFn  func()
		want    int
		wantErr bool
	}{
		{
			name: "no migrations applied",
			mockFn: func() {
				rows := sqlmock.NewRows([]string{"version", "applied_at"})
				mock.ExpectQuery("SELECT version, applied_at FROM schema_migrations").
					WillReturnRows(rows)
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "multiple migrations",
			mockFn: func() {
				rows := sqlmock.NewRows([]string{"version", "applied_at"}).
					AddRow("001_initial.sql", now).
					AddRow("002_users.sql", now)
				mock.ExpectQuery("SELECT version, applied_at FROM schema_migrations").
					WillReturnRows(rows)
			},
			want:    2,
			wantErr: false,
		},
		{
			name: "database error",
			mockFn: func() {
				mock.ExpectQuery("SELECT version, applied_at FROM schema_migrations").
					WillReturnError(errors.New("table not found"))
			},
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockFn()
			got, err := db.getAppliedMigrations(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("getAppliedMigrations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.want {
				t.Errorf("getAppliedMigrations() returned %d migrations, want %d", len(got), tt.want)
			}
		})
	}
}

func TestMigrationStatus(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	now := time.Now()

	// Get the actual migration files to set up expectations correctly
	files, err := db.getMigrationFiles()
	if err != nil {
		t.Fatalf("getMigrationFiles() failed: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no migration files found")
	}

	tests := []struct {
		name    string
		mockFn  func()
		wantErr bool
	}{
		{
			name: "success with some applied",
			mockFn: func() {
				// Only first migration applied
				rows := sqlmock.NewRows([]string{"version", "applied_at"}).
					AddRow(files[0], now)
				mock.ExpectQuery("SELECT version, applied_at FROM schema_migrations").
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			mockFn: func() {
				mock.ExpectQuery("SELECT version, applied_at FROM schema_migrations").
					WillReturnError(errors.New("connection lost"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockFn()
			status, err := db.MigrationStatus(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("MigrationStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Should have an entry for each migration file
				if len(status) != len(files) {
					t.Errorf("MigrationStatus() returned %d entries, want %d", len(status), len(files))
				}
			}
		})
	}
}

func TestPrintMigrationStatus(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	now := time.Now()

	// Get migration files
	files, err := db.getMigrationFiles()
	if err != nil {
		t.Fatalf("getMigrationFiles() failed: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no migration files found")
	}

	// Mock successful query
	rows := sqlmock.NewRows([]string{"version", "applied_at"}).
		AddRow(files[0], now)
	mock.ExpectQuery("SELECT version, applied_at FROM schema_migrations").
		WillReturnRows(rows)

	logger := testLogger()
	err = db.PrintMigrationStatus(context.Background(), logger)
	if err != nil {
		t.Errorf("PrintMigrationStatus() unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPrintMigrationStatus_Error(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := &DB{
		DB:     mockDB,
		logger: testLogger(),
		config: DefaultConfig(),
	}

	mock.ExpectQuery("SELECT version, applied_at FROM schema_migrations").
		WillReturnError(errors.New("database error"))

	logger := testLogger()
	err = db.PrintMigrationStatus(context.Background(), logger)
	if err == nil {
		t.Error("PrintMigrationStatus() expected error")
	}
}
