// Package budget implements integer-only, conservative run and project accounting.
package budget

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

const Model = "deepseek/deepseek-v4.1-flash"
const Provider = "deepinfra/fp8"
const Profile = "phase1-demo"
const Protocol = "feam.web.v1"

var ErrInvalid = errors.New("invalid allocation or accounting state")
var ErrExhausted = errors.New("budget_exhausted")
var ErrConflict = errors.New("conflict")
var idRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ValidID(s string) bool { return idRE.MatchString(s) }

// Prices are USD microdollars per million tokens, not floating point dollars.
// FeeBasisPoints includes all applicable fees; 10000 means no extra surcharge.
type Allocation struct {
	Protocol             string    `json:"protocol"`
	ID                   string    `json:"allocation_id"`
	ProjectID            string    `json:"project_id"`
	RunID                string    `json:"run_id"`
	DeploymentID         string    `json:"deployment_id"`
	ActivationGeneration int64     `json:"activation_generation"`
	LedgerRevision       int64     `json:"ledger_revision"`
	Amount               int64     `json:"amount_usd_micros"`
	RequestLimit         int64     `json:"request_limit_usd_micros"`
	UserDailyLimit       int64     `json:"user_daily_limit_usd_micros"`
	Model                string    `json:"model"`
	Provider             string    `json:"provider"`
	Profile              string    `json:"profile"`
	InputPrice           int64     `json:"input_price_usd_micros_per_million"`
	OutputPrice          int64     `json:"output_price_usd_micros_per_million"`
	FeeBasisPoints       int64     `json:"fee_basis_points"`
	MaxOutputTokens      int64     `json:"max_output_tokens"`
	NotBefore            time.Time `json:"not_before"`
	ExpiresAt            time.Time `json:"expires_at"`
}

type SignedAllocation struct {
	Allocation Allocation `json:"allocation"`
	Signature  []byte     `json:"signature"`
}

func (a Allocation) Validate() error {
	if a.RequestLimit < 1 || a.RequestLimit > a.Amount || a.UserDailyLimit < 1 || a.UserDailyLimit > a.Amount {
		return ErrInvalid
	}
	if a.Protocol != Protocol || !ValidID(a.ID) || !ValidID(a.ProjectID) || !ValidID(a.RunID) || !ValidID(a.DeploymentID) || a.ActivationGeneration < 1 || a.LedgerRevision < 1 || a.Amount < 1 || a.Amount > 1_000_000_000_000 || a.Model != Model || a.Provider != Provider || a.Profile != Profile || a.InputPrice < 1 || a.OutputPrice < 1 || a.InputPrice > 1_000_000_000 || a.OutputPrice > 1_000_000_000 || a.FeeBasisPoints < 10000 || a.FeeBasisPoints > 100000 || a.MaxOutputTokens < 1 || a.MaxOutputTokens > 8192 || a.NotBefore.IsZero() || !a.ExpiresAt.After(a.NotBefore) || a.ExpiresAt.Sub(a.NotBefore) > 7*24*time.Hour {
		return ErrInvalid
	}
	return nil
}

func Sign(a Allocation, key ed25519.PrivateKey) (SignedAllocation, error) {
	if err := a.Validate(); err != nil {
		return SignedAllocation{}, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return SignedAllocation{}, ErrInvalid
	}
	b, _ := json.Marshal(a)
	return SignedAllocation{a, ed25519.Sign(key, b)}, nil
}

func (s SignedAllocation) Verify(key ed25519.PublicKey, deployment string, generation int64, now time.Time) error {
	if err := s.Allocation.Validate(); err != nil {
		return err
	}
	b, _ := json.Marshal(s.Allocation)
	if len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, b, s.Signature) || s.Allocation.DeploymentID != deployment || s.Allocation.ActivationGeneration != generation || now.Before(s.Allocation.NotBefore) || !now.Before(s.Allocation.ExpiresAt) {
		return ErrInvalid
	}
	return nil
}

func (s SignedAllocation) Digest() string {
	b, _ := json.Marshal(s)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// ReserveCost uses UTF-8 request bytes as a conservative upper bound on input
// tokens. Reasoning must be disabled and max_tokens enforced by the broker.
func (a Allocation) ReserveCost(requestBytes int64) (int64, error) {
	if a.Validate() != nil || requestBytes < 1 || requestBytes > 1<<20 {
		return 0, ErrInvalid
	}
	n := requestBytes*a.InputPrice + a.MaxOutputTokens*a.OutputPrice
	n = (n + 999999) / 1000000
	return (n*a.FeeBasisPoints + 9999) / 10000, nil
}
