package database

import (
	"crypto/tls"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-sql-driver/mysql"
)

const destTLSConfigName = "dest-mysql-tls"

// DestinationDBConfig holds connection parameters for the destination
// (DigitalOcean Managed MySQL) database that mirrors the source tables.
type DestinationDBConfig struct {
	Host     string
	Port     uint16
	User     string
	Password string
	DBName   string
}

// NewDestinationConnector opens a pooled *sql.DB to the destination MySQL
// instance over TLS, using tlsConfig as built by utils.SetupTLS from the
// pinned CA cert. It retries the initial connection with backoff since this
// is a long-running daemon that may start before the destination is
// reachable.
func NewDestinationConnector(cfg DestinationDBConfig, tlsConfig *tls.Config) (*sql.DB, error) {
	if err := mysql.RegisterTLSConfig(destTLSConfigName, tlsConfig); err != nil {
		return nil, fmt.Errorf("failed to register destination TLS config: %w", err)
	}

	driverCfg := mysql.NewConfig()
	driverCfg.User = cfg.User
	driverCfg.Passwd = cfg.Password
	driverCfg.Net = "tcp"
	driverCfg.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	driverCfg.DBName = cfg.DBName
	driverCfg.ParseTime = true
	driverCfg.Loc = time.UTC
	driverCfg.Collation = "utf8mb4_general_ci"
	driverCfg.TLSConfig = destTLSConfigName

	connector, err := mysql.NewConnector(driverCfg)
	if err != nil {
		return nil, fmt.Errorf("invalid destination MySQL config: %w", err)
	}

	db, err := connectWithRetry(connector, 5, 2*time.Second)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	return db, nil
}

func connectWithRetry(connector driver.Connector, maxRetries int, baseDelay time.Duration) (*sql.DB, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		db := sql.OpenDB(connector)
		if pingErr := db.Ping(); pingErr != nil {
			slog.Error("Failed to ping destination MySQL", "attempt", i+1, "error", pingErr)
			db.Close()
			lastErr = pingErr
			time.Sleep(baseDelay * time.Duration(1<<i))
			continue
		}
		return db, nil
	}

	return nil, fmt.Errorf("could not connect to destination MySQL after %d attempts: %w", maxRetries, lastErr)
}
