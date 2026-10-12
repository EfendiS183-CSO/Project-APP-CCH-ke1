package database

import (
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/platform/config"
)

const (
	maxOpenConnections = 10
	maxIdleConnections = 5
	connectionLifetime = 5 * time.Minute
)

func OpenMySQL(settings config.MySQLSettings) (*sql.DB, error) {
	if !settings.Configured() {
		return nil, fmt.Errorf("MySQL settings are incomplete")
	}

	driverConfig := mysql.Config{
		User:      settings.User,
		Passwd:    settings.Password,
		Net:       "tcp",
		Addr:      net.JoinHostPort(settings.Host, settings.Port),
		DBName:    settings.Database,
		ParseTime: true,
		Loc:       time.UTC,
		Timeout:   3 * time.Second,
		Params: map[string]string{
			"charset":              "utf8mb4",
			"allowNativePasswords": "true",
		},
	}

	connection, err := sql.Open("mysql", driverConfig.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("configure MySQL connection: %w", err)
	}

	connection.SetMaxOpenConns(maxOpenConnections)
	connection.SetMaxIdleConns(maxIdleConnections)
	connection.SetConnMaxLifetime(connectionLifetime)

	return connection, nil
}
