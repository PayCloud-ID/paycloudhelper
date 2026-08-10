package pcauth

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestHashIsCompatibleWithDeployedHashes(t *testing.T) {
	const (
		plaintext = "PassQwerty123!"
		salt      = "e07ac3ac-5cc0-4b38-8087-96444a2155c7"
		hash      = "$2a$10$Uz7j/Nl.7FEhGrsXgIfg8.SfNkPFtTsbs18xDJYmypD.6rr6nJXMa"
	)

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext+salt)); err != nil {
		t.Fatalf("scheme drift: %v", err)
	}
}

func TestGenerateHashAndSaltUsesConfiguredCost(t *testing.T) {
	restoreDefaultCost(t)
	ConfigureCost(11)

	salt, hash, err := GenerateHashAndSalt("hunter2")
	if err != nil {
		t.Fatalf("GenerateHashAndSalt: %v", err)
	}
	if _, err := uuid.Parse(salt); err != nil {
		t.Fatalf("salt is not a UUID: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("hunter2"+salt)); err != nil {
		t.Fatalf("generated hash does not verify: %v", err)
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost != 11 {
		t.Fatalf("cost=%d, want 11", cost)
	}
}

func TestConfigureCostClampsSupportedRange(t *testing.T) {
	restoreDefaultCost(t)

	ConfigureCost(bcrypt.MinCost - 1)
	if got := bcryptCost(); got != bcrypt.MinCost {
		t.Fatalf("minimum-clamped cost=%d, want %d", got, bcrypt.MinCost)
	}

	ConfigureCost(maximumCost + 1)
	if got := bcryptCost(); got != maximumCost {
		t.Fatalf("maximum-clamped cost=%d, want %d", got, maximumCost)
	}
}

func TestConfigureCostProviderIsEvaluatedPerCall(t *testing.T) {
	restoreDefaultCost(t)
	cost := 6
	ConfigureCostProvider(func() int { return cost })

	if got := bcryptCost(); got != 6 {
		t.Fatalf("provider cost=%d, want 6", got)
	}
	cost = 7
	if got := bcryptCost(); got != 7 {
		t.Fatalf("updated provider cost=%d, want 7", got)
	}
}

func TestGenerateHashAndSaltReturnsBcryptError(t *testing.T) {
	restoreDefaultCost(t)
	tooLong := strings.Repeat("x", 73)

	salt, hash, err := GenerateHashAndSalt(tooLong)
	if !errors.Is(err, bcrypt.ErrPasswordTooLong) {
		t.Fatalf("error=%v, want ErrPasswordTooLong", err)
	}
	if salt != "" || hash != "" {
		t.Fatalf("salt=%q hash=%q, want both empty", salt, hash)
	}
}

func TestVerifyAndMaybeRehash(t *testing.T) {
	restoreDefaultCost(t)
	const plaintext = "hunter2"

	t.Run("upgrades weaker cost", func(t *testing.T) {
		salt := uuid.NewString()
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(plaintext+salt), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("GenerateFromPassword: %v", err)
		}

		result, err := VerifyAndMaybeRehash(plaintext, salt, string(hashBytes))
		if err != nil {
			t.Fatalf("VerifyAndMaybeRehash: %v", err)
		}
		if !result.Matched || !result.NeedsRehash {
			t.Fatalf("result=%+v, want matched rehash", result)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(result.NewHash), []byte(plaintext+result.NewSalt)); err != nil {
			t.Fatalf("replacement hash does not verify: %v", err)
		}
	})

	t.Run("keeps current cost", func(t *testing.T) {
		salt, hash, err := GenerateHashAndSalt(plaintext)
		if err != nil {
			t.Fatalf("GenerateHashAndSalt: %v", err)
		}

		result, err := VerifyAndMaybeRehash(plaintext, salt, hash)
		if err != nil {
			t.Fatalf("VerifyAndMaybeRehash: %v", err)
		}
		if !result.Matched || result.NeedsRehash || result.NewSalt != "" || result.NewHash != "" {
			t.Fatalf("result=%+v, want matched without rehash", result)
		}
	})

	t.Run("rejects wrong password", func(t *testing.T) {
		salt, hash, err := GenerateHashAndSalt(plaintext)
		if err != nil {
			t.Fatalf("GenerateHashAndSalt: %v", err)
		}

		result, err := VerifyAndMaybeRehash("wrong-password", salt, hash)
		if err != nil {
			t.Fatalf("VerifyAndMaybeRehash: %v", err)
		}
		if result.Matched || result.NeedsRehash {
			t.Fatalf("result=%+v, want no match", result)
		}
	})

	t.Run("accepts stronger cost without downgrade", func(t *testing.T) {
		salt := uuid.NewString()
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(plaintext+salt), DefaultCost+1)
		if err != nil {
			t.Fatalf("GenerateFromPassword: %v", err)
		}

		result, err := VerifyAndMaybeRehash(plaintext, salt, string(hashBytes))
		if err != nil {
			t.Fatalf("VerifyAndMaybeRehash: %v", err)
		}
		if !result.Matched || result.NeedsRehash {
			t.Fatalf("result=%+v, want matched without downgrade", result)
		}
	})

	t.Run("treats empty material as no match", func(t *testing.T) {
		for _, tc := range []struct{ salt, hash string }{{"", "hash"}, {"salt", ""}, {" ", "hash"}} {
			result, err := VerifyAndMaybeRehash(plaintext, tc.salt, tc.hash)
			if err != nil || result.Matched || result.NeedsRehash {
				t.Fatalf("VerifyAndMaybeRehash(%q, %q) result=%+v err=%v", tc.salt, tc.hash, result, err)
			}
		}
	})

	t.Run("returns malformed hash error", func(t *testing.T) {
		result, err := VerifyAndMaybeRehash(plaintext, "salt", "not-bcrypt")
		if err == nil || result.Matched {
			t.Fatalf("result=%+v err=%v, want error without match", result, err)
		}
	})
}

func restoreDefaultCost(t *testing.T) {
	t.Helper()
	previousCost := configuredCost.Load()
	previousProvider := configuredCostProvider.Load()
	t.Cleanup(func() {
		configuredCost.Store(previousCost)
		configuredCostProvider.Store(previousProvider)
	})
}
