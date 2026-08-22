package telemetry

import (
	"strings"
	"testing"
)

// The literals in this file are syntactically valid but deliberately fake.
// They exist so the redaction rules are exercised against realistic shapes;
// none of them is a real credential.
const (
	fakeStellarSecret = "S" + "BSVTQO4V6WQNQK4TSFVQVUDCUKYJ2ZQFPKGZVFPWMJXW2WOHVUTPQKZ"
	fakeStellarPublic = "G" + "A5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN"
	fakeJWT           = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" + "." + "eyJzdWIiOiIxMjM0NSJ9" + "." + "dBjftJeZ4CVPmB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	fakeAnthropicKey  = "sk-" + "ant-api03-" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	fakePaystackKey   = "sk_" + "live_" + "abc123def456ghi789"
)

func TestRedactValueStripsSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"stellar secret seed", fakeStellarSecret},
		{"jwt", fakeJWT},
		{"anthropic api key", fakeAnthropicKey},
		{"paystack secret key", fakePaystackKey},
		{"bearer header", "Bearer " + fakeJWT},
		{"secret embedded in error text", "invalid operator secret: " + fakeStellarSecret},
		{"jwt embedded in url", "https://api.example.com/cb?token=" + fakeJWT},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactValue(tc.input)
			if strings.Contains(got, tc.input) && tc.input != "" {
				t.Fatalf("RedactValue returned the raw secret: %q", got)
			}
			if !strings.Contains(got, RedactedPlaceholder) {
				t.Fatalf("RedactValue(%q) = %q, expected it to contain %q", tc.name, got, RedactedPlaceholder)
			}
		})
	}
}

// A public Stellar address is not a secret and is useful on a span for
// correlating chain activity, so redaction must not destroy it.
func TestRedactValuePreservesNonSecrets(t *testing.T) {
	tests := []string{
		fakeStellarPublic,
		"CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
		"deposit",
		"GET /api/v1/portfolio",
	}

	for _, input := range tests {
		if got := RedactValue(input); got != input {
			t.Errorf("RedactValue(%q) = %q, want it unchanged", input, got)
		}
	}
}

func TestRedactValueTruncatesLongValues(t *testing.T) {
	long := strings.Repeat("a", maxAttributeLen*2)
	got := RedactValue(long)
	if len(got) > maxAttributeLen+3 {
		t.Fatalf("RedactValue did not truncate: got length %d", len(got))
	}
}

func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{
		"db.statement.parameters",
		"auth.token",
		"http.request.header.authorization",
		"operator_secret",
		"signed_xdr",
		"anthropic.prompt",
		"model.completion",
		"bank.account_number",
		"user.balance",
		"JWT",
		"Private_Key",
	}
	for _, key := range sensitive {
		if !IsSensitiveKey(key) {
			t.Errorf("IsSensitiveKey(%q) = false, want true", key)
		}
	}

	safe := []string{
		"http.route",
		"db.system",
		"soroban.contract_id",
		"soroban.function",
		"anthropic.model",
		"anthropic.input_tokens",
		"nester.request_id",
	}
	for _, key := range safe {
		if IsSensitiveKey(key) {
			t.Errorf("IsSensitiveKey(%q) = true, want false", key)
		}
	}
}

// SafeAttribute must refuse a sensitive key even when the value itself looks
// innocuous, because the key names a category that is never safe to export.
func TestSafeAttributeRedactsBySensitiveKey(t *testing.T) {
	attr := SafeAttribute("db.statement.parameters", "42")
	if attr.Value.AsString() != RedactedPlaceholder {
		t.Fatalf("SafeAttribute leaked a value under a sensitive key: %q", attr.Value.AsString())
	}
}

func TestSafeAttributeRedactsBySensitiveValue(t *testing.T) {
	attr := SafeAttribute("stellar.address", fakeStellarSecret)
	if strings.Contains(attr.Value.AsString(), fakeStellarSecret) {
		t.Fatalf("SafeAttribute leaked a secret seed: %q", attr.Value.AsString())
	}
}
