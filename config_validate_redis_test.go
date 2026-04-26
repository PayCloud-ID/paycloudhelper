package paycloudhelper

import (
	"testing"
)

// ValidateConfiguration includes Redis.* checks when redisOptions is set.
func TestValidateConfiguration_withRedisOptions(t *testing.T) {
	_ = setupMiniredis(t)
	t.Setenv("APP_NAME", "cfg-redis-test")
	t.Setenv("APP_ENV", "develop")
	t.Setenv("SENTRY_DSN", "")

	errs := ValidateConfiguration()
	if len(errs) == 0 {
		t.Fatal("expected at least one validation entry")
	}
	foundRedis := false
	for _, e := range errs {
		if e.Field == "Redis.Password" || e.Field == "Redis.Addr" {
			foundRedis = true
			break
		}
	}
	if !foundRedis {
		t.Fatalf("expected Redis-related ConfigError, got %#v", errs)
	}
}
