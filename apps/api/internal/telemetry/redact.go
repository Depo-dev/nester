package telemetry

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

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
	// None of these patterns use \b anchors. A secret is frequently glued to
	// surrounding text with no word boundary — "secret=SB...", a DSN, an
	// interpolated error string — and an anchored pattern silently fails to
	// match those, which is the worst possible failure mode for a redactor.
	// Matching unanchored costs an occasional over-redaction and is the right
	// trade for a financial application.

	// stellarSecretPattern matches a Stellar secret seed. StrKey seeds are
	// base32, start with 'S', and are 56 characters long. Matching this on any
	// value bound for a span is a backstop against an operator secret being
	// passed where a public address was expected.
	stellarSecretPattern = regexp.MustCompile(`S[A-Z2-7]{55}`)

	// jwtPattern matches a three-segment JSON Web Token.
	jwtPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`)

	// bearerPattern matches an Authorization header value.
	bearerPattern = regexp.MustCompile(`(?i)(bearer|basic)\s+\S+`)

	// anthropicKeyPattern matches an Anthropic API key.
	anthropicKeyPattern = regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]+`)

	// genericKeyPattern matches common API-key prefixes from the payment
	// providers this codebase integrates with (Paystack, Flutterwave, Stripe).
	genericKeyPattern = regexp.MustCompile(`(sk|pk|rk)_(live|test)_[A-Za-z0-9]+`)

	// highEntropyPattern is the last line of defence for truncation. A long
	// base32/base64-ish run that survived every named pattern may still be a
	// credential shape this code does not yet know about; if such a run would
	// be cut by truncation it is redacted whole rather than exported as a
	// partial secret. See truncate.
	highEntropyPattern = regexp.MustCompile(`[A-Za-z0-9_\-+/=]{40,}`)
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
	"statement",
}

// canonicalStatementKeys are the exact OTel semantic-convention keys allowed
// to carry SQL text, and only ever *parameterised* text with placeholders
// left intact — never interpolated values. They are exempted from the
// "statement" fragment rule by exact match, so that any other
// statement-shaped key (db.statement.parameters, db.statement.rendered, a
// bound-parameter variant) is still rejected. The instrumentation that writes
// these keys is responsible for passing parameterised SQL; this list only
// governs whether the key itself is permitted at all.
var canonicalStatementKeys = []string{
	"db.statement",
	"db.query.text",
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
//
// Ordering matters. The fragment rules are evaluated first and win outright:
// an exception may only ever excuse the "token" rule, never any other. This
// prevents a key such as "operator_secret.token_count" or
// "db.statement.max_tokens" from riding an exempt suffix past a rule that
// would otherwise have rejected it.
func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(key)

	// Exact-match allowance for the canonical parameterised-SQL keys. This is
	// checked first and only ever matches the whole key, so derived keys such
	// as "db.statement.parameters" fall through to the fragment rules below.
	for _, allowed := range canonicalStatementKeys {
		if lower == allowed {
			return false
		}
	}

	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}

	if !strings.Contains(lower, "token") {
		return false
	}

	// The key contains "token" and nothing else disqualified it, so a token
	// *count* exception may apply.
	for _, exception := range safeKeyExceptions {
		if lower == exception || strings.HasSuffix(lower, "."+exception) {
			return false
		}
	}
	return true
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

	// Attribute values must be valid UTF-8: OTLP is protobuf, whose string
	// fields require it, and an invalid byte sequence corrupts the attribute
	// on export. Input can be arbitrary — an error string wrapping a raw
	// network read, a byte slice rendered with %s — so coerce it here rather
	// than trusting callers. ToValidUTF8 replaces bad sequences in place,
	// preserving offsets for the patterns below.
	value = strings.ToValidUTF8(value, "")

	redacted := stellarSecretPattern.ReplaceAllString(value, RedactedPlaceholder)
	redacted = jwtPattern.ReplaceAllString(redacted, RedactedPlaceholder)
	redacted = bearerPattern.ReplaceAllString(redacted, RedactedPlaceholder)
	redacted = anthropicKeyPattern.ReplaceAllString(redacted, RedactedPlaceholder)
	redacted = genericKeyPattern.ReplaceAllString(redacted, RedactedPlaceholder)

	return truncate(redacted)
}

// splitsHighEntropyRun reports whether cutting value at limit would land in
// the middle of a long opaque run. Such a cut would export a partial
// credential, which is both a disclosure and useless for debugging.
func splitsHighEntropyRun(value string, limit int) bool {
	for _, loc := range highEntropyPattern.FindAllStringIndex(value, -1) {
		if loc[0] < limit && loc[1] > limit {
			return true
		}
	}
	return false
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
//
// Two hazards are handled. First, cutting mid-run of a long opaque token would
// export a usable prefix of a credential that the named patterns did not
// recognise, so such a value is redacted entirely instead. Second, cutting at
// an arbitrary byte offset can split a multi-byte rune and produce invalid
// UTF-8, so the cut is moved back to a rune boundary.
func truncate(value string) string {
	if len(value) <= maxAttributeLen {
		return value
	}

	if splitsHighEntropyRun(value, maxAttributeLen) {
		return RedactedPlaceholder
	}

	cut := maxAttributeLen
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut] + "..."
}
