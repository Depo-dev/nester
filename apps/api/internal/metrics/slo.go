package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// This file defines the service level indicators for nester#1056. The metrics
// in metrics.go describe how the process behaves; these describe whether the
// product worked for a user, which is what an SLO is written against.
//
// The cardinality policy in the package doc applies here without exception,
// and the financial domain makes it sharper: an amount, a vault ID, a wallet
// address, or a transaction hash is never a label. Every label below is a
// closed Go constant set, so the series count is fixed at compile time and
// cannot be moved by traffic or by a caller.

// flowDurationBuckets covers the settlement path for a deposit or withdrawal,
// which is dominated by Soroban ledger close time rather than by anything the
// API does. Ledgers close at roughly 5s intervals, so a single-ledger
// confirmation lands near 5-6s and a two-ledger confirmation near 11s. The
// buckets are placed to resolve that band: 2.5s and 5s bracket the
// single-ledger case, 10s and 15s the two-and-three-ledger cases, and 30s/60s
// separate "slow" from "the chain or the invoker is stuck". Reusing
// requestDurationBuckets here would put half the resolution below 250ms,
// where no confirmed deposit can ever land.
var flowDurationBuckets = []float64{
	1, 2.5, 5, 7.5, 10, 15, 20, 30, 45, 60, 120,
}

// Flow identifies a user-visible financial flow.
//
// Deposits and withdrawals get separate SLOs rather than one "transaction"
// SLO because their failure modes and their user impact differ: a failed
// deposit is money that did not start earning, a failed withdrawal is money
// the user cannot reach, and the second is materially worse. Averaging them
// into one indicator hides a withdrawal outage behind healthy deposit volume.
type Flow string

const (
	FlowDeposit    Flow = "deposit"
	FlowWithdrawal Flow = "withdrawal"
)

// FlowOutcome is the terminal classification of one flow attempt.
//
// The split between rejected, cancelled, and the two failure kinds is the
// whole substance of the deposit and withdrawal SLIs, so each is defined
// against the code that produces it:
//
//   - OutcomeSucceeded: the chain call returned without error and the ledger
//     row was written. Both halves are required; a chain success whose
//     database write failed is not a success, because the user's balance does
//     not reflect their money.
//
//   - OutcomeRejected: the request never reached the chain because the
//     service correctly refused it — zero or negative amount, excess decimal
//     scale, closed vault, insufficient balance, unknown vault. These are the
//     service working as designed against an invalid request, so they are
//     excluded from the SLI denominator. Excluding them is what stops a
//     client looping an invalid request from manufacturing an SLO breach.
//
//   - OutcomeCancelled: the user declined the wallet signature, or abandoned
//     the attempt before submission. Excluded from the denominator on the
//     issue's instruction, and separately counted so that a wave of
//     cancellations caused by a broken signing prompt is still visible rather
//     than silently dropped.
//
//   - OutcomeFailedChain: the Soroban invocation returned an error, timed
//     out, or the transaction failed on-chain. Counted as a failure. This
//     includes upstream RPC problems: from the user's position "the network
//     was down" and "we could not reach the network" are the same event, and
//     an SLI that excused infrastructure it does not own would report health
//     during a total outage.
//
//   - OutcomeFailedInternal: the API failed on its own side — database write
//     failure, panic, unhandled path. Always a failure.
//
// A contract-level rejection that is really a user error rather than a system
// fault is the one genuinely ambiguous case; ErrBelowMinDeposit is mapped to
// OutcomeRejected because the contract refused a request that the API should
// have caught, and counting it as a chain failure would let a UI bug that
// permits sub-minimum deposits burn the deposit error budget.
type FlowOutcome string

const (
	OutcomeSucceeded      FlowOutcome = "succeeded"
	OutcomeRejected       FlowOutcome = "rejected"
	OutcomeCancelled      FlowOutcome = "cancelled"
	OutcomeFailedChain    FlowOutcome = "failed_chain"
	OutcomeFailedInternal FlowOutcome = "failed_internal"
)

// sloCollectors holds the SLI instrumentation. It is embedded in Metrics
// rather than living in a second registry so that one scrape carries both the
// infrastructure and the product view; correlating them across two endpoints
// during an incident is exactly the friction a runbook cannot afford.
type sloCollectors struct {
	flowAttemptsTotal *prometheus.CounterVec
	flowDuration      *prometheus.HistogramVec

	indexerLagLedgers      prometheus.Gauge
	indexerLagStaleness    prometheus.Gauge
	indexerLagScrapeErrors prometheus.Counter
}

func newSLOCollectors() *sloCollectors {
	return &sloCollectors{
		// The SLI counter. Success rate is
		//   succeeded / (succeeded + failed_chain + failed_internal)
		// with rejected and cancelled excluded from both halves. Keeping all
		// five outcomes on one counter rather than splitting successes and
		// failures into separate metrics means the denominator is always
		// derivable from a single series selector, and an outcome that is
		// added later cannot silently fall out of the denominator.
		flowAttemptsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "flow",
			Name:      "attempts_total",
			Help:      "Terminal outcomes of deposit and withdrawal attempts, by flow and outcome.",
		}, []string{"flow", "outcome"}),

		// Latency is observed only for attempts that reached a terminal
		// state on-chain, and the outcome label is retained because a
		// failure that takes 45s to surface and a success that takes 6s are
		// different operational events. Rejected attempts are never observed
		// here: they terminate in microseconds without touching the chain,
		// and including them would drag every percentile toward zero and
		// make the latency SLI report health during a chain stall.
		flowDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: "flow",
			Name:      "duration_seconds",
			Help:      "End-to-end latency of deposit and withdrawal attempts that reached the chain, by flow and outcome.",
			Buckets:   flowDurationBuckets,
		}, []string{"flow", "outcome"}),

		// Balance freshness. Ledgers rather than seconds because the
		// indexer's own unit is the ledger sequence, and converting to time
		// would bake in an assumed close interval that varies.
		indexerLagLedgers: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "indexer",
			Name:      "lag_ledgers",
			Help:      "Network ledger tip minus last successfully indexed ledger.",
		}),

		// A lag gauge alone cannot distinguish "lag is 0" from "the sampler
		// died and the value is frozen at its last write". This gauge is the
		// age of the lag reading, so an alert can require freshness of the
		// freshness signal. Without it the balance SLI fails silently in the
		// most dangerous direction: reporting perfect health.
		indexerLagStaleness: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "indexer",
			Name:      "lag_last_sample_age_seconds",
			Help:      "Seconds since the indexer lag gauge was last successfully updated.",
		}),

		// Sampling errors are counted rather than folded into the lag value,
		// because writing a sentinel lag on error would be indistinguishable
		// from a real stall and would page for the wrong reason.
		indexerLagScrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "indexer",
			Name:      "lag_sample_errors_total",
			Help:      "Failed attempts to sample indexer lag.",
		}),
	}
}

func (c *sloCollectors) collectors() []prometheus.Collector {
	return []prometheus.Collector{
		c.flowAttemptsTotal,
		c.flowDuration,
		c.indexerLagLedgers,
		c.indexerLagStaleness,
		c.indexerLagScrapeErrors,
	}
}

// RecordFlowAttempt records one terminal deposit or withdrawal outcome.
//
// duration is the time from the service accepting the request to the terminal
// outcome. It is observed only when the attempt reached the chain: a rejected
// attempt passes a non-positive duration and is counted but not timed.
//
// Callers must call this exactly once per attempt, on every return path. The
// helper in the service layer wraps it in a defer so a newly added early
// return cannot silently drop an attempt from the denominator, which would
// inflate the reported success rate — the one direction of error an SLI must
// never make.
func (m *Metrics) RecordFlowAttempt(flow Flow, outcome FlowOutcome, duration time.Duration) {
	if m == nil {
		return
	}

	m.slo.flowAttemptsTotal.WithLabelValues(string(flow), string(outcome)).Inc()

	if duration > 0 && outcome != OutcomeRejected {
		m.slo.flowDuration.WithLabelValues(string(flow), string(outcome)).Observe(duration.Seconds())
	}
}

// SetIndexerLag publishes the current indexer lag and resets the staleness
// gauge, which the sampler's own ticker ages between calls.
func (m *Metrics) SetIndexerLag(lagLedgers uint64) {
	if m == nil {
		return
	}

	m.slo.indexerLagLedgers.Set(float64(lagLedgers))
	m.slo.indexerLagStaleness.Set(0)
}

// SetIndexerLagSampleAge publishes how old the lag reading is.
func (m *Metrics) SetIndexerLagSampleAge(age time.Duration) {
	if m == nil {
		return
	}

	m.slo.indexerLagStaleness.Set(age.Seconds())
}

// RecordIndexerLagSampleError counts a failed lag sample.
func (m *Metrics) RecordIndexerLagSampleError() {
	if m == nil {
		return
	}

	m.slo.indexerLagScrapeErrors.Inc()
}
