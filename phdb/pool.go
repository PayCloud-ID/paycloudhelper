// Package phdb centralizes database/sql connection-pool configuration for
// PayCloud services so pool sizing is consistent, env-driven, and safe (never
// unbounded). See paycloud-docs postgres-migration 01-analysis/05-...
package phdb

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"time"

	"github.com/PayCloud-ID/paycloudhelper/phlogger"
)

// PoolConfig holds the four database/sql pool knobs. All four are always set by
// Apply — including ConnMaxIdleTime, which most services historically omitted.
//
// Logger, when non-nil, receives the Apply log line instead of the default
// phlogger/log fallback (avoids an import cycle with the root paycloudhelper package).
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	Logger          func(format string, args ...any)
}

// DefaultPoolConfig returns conservative defaults sized for a PgBouncer-fronted
// PostgreSQL instance. Kept small on purpose: PgBouncer multiplexes, so large
// per-pod pools only inflate the client-connection count.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    15,
		MaxIdleConns:    15,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// Validate rejects configurations that are unsafe for PostgreSQL. Most importantly
// it forbids MaxOpenConns <= 0, which database/sql treats as UNLIMITED — a fast
// path to exhausting a small PostgreSQL instance.
func (c PoolConfig) Validate() error {
	if c.MaxOpenConns <= 0 {
		return fmt.Errorf("phdb: MaxOpenConns must be > 0 (0 means unlimited); got %d", c.MaxOpenConns)
	}
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("phdb: MaxIdleConns must be >= 0; got %d", c.MaxIdleConns)
	}
	return nil
}

// LoadPoolConfig reads <prefix>_MAX_OPEN_CONN, <prefix>_MAX_IDLE_CONN,
// <prefix>_CONN_MAX_LIFETIME (minutes), <prefix>_CONN_MAX_IDLE_TIME (minutes).
// Missing or non-positive values fall back to DefaultPoolConfig — a bad env value
// can never produce an unbounded pool.
func LoadPoolConfig(prefix string) PoolConfig {
	d := DefaultPoolConfig()
	return PoolConfig{
		MaxOpenConns:    envIntPositive(prefix+"_MAX_OPEN_CONN", d.MaxOpenConns),
		MaxIdleConns:    envIntPositive(prefix+"_MAX_IDLE_CONN", d.MaxIdleConns),
		ConnMaxLifetime: time.Duration(envIntPositive(prefix+"_CONN_MAX_LIFETIME", int(d.ConnMaxLifetime/time.Minute))) * time.Minute,
		ConnMaxIdleTime: time.Duration(envIntPositive(prefix+"_CONN_MAX_IDLE_TIME", int(d.ConnMaxIdleTime/time.Minute))) * time.Minute,
	}
}

func envIntPositive(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

// lifetimeWithJitter stretches lifetime by a per-process random offset of up to
// +10% so connections across a pod do not expire in lockstep after a deploy.
// Precedent: pgxpool.Config.MaxConnLifetimeJitter.
func lifetimeWithJitter(lifetime time.Duration) time.Duration {
	if lifetime <= 0 {
		return lifetime
	}
	jitterBudget := lifetime / 10
	if jitterBudget <= 0 {
		return lifetime
	}
	return lifetime + rand.N(jitterBudget)
}

// Apply validates cfg and applies all four pool knobs to sqlDB, then logs the
// effective configuration (observability parity with dual-engine-pattern.md).
// ConnMaxLifetime is stretched by up to +10% jitter before SetConnMaxLifetime.
func Apply(sqlDB *sql.DB, cfg PoolConfig) error {
	if sqlDB == nil {
		return fmt.Errorf("phdb: sqlDB is nil")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	maxIdle := cfg.MaxIdleConns
	if maxIdle > cfg.MaxOpenConns {
		maxIdle = cfg.MaxOpenConns
	}

	lifetime := lifetimeWithJitter(cfg.ConnMaxLifetime)

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(lifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	logPool(cfg, lifetime)
	return nil
}

// OpenAndApply applies pool configuration to an already-open *sql.DB and returns
// it. DSN building stays per-service / dual-engine — this helper only configures
// the pool knobs.
func OpenAndApply(sqlDB *sql.DB, cfg PoolConfig) (*sql.DB, error) {
	if err := Apply(sqlDB, cfg); err != nil {
		return nil, err
	}
	return sqlDB, nil
}

func logPool(cfg PoolConfig, effectiveLifetime time.Duration) {
	const format = "[phdb] pool applied MaxOpen=%d MaxIdle=%d Lifetime=%s IdleTime=%s"
	args := []any{cfg.MaxOpenConns, cfg.MaxIdleConns, effectiveLifetime, cfg.ConnMaxIdleTime}
	if cfg.Logger != nil {
		cfg.Logger(format, args...)
		return
	}
	// Prefer phlogger (same sink as pchelper.LogI) without importing the root package.
	phlogger.LogI(format, args...)
}
