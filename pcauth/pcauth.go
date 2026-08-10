// Package pcauth provides PayCloud's canonical password hashing.
//
// The scheme is bcrypt over the plaintext with a per-user UUID appended as a
// salt:
//
//	hash = bcrypt(plaintext + salt, cost)
//
// Both values are stored in the clear. This mirrors the scheme verified by the
// clientpg-manager login path. Do not vary the concatenation order or salt
// shape: existing hashes cannot be re-derived.
package pcauth

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// DefaultCost is the bcrypt cost used for newly issued hashes when no override
// is configured. It matches the cost used by existing PayCloud password hashes.
const DefaultCost = 10

const maximumCost = 14

var configuredCost atomic.Int32
var configuredCostProvider atomic.Pointer[costProvider]

type costProvider struct {
	load func() int
}

// VerificationResult contains the password verification outcome and optional
// replacement material when a weaker bcrypt hash should be upgraded.
type VerificationResult struct {
	Matched     bool
	NeedsRehash bool
	NewSalt     string
	NewHash     string
}

// ConfigureCost sets the bcrypt cost used for new hashes and rehashes. Values
// outside PayCloud's operational range are clamped to bcrypt.MinCost through 14.
// Consumers normally rely on paycloudhelper.InitializeApp to configure this
// from BCRYPT_COST.
func ConfigureCost(cost int) {
	configuredCost.Store(int32(clampCost(cost)))
	configuredCostProvider.Store(nil)
}

// ConfigureCostProvider sets a concurrency-safe source for the bcrypt cost.
// The returned value is clamped using the same policy as ConfigureCost. The
// root paycloudhelper package uses this to honor BCRYPT_COST without coupling
// this standalone subpackage to environment loading.
func ConfigureCostProvider(provider func() int) {
	if provider == nil {
		configuredCostProvider.Store(nil)
		return
	}
	configuredCostProvider.Store(&costProvider{load: provider})
}

// GenerateHashAndSalt creates a UUID salt and hashes plaintext+salt with the
// configured bcrypt cost.
func GenerateHashAndSalt(plaintext string) (salt, hash string, err error) {
	return generateHashAndSaltAtCost(plaintext, bcryptCost())
}

// VerifyAndMaybeRehash verifies a password and, after a successful match,
// returns replacement material when the stored hash uses a lower bcrypt cost.
func VerifyAndMaybeRehash(plaintext, salt, hash string) (VerificationResult, error) {
	var result VerificationResult

	if strings.TrimSpace(salt) == "" || strings.TrimSpace(hash) == "" {
		return result, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext+salt)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return result, nil
		}
		return result, fmt.Errorf("compare password: %w", err)
	}
	result.Matched = true

	currentCost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return result, fmt.Errorf("read password hash cost: %w", err)
	}
	targetCost := bcryptCost()
	if currentCost >= targetCost {
		return result, nil
	}

	newSalt, newHash, err := generateHashAndSaltAtCost(plaintext, targetCost)
	if err != nil {
		return result, fmt.Errorf("rehash password: %w", err)
	}

	result.NeedsRehash = true
	result.NewSalt = newSalt
	result.NewHash = newHash
	return result, nil
}

func bcryptCost() int {
	if provider := configuredCostProvider.Load(); provider != nil {
		return clampCost(provider.load())
	}
	cost := int(configuredCost.Load())
	if cost == 0 {
		return DefaultCost
	}
	return cost
}

func clampCost(cost int) int {
	if cost < bcrypt.MinCost {
		return bcrypt.MinCost
	}
	if cost > maximumCost {
		return maximumCost
	}
	return cost
}

func generateHashAndSaltAtCost(plaintext string, cost int) (salt, hash string, err error) {
	salt = uuid.NewString()
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(plaintext+salt), cost)
	if err != nil {
		return "", "", err
	}
	return salt, string(hashBytes), nil
}
