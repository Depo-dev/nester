package stellar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stellar/go/keypair"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/suncrestlabs/nester/apps/api/internal/telemetry"
)

// newTestOperatorSecret generates a throwaway Stellar keypair for the test.
//
// It is generated rather than hardcoded so that no credential-shaped literal
// is ever committed to the repository — a random key here controls no account
// and holds no value, while still exercising the real signing path that must
// not leak into telemetry.
func newTestOperatorSecret(t *testing.T) string {
	t.Helper()
	kp, err := keypair.Random()
	if err != nil {
		t.Fatalf("generate test keypair: %v", err)
	}
	return kp.Seed()
}

const (
	testContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	testFunction   = "harvest"
)

func newInvokerSpanRecorder(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	return exporter
}

// fakeSorobanRPC stands in for a Soroban RPC node and Horizon. It returns
// well-formed responses containing XDR-shaped payloads so the test can assert
// those payloads never reach a span.
func fakeSorobanRPC(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Horizon account lookup for the sequence number.
		if strings.HasPrefix(r.URL.Path, "/accounts/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sequence":"123456789"}`))
			return
		}

		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "simulateTransaction":
			// A realistic simulation response: transactionData is base64 XDR.
			_, _ = w.Write([]byte(`{"result":{
				"transactionData":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
				"minResourceFee":"12345",
				"results":[{"xdr":"AAAAAQAAAAEAAAAHc2VjcmV0eA=="}]
			}}`))
		case "sendTransaction":
			_, _ = w.Write([]byte(`{"result":{
				"hash":"a1b2c3d4e5f60718293a4b5c6d7e8f901a2b3c4d5e6f708192a3b4c5d6e7f801",
				"status":"PENDING"
			}}`))
		case "getTransaction":
			_, _ = w.Write([]byte(`{"result":{"status":"SUCCESS"}}`))
		default:
			_, _ = w.Write([]byte(`{"result":{}}`))
		}
	}))
}

// forbiddenInSpans are values that must never appear anywhere in exported
// telemetry, checked against every attribute, event and status message.
func assertNoSecretsInSpans(t *testing.T, spans tracetest.SpanStubs, operatorSecret, operatorPublic string) {
	t.Helper()

	forbidden := map[string]string{
		"operator secret seed":  operatorSecret,
		"operator public key":   operatorPublic,
		"simulation XDR":        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"simulation result XDR": "AAAAAQAAAAEAAAAHc2VjcmV0eA==",
	}

	for _, span := range spans {
		var haystack strings.Builder
		haystack.WriteString(span.Name)
		haystack.WriteString(" ")
		haystack.WriteString(span.Status.Description)

		for _, attr := range span.Attributes {
			haystack.WriteString(" ")
			haystack.WriteString(string(attr.Key))
			haystack.WriteString("=")
			haystack.WriteString(attr.Value.String())
		}
		for _, event := range span.Events {
			haystack.WriteString(" ")
			haystack.WriteString(event.Name)
			for _, attr := range event.Attributes {
				haystack.WriteString(" ")
				haystack.WriteString(attr.Value.String())
			}
		}

		exported := haystack.String()
		for label, secret := range forbidden {
			if secret == "" {
				continue
			}
			if strings.Contains(exported, secret) {
				t.Errorf("span %q leaked %s", span.Name, label)
			}
		}

		// No attribute key may itself be one the redactor rejects.
		for _, attr := range span.Attributes {
			if telemetry.IsSensitiveKey(string(attr.Key)) &&
				attr.Value.String() != telemetry.RedactedPlaceholder {
				t.Errorf("span %q recorded sensitive key %q with a real value", span.Name, attr.Key)
			}
		}
	}
}

// A successful invocation must produce a contract span carrying the contract
// ID, the function name, and the transaction hash — and nothing secret.
func TestInvokeVoidFunctionSpanAttributes(t *testing.T) {
	exporter := newInvokerSpanRecorder(t)
	rpc := fakeSorobanRPC(t)
	defer rpc.Close()

	operatorSecret := newTestOperatorSecret(t)
	invoker, err := NewContractInvoker(rpc.URL, rpc.URL, "Test SDF Network ; September 2015", operatorSecret)
	if err != nil {
		t.Fatalf("NewContractInvoker: %v", err)
	}

	// Submit only — InvokeVoidFunction would poll on a 3s ticker.
	hash, err := invoker.InvokeVoidFunctionSubmit(context.Background(), testContractID, testFunction)
	if err != nil {
		t.Fatalf("InvokeVoidFunctionSubmit: %v", err)
	}
	if hash == "" {
		t.Fatal("expected a transaction hash")
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("no spans recorded for a Soroban invocation")
	}

	// The RPC round trips must each be visible.
	var sawSimulate, sawSend bool
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if attr.Key == attrRPCMethod {
				switch attr.Value.AsString() {
				case "simulateTransaction":
					sawSimulate = true
				case "sendTransaction":
					sawSend = true
				}
			}
		}
	}
	if !sawSimulate {
		t.Error("no span for simulateTransaction")
	}
	if !sawSend {
		t.Error("no span for sendTransaction")
	}

	assertNoSecretsInSpans(t, spans, operatorSecret, invoker.kp.Address())
}

// The contract-level span must carry the low-cardinality metadata an operator
// groups by, and must still be free of secrets on the failure path.
func TestSimulateVoidFunctionRecordsContractMetadata(t *testing.T) {
	exporter := newInvokerSpanRecorder(t)
	rpc := fakeSorobanRPC(t)
	defer rpc.Close()

	operatorSecret := newTestOperatorSecret(t)
	invoker, err := NewContractInvoker(rpc.URL, rpc.URL, "Test SDF Network ; September 2015", operatorSecret)
	if err != nil {
		t.Fatalf("NewContractInvoker: %v", err)
	}

	if err := invoker.SimulateVoidFunction(context.Background(), testContractID, testFunction); err != nil {
		t.Fatalf("SimulateVoidFunction: %v", err)
	}

	spans := exporter.GetSpans()

	var contractSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "soroban.simulate" {
			contractSpan = &spans[i]
		}
	}
	if contractSpan == nil {
		t.Fatal("no soroban.simulate span recorded")
	}

	attrs := map[string]string{}
	for _, attr := range contractSpan.Attributes {
		attrs[string(attr.Key)] = attr.Value.AsString()
	}

	if got := attrs[string(attrContractID)]; got != testContractID {
		t.Errorf("%s = %q, want %q", attrContractID, got, testContractID)
	}
	if got := attrs[string(attrFunction)]; got != testFunction {
		t.Errorf("%s = %q, want %q", attrFunction, got, testFunction)
	}

	assertNoSecretsInSpans(t, spans, operatorSecret, invoker.kp.Address())
}

// An RPC failure must mark the span errored and retained, without the error
// message carrying transaction or key material.
func TestRPCFailureIsRecordedWithoutSecrets(t *testing.T) {
	exporter := newInvokerSpanRecorder(t)

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/accounts/") {
			_, _ = w.Write([]byte(`{"sequence":"1"}`))
			return
		}
		// A simulation failure that echoes XDR back in the message, which is
		// exactly the shape that must not reach a span verbatim.
		_, _ = w.Write([]byte(`{"result":{"error":"simulation failed: AAAAAQAAAAEAAAAHc2VjcmV0eA=="}}`))
	}))
	defer failing.Close()

	operatorSecret := newTestOperatorSecret(t)
	invoker, err := NewContractInvoker(failing.URL, failing.URL, "Test SDF Network ; September 2015", operatorSecret)
	if err != nil {
		t.Fatalf("NewContractInvoker: %v", err)
	}

	if err := invoker.SimulateVoidFunction(context.Background(), testContractID, testFunction); err == nil {
		t.Fatal("expected the simulation to fail")
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("no spans recorded")
	}

	var retained bool
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if attr.Key == telemetry.RetentionAttributeKey && attr.Value.AsBool() {
				retained = true
			}
		}
	}
	if !retained {
		t.Error("failed Soroban call was not marked for retention")
	}

	assertNoSecretsInSpans(t, spans, operatorSecret, invoker.kp.Address())
}
