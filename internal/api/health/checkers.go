package health

import (
	"context"
	"database/sql"
	"fmt"
)

// SQLiteChecker checks SQLite database connectivity.
type SQLiteChecker struct {
	db *sql.DB
}

// NewSQLiteChecker creates a new SQLite health checker.
func NewSQLiteChecker(db *sql.DB) *SQLiteChecker {
	return &SQLiteChecker{db: db}
}

// Name returns the checker name.
func (c *SQLiteChecker) Name() string {
	return "sqlite"
}

// Check verifies the SQLite database is accessible.
func (c *SQLiteChecker) Check(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return c.db.PingContext(ctx)
}

// Pinger interface for databases that support ping.
type Pinger interface {
	Ping(ctx context.Context) error
}

// ClickHouseChecker checks ClickHouse connectivity.
type ClickHouseChecker struct {
	pinger Pinger
}

// NewClickHouseChecker creates a new ClickHouse health checker.
func NewClickHouseChecker(p Pinger) *ClickHouseChecker {
	return &ClickHouseChecker{pinger: p}
}

// Name returns the checker name.
func (c *ClickHouseChecker) Name() string {
	return "clickhouse"
}

// Check verifies ClickHouse is accessible.
func (c *ClickHouseChecker) Check(ctx context.Context) error {
	if c.pinger == nil {
		return fmt.Errorf("clickhouse not configured")
	}
	return c.pinger.Ping(ctx)
}

// GRPCChecker checks if the gRPC server is accepting connections.
type GRPCChecker struct {
	isRunning func() bool
}

// NewGRPCChecker creates a new gRPC health checker.
func NewGRPCChecker(isRunning func() bool) *GRPCChecker {
	return &GRPCChecker{isRunning: isRunning}
}

// Name returns the checker name.
func (c *GRPCChecker) Name() string {
	return "grpc"
}

// Check verifies the gRPC server is running.
func (c *GRPCChecker) Check(ctx context.Context) error {
	if c.isRunning == nil || !c.isRunning() {
		return fmt.Errorf("grpc server not running")
	}
	return nil
}
