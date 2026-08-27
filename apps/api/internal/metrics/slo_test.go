package metrics

import (
	"testing"
	"time"
)

// A nil *Metrics must be safe on every recording path.
//
// The vault service holds an optional metrics pointer, so a service
// constructed without one — tests, tooling, or a wiring mistake — must behave
// exactly as before rather than panicking part-way through a deposit. Losing
// observability must never be able to lose a user's money, and this is the
// test that keeps that true as recording paths are added.
func TestNilMetricsIsSafeOnEveryPath(t *testing.T) {
	var m *Metrics

	m.RecordFlowAttempt(FlowDeposit, OutcomeSucceeded, time.Second)
	m.RecordFlowAttempt(FlowWithdrawal, OutcomeFailedChain, 0)
	m.SetIndexerLag(42)
	m.SetIndexerLagSampleAge(time.Minute)
	m.RecordIndexerLagSampleError()
}

func TestFlowAttemptRecordsCounterAndHistogram(t *testing.T) {
	m := New()

	m.RecordFlowAttempt(FlowDeposit, OutcomeSucceeded, 6*time.Second)

	labels := map[string]string{"flow": "deposit", "outcome": "succeeded"}

	if got := counterValue(t, m.Registry(), "nester_flow_attempts_total", labels); got != 1 {
		t.Fatalf("attempts counter = %v, want 1", got)
	}
	if got := histogramCount(t, m.Registry(), "nester_flow_duration_seconds", labels); got != 1 {
		t.Fatalf("duration observations = %d, want 1", got)
	}
}

// A rejected attempt is counted but never timed.
//
// It terminates in microseconds without touching the chain, so including it in
// the latency histogram would drag every percentile toward zero and let the
// latency SLI report health during a chain stall — the SLI would look best
// exactly when the service was refusing everything.
func TestRejectedAttemptsAreCountedButNotTimed(t *testing.T) {
	m := New()

	m.RecordFlowAttempt(FlowDeposit, OutcomeRejected, 5*time.Second)

	labels := map[string]string{"flow": "deposit", "outcome": "rejected"}

	if got := counterValue(t, m.Registry(), "nester_flow_attempts_total", labels); got != 1 {
		t.Fatalf("rejected attempt was not counted: got %v, want 1", got)
	}
	if got := histogramCount(t, m.Registry(), "nester_flow_duration_seconds", labels); got != 0 {
		t.Fatalf("rejected attempt was timed: got %d observations, want 0", got)
	}
}

// A zero or negative duration is never observed. A caller that could not
// determine a start time must not be able to plant a 0s sample that makes the
// latency SLI look perfect.
func TestZeroDurationIsNotObserved(t *testing.T) {
	m := New()

	m.RecordFlowAttempt(FlowWithdrawal, OutcomeSucceeded, 0)

	labels := map[string]string{"flow": "withdrawal", "outcome": "succeeded"}

	if got := counterValue(t, m.Registry(), "nester_flow_attempts_total", labels); got != 1 {
		t.Fatalf("attempt was not counted: got %v, want 1", got)
	}
	if got := histogramCount(t, m.Registry(), "nester_flow_duration_seconds", labels); got != 0 {
		t.Fatalf("zero duration was observed: got %d, want 0", got)
	}
}

// Failures are timed as well as counted: how long a failure took to surface is
// operationally different from how long a success took, and the runbook reads
// both.
func TestFailedAttemptsAreTimed(t *testing.T) {
	m := New()

	m.RecordFlowAttempt(FlowDeposit, OutcomeFailedChain, 45*time.Second)

	labels := map[string]string{"flow": "deposit", "outcome": "failed_chain"}

	if got := histogramCount(t, m.Registry(), "nester_flow_duration_seconds", labels); got != 1 {
		t.Fatalf("failed attempt was not timed: got %d, want 1", got)
	}
}

// The label set is fixed at compile time. This asserts the series count cannot
// be moved by traffic or by a caller: five outcomes across two flows is the
// entire space, and it is reached only by outcomes the code actually emits.
func TestFlowLabelCardinalityIsBounded(t *testing.T) {
	m := New()

	flows := []Flow{FlowDeposit, FlowWithdrawal}
	outcomes := []FlowOutcome{
		OutcomeSucceeded,
		OutcomeRejected,
		OutcomeCancelled,
		OutcomeFailedChain,
		OutcomeFailedInternal,
	}

	for _, flow := range flows {
		for _, outcome := range outcomes {
			m.RecordFlowAttempt(flow, outcome, time.Second)
		}
	}

	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	for _, family := range families {
		if family.GetName() != "nester_flow_attempts_total" {
			continue
		}
		if got := len(family.GetMetric()); got != len(flows)*len(outcomes) {
			t.Fatalf("attempts series = %d, want %d", got, len(flows)*len(outcomes))
		}
		return
	}

	t.Fatal("nester_flow_attempts_total not found in registry")
}

func TestIndexerLagGauges(t *testing.T) {
	m := New()

	m.SetIndexerLag(37)
	if got := gaugeValue(t, m.Registry(), "nester_indexer_lag_ledgers"); got != 37 {
		t.Fatalf("lag gauge = %v, want 37", got)
	}

	// A successful sample resets the staleness gauge: the reading is current.
	if got := gaugeValue(t, m.Registry(), "nester_indexer_lag_last_sample_age_seconds"); got != 0 {
		t.Fatalf("sample age after a successful sample = %v, want 0", got)
	}

	m.SetIndexerLagSampleAge(90 * time.Second)
	if got := gaugeValue(t, m.Registry(), "nester_indexer_lag_last_sample_age_seconds"); got != 90 {
		t.Fatalf("sample age = %v, want 90", got)
	}
}

// The staleness gauge is what distinguishes "lag is genuinely low" from "the
// sampler died and the value is frozen at a healthy number". Without it the
// balance-freshness SLI fails in its most dangerous direction: reporting
// perfect health. This asserts the lag value and its age are independent, so
// an ageing sample cannot be masked by a stale-but-low lag reading.
func TestStalenessIsIndependentOfLagValue(t *testing.T) {
	m := New()

	m.SetIndexerLag(3)
	m.SetIndexerLagSampleAge(600 * time.Second)

	if got := gaugeValue(t, m.Registry(), "nester_indexer_lag_ledgers"); got != 3 {
		t.Fatalf("lag gauge = %v, want 3 (a healthy-looking value)", got)
	}
	if got := gaugeValue(t, m.Registry(), "nester_indexer_lag_last_sample_age_seconds"); got != 600 {
		t.Fatalf("sample age = %v, want 600 (stale despite the healthy lag)", got)
	}
}

func TestIndexerLagSampleErrorsAreCounted(t *testing.T) {
	m := New()

	m.RecordIndexerLagSampleError()
	m.RecordIndexerLagSampleError()

	if got := counterValue(t, m.Registry(), "nester_indexer_lag_sample_errors_total", nil); got != 2 {
		t.Fatalf("sample errors = %v, want 2", got)
	}
}
