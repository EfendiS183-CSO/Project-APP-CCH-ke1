package database

import (
	"testing"

	"cch.local/backend/internal/platform/config"
)

func TestOpenMySQLRejectsIncompleteSettings(t *testing.T) {
	connection, err := OpenMySQL(config.MySQLSettings{Host: "127.0.0.1"})
	if err == nil {
		t.Fatal("expected incomplete settings to be rejected")
	}
	if connection != nil {
		t.Fatal("connection should be nil when settings are incomplete")
	}
}

func TestOpenMySQLBuildsConnectionWithoutConnecting(t *testing.T) {
	connection, err := OpenMySQL(config.MySQLSettings{
		Host:     "127.0.0.1",
		Port:     "3306",
		User:     "cch_app_test",
		Password: "test-only-p@ss:/?#",
		Database: "cch_test",
	})
	if err != nil {
		t.Fatalf("open connection configuration: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close connection pool: %v", err)
	}
}
