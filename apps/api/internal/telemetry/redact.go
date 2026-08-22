package telemetry

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// This file is the single choke point for deciding what may be written to a
// span (nester#1054).
//
// Nester moves user money, so telemetry is treated as an untrusted egress
// channel: spans leave the process, land in a trace backend, and are readable
// by anyone with dashboard access. Anything secret that reaches a span is
// disclosed. The rule applied throughout is allow-list, not deny-list —
// instrumentation records a small set of known-safe, low-cardinality facts
// rather than dumping structures and hoping nothing sensitive is inside.
//
// Values that must never appear on a span:
//   - Stellar secret seeds (S...) and any private key material
//   - Signed transaction XDRs (they embed signatures and full operation data)
//   - JWTs, session tokens, API keys, Authorization headers
//   - SQL parameter values (they carry user financial data)
//   - Raw prompts and model responses (they carry user financial data)
//   - Account numbers, balances, and other user financial records

// maxAttributeLen bounds any free-form string written to a span. Long values
// are both a cardinality problem and a disclosure risk: truncation limits how
// much of an accidentally-sensitive value could ever escape.
const maxAttributeLen = 256

// RedactedPlaceholder replaces any value judged sensitive.
const RedactedPlaceholder = "[REDACTED]"

var (
	// stellarSecretPattern matches a Stellar secret seed. StrKey seeds are
	// base32, start with 'S', and are 56 characters long. Matching this on any
	// value bound for a span is a backstop against an operator secret being
	// passed where a public address was expected.
	stellarSecretPattern = regexp.MustCompile(`\bS[A-Z2-7]{55}\b`)

	// jwtPattern matches a three-segment JSON Web Token.
	jwtPattern = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`)

	// bearerPattern matches an Authorization header value.
	bearerPattern = regexp.MustCompile(`(?i)\b(bearer|basic)\s+\S+`)

	// anthropicKeyPattern matches an Anthropic API key.
	anthropicKeyPattern = regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]+`)

	// genericKeyPattern matches common API-key prefixes from the payment
	// providers this codebase integrates with (Paystack, Flutterwave, Stripe).
	genericKeyPattern = regexp.MustCompile(`\b(sk|pk|rk)_(live|test)_[A-Za-z0-9]+`)
)

// sensitiveKeyFragments name attribute keys whose values are never safe to
// record, regardless of content. Matching is case-insensitive substring.
var sensitiveKeyFragments = []string{
	"secret",
	"password",
	"passwd",
	"authorization",
	"api_key",
	"apikey",
	"private",
	"credential",
	"signature",
	"seed",
	"xdr",
	"prompt",
	"completion",
	"response_body",
	"request_body",
	"account_number",
	"balance",
	"cipher",
	"jwt",
	"parameter",
	"binding",
	"arg_value",
}

// safeKeyExceptions are keys that a fragment rule would otherwise reject but
// which carry no sensitive content. Token *counts* are the motivating case:
// "token" appears in the key, yet the value is an integer the issue explicitly
// requires on Anthropic spans. Exceptions are matched as whole keys or as a
// suffix so a crafted key cannot smuggle a secret past the fragment rules.
var safeKeyExceptions = []string{
	"input_tokens",
	"output_tokens",
	"total_tokens",
	"token_count",
	"max_tokens",
}

// IsSensitiveKey reports whether an attribute key names a value that must
// never be exported.
func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(key)

	for _, exception := range safeKeyExceptions {
		if lower == exception || strings.HasSuffix(lower, "."+exception) {
			return false
		}
	}

	// "token" is handled separately from the fragment list so the exceptions
	// above can carve out token counts without also weakening the other rules.
	if strings.Contains(lower, "token") {
		return true
	}

	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

// RedactValue strips secret material from a free-form string and truncates it.
//
// This is a defence in depth, not the primary control. The primary control is
// that instrumentation only ever records explicitly-chosen safe fields. This
// function exists so that a future call site which records something
// unexpected still cannot leak a key, a token, or a signed transaction.
func RedactValue(value string) string {
	if value == "" {
		return ""
	}

	redacted := stellarSecretPattern.ReplaceAllString(value, RedactedPlaceholder)
	redacted = jwtPattern.ReplaceAllString(redacted, RedactedPlaceholder)
	redacted = bearerPattern.ReplaceAllString(redacted, RedactedPlaceholder)
	redacted = anthropicKeyPattern.ReplaceAllString(redacted, RedactedPlaceholder)
	redacted = genericKeyPattern.ReplaceAllString(redacted, RedactedPlaceholder)

	return truncate(redacted)
}

// SafeAttribute builds a string attribute with the key policy and value
// redaction applied. A sensitive key yields the placeholder rather than being
// dropped, so the shape of a trace stays stable and the omission is visible to
// whoever is reading it.
func SafeAttribute(key, value string) attribute.KeyValue {
	if IsSensitiveKey(key) {
		return attribute.String(key, RedactedPlaceholder)
	}
	return attribute.String(key, RedactValue(value))
}

// SetSafeAttributes applies SafeAttribute to each pair and records them.
// Pairs must be key/value strings; a trailing odd element is ignored.
func SetSafeAttributes(span trace.Span, keyValues ...string) {
	if span == nil || !span.IsRecording() {
		return
	}
	attrs := make([]attribute.KeyValue, 0, len(keyValues)/2)
	for i := 0; i+1 < len(keyValues); i += 2 {
		attrs = append(attrs, SafeAttribute(keyValues[i], keyValues[i+1]))
	}
	span.SetAttributes(attrs...)
}

// RecordError marks a span as failed and flags it for retention.
//
// It deliberately records only the error's type and a redacted message. Error
// strings in this codebase can interpolate values (a DSN, an RPC payload, a
// contract argument), so they are passed through RedactValue rather than
// trusted. Errors are always marked for retention because an unsampled error
// trace is the exact trace an operator needs during an incident.
func RecordError(span trace.Span, err error) {
	if span == nil || !span.IsRecording() || err == nil {
		return
	}

	// The SDK's RecordError writes err.Error() verbatim into the exception
	// event, so the raw error must never be handed to it. Wrapping the
	// redacted text in a fresh error preserves the event shape (including the
	// original type name, which is diagnostic and not sensitive) while
	// guaranteeing the exported message has been through redaction.
	redactedMsg := RedactValue(err.Error())

	span.SetStatus(codes.Error, redactedMsg)
	span.RecordError(
		errors.New(redactedMsg),
		trace.WithAttributes(attribute.String("exception.type", errorTypeName(err))),
	)
	MarkForRetention(span)
}

// errorTypeName reports the concrete Go type of an error. The type name is a
// compile-time identifier, never user data, so it is safe to export and is
// often the fastest way to classify a failure on a trace.
func errorTypeName(err error) string {
	return fmt.Sprintf("%T", err)
}

// truncate bounds a value's length, appending an ellipsis when it was cut.
func truncate(value string) string {
	if len(value) <= maxAttributeLen {
		return value
	}
	return value[:maxAttributeLen] + "..."
}
