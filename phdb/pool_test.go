package phdb

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDefaultPoolConfig(t *testing.T) {
	t.Parallel()
	c := DefaultPoolConfig()
	if c.MaxOpenConns != 15 || c.MaxIdleConns != 15 {
		t.Fatalf("defaults: got MaxOpen=%d MaxIdle=%d, want 15/15", c.MaxOpenConns, c.MaxIdleConns)
	}
	if c.ConnMaxLifetime != 30*time.Minute || c.ConnMaxIdleTime != 5*time.Minute {
		t.Fatalf("defaults: got life=%s idle=%s, want 30m/5m", c.ConnMaxLifetime, c.ConnMaxIdleTime)
	}
}

func TestPoolConfigValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     PoolConfig
		wantErr bool
	}{
		{"valid", PoolConfig{MaxOpenConns: 20, MaxIdleConns: 20, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute}, false},
		{"unbounded maxopen", PoolConfig{MaxOpenConns: 0, MaxIdleConns: 5, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute}, true},
		{"negative maxopen", PoolConfig{MaxOpenConns: -1, MaxIdleConns: 5, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute}, true},
		{"idle gt open clamps ok", PoolConfig{MaxOpenConns: 10, MaxIdleConns: 50, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute}, false},
		{"negative maxidle", PoolConfig{MaxOpenConns: 10, MaxIdleConns: -1, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadPoolConfig(t *testing.T) {
	t.Run("defaults when unset", func(t *testing.T) {
		c := LoadPoolConfig("DB")
		want := DefaultPoolConfig()
		if c.MaxOpenConns != want.MaxOpenConns ||
			c.MaxIdleConns != want.MaxIdleConns ||
			c.ConnMaxLifetime != want.ConnMaxLifetime ||
			c.ConnMaxIdleTime != want.ConnMaxIdleTime {
			t.Fatalf("unset: got %+v, want defaults %+v", c, want)
		}
	})
	t.Run("reads DB_ prefix (minutes for durations)", func(t *testing.T) {
		t.Setenv("DB_MAX_OPEN_CONN", "20")
		t.Setenv("DB_MAX_IDLE_CONN", "20")
		t.Setenv("DB_CONN_MAX_LIFETIME", "30")
		t.Setenv("DB_CONN_MAX_IDLE_TIME", "5")
		c := LoadPoolConfig("DB")
		if c.MaxOpenConns != 20 || c.MaxIdleConns != 20 ||
			c.ConnMaxLifetime != 30*time.Minute || c.ConnMaxIdleTime != 5*time.Minute {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("family prefix DB_RPL", func(t *testing.T) {
		t.Setenv("DB_RPL_MAX_OPEN_CONN", "12")
		c := LoadPoolConfig("DB_RPL")
		if c.MaxOpenConns != 12 {
			t.Fatalf("got MaxOpen=%d, want 12", c.MaxOpenConns)
		}
	})
	t.Run("family prefix DB_ACC", func(t *testing.T) {
		t.Setenv("DB_ACC_MAX_OPEN_CONN", "8")
		t.Setenv("DB_ACC_MAX_IDLE_CONN", "8")
		c := LoadPoolConfig("DB_ACC")
		if c.MaxOpenConns != 8 || c.MaxIdleConns != 8 {
			t.Fatalf("got MaxOpen=%d MaxIdle=%d, want 8/8", c.MaxOpenConns, c.MaxIdleConns)
		}
	})
	t.Run("invalid value falls back to default (not unbounded)", func(t *testing.T) {
		t.Setenv("DB_MAX_OPEN_CONN", "0")
		c := LoadPoolConfig("DB")
		if c.MaxOpenConns != DefaultPoolConfig().MaxOpenConns {
			t.Fatalf("0 should fall back to default, got %d", c.MaxOpenConns)
		}
	})
	t.Run("negative value falls back to default", func(t *testing.T) {
		t.Setenv("DB_MAX_OPEN_CONN", "-5")
		c := LoadPoolConfig("DB")
		if c.MaxOpenConns != DefaultPoolConfig().MaxOpenConns {
			t.Fatalf("-5 should fall back to default, got %d", c.MaxOpenConns)
		}
	})
}

func TestApplySetsMaxOpen(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = mock
	cfg := PoolConfig{
		MaxOpenConns:    7,
		MaxIdleConns:    7,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Minute,
		Logger:          func(string, ...any) {},
	}
	if err := Apply(db, cfg); err != nil {
		t.Fatal(err)
	}
	if got := db.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("MaxOpenConnections=%d, want 7", got)
	}
}

func TestApplyRejectsUnbounded(t *testing.T) {
	t.Parallel()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Apply(db, PoolConfig{MaxOpenConns: 0}); err == nil {
		t.Fatal("expected error for unbounded pool")
	}
}

func TestApplyRejectsNilDB(t *testing.T) {
	t.Parallel()
	if err := Apply(nil, DefaultPoolConfig()); err == nil {
		t.Fatal("expected error for nil sqlDB")
	}
}

func TestApplyClampsIdleToOpen(t *testing.T) {
	t.Parallel()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := PoolConfig{
		MaxOpenConns:    5,
		MaxIdleConns:    50,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Minute,
		Logger:          func(string, ...any) {},
	}
	if err := Apply(db, cfg); err != nil {
		t.Fatal(err)
	}
	if got := db.Stats().MaxOpenConnections; got != 5 {
		t.Fatalf("MaxOpenConnections=%d, want 5", got)
	}
}

func TestLifetimeWithJitter(t *testing.T) {
	t.Parallel()
	base := 30 * time.Minute
	upper := base + base/10 // exclusive upper bound of rand.N
	for i := 0; i < 50; i++ {
		got := lifetimeWithJitter(base)
		if got < base || got >= upper {
			t.Fatalf("jitter out of range: got %s, want [%s, %s)", got, base, upper)
		}
	}
	if got := lifetimeWithJitter(0); got != 0 {
		t.Fatalf("zero lifetime: got %s, want 0", got)
	}
}

func TestApplyWithCustomLogger(t *testing.T) {
	t.Parallel()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	called := false
	cfg := DefaultPoolConfig()
	cfg.Logger = func(format string, args ...any) {
		called = true
		if format == "" {
			t.Fatal("empty log format")
		}
		_ = args
	}
	if err := Apply(db, cfg); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected custom Logger to be called")
	}
}

func TestApplyWithoutCustomLogger(t *testing.T) {
	t.Parallel()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Apply(db, DefaultPoolConfig()); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAndApply(t *testing.T) {
	t.Parallel()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cfg := DefaultPoolConfig()
	cfg.Logger = func(string, ...any) {}
	got, err := OpenAndApply(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != db {
		t.Fatal("OpenAndApply should return the same *sql.DB")
	}
	if got.Stats().MaxOpenConnections != cfg.MaxOpenConns {
		t.Fatalf("MaxOpen=%d, want %d", got.Stats().MaxOpenConnections, cfg.MaxOpenConns)
	}
}

func TestOpenAndApplyRejectsUnbounded(t *testing.T) {
	t.Parallel()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := OpenAndApply(db, PoolConfig{MaxOpenConns: 0}); err == nil {
		t.Fatal("expected error for unbounded pool")
	}
}
