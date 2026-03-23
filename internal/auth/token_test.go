// @relay-project: relay
// @relay-path: internal/auth/token_test.go
package auth

import "testing"

func TestValidateRelayToken(t *testing.T) {
	tests := []struct {
		name      string
		presented string
		expected  string
		want      bool
	}{
		{"matching tokens", "secret-abc", "secret-abc", true},
		{"mismatched tokens", "wrong", "secret-abc", false},
		{"empty presented", "", "secret-abc", false},
		{"empty expected — always deny", "anything", "", false},
		{"both empty — deny", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateRelayToken(tt.presented, tt.expected)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIdentityClaimDTO_HasScope(t *testing.T) {
	c := &IdentityClaimDTO{Scopes: []string{"execute", "observe"}}
	if !c.HasScope("execute") {
		t.Error("expected execute scope")
	}
	if !c.HasScope("observe") {
		t.Error("expected observe scope")
	}
	if c.HasScope("admin") {
		t.Error("should not have admin scope")
	}
	if c.HasScope("") {
		t.Error("empty scope should not match")
	}
}
