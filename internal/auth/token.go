// @relay-project: relay
// @relay-path: internal/auth/token.go
// Package auth handles tunnel authentication and identity verification.
// ADR-041: relay token validates tunnel ownership (constant-time compare).
// ADR-042: identity token validated against Gate for inbound requests.
package auth

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ValidateRelayToken checks the relay token using constant-time comparison.
// Returns true if the presented token matches the expected token.
// Always returns false if expected is empty.
func ValidateRelayToken(presented, expected string) bool {
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}

// IdentityClaim is the Gate-validated actor identity.
// Mirrors accord.IdentityClaimDTO — inlined to avoid premature Accord dependency.
type IdentityClaim struct {
	Subject   string   `json:"sub"`
	Scopes    []string `json:"scp"`
	ExpiresAt int64    `json:"exp"`
	TokenID   string   `json:"jti"`
}

// HasScope returns true if the claim contains the requested scope.
func (c *IdentityClaim) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// GateValidator validates identity tokens against Gate (ADR-042).
type GateValidator struct {
	gateAddr     string
	serviceToken string
	client       *http.Client
}

// NewGateValidator creates a GateValidator.
func NewGateValidator(gateAddr, serviceToken string) *GateValidator {
	return &GateValidator{
		gateAddr:     gateAddr,
		serviceToken: serviceToken,
		client:       &http.Client{Timeout: 3 * time.Second},
	}
}

// Validate calls POST /gate/validate to check the token's signature,
// expiry, and revocation status.
// Returns (nil, nil) if token is absent — callers decide if anonymous is allowed.
func (v *GateValidator) Validate(identityToken string) (*IdentityClaim, error) {
	if identityToken == "" {
		return nil, nil // anonymous — not an error
	}

	body := fmt.Sprintf(`{"token":%q}`, identityToken)
	req, err := http.NewRequest(http.MethodPost, v.gateAddr+"/gate/validate",
		strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gate validate: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if v.serviceToken != "" {
		req.Header.Set("X-Service-Token", v.serviceToken)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gate validate: request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Valid  bool           `json:"valid"`
		Claim  *IdentityClaim `json:"claim,omitempty"`
		Reason string         `json:"reason,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gate validate: decode: %w", err)
	}
	if !result.Valid {
		return nil, fmt.Errorf("gate validate: %s", result.Reason)
	}
	return result.Claim, nil
}
