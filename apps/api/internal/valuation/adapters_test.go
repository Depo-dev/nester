package valuation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/suncrestlabs/nester/apps/api/internal/domain/portfolio"
)

// captureHandler is a minimal slog.Handler that records emitted records as
// plain strings for assertion, mirroring the one in
// reconciliation/engine_test.go.
type captureHandler struct {
	records *[]string
}

func (h captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h captureHandler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder
	sb.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		sb.WriteString(fmt.Sprintf(" %s=%v", a.Key, a.Value))
		return true
	})
	*h.records = append(*h.records, sb.String())
	return nil
}
func (h captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h captureHandler) WithGroup(string) slog.Handler      { return h }

type erroringUserPusher struct{ err error }

func (e erroringUserPusher) PushToUser(context.Context, uuid.UUID, string, any) error {
	return e.err
}

func TestWSNotifier_PushValuation_LogsFailurePushError(t *testing.T) {
	pushErr := errors.New("hub unavailable")
	var logs []string
	logger := slog.New(captureHandler{records: &logs})
	notifier := NewWSNotifier(erroringUserPusher{err: pushErr}, logger)

	notifier.PushValuation(uuid.New(), portfolio.Valuation{})

	if len(logs) != 1 {
		t.Fatalf("expected exactly one log record, got %d: %v", len(logs), logs)
	}
	if !strings.Contains(logs[0], pushErr.Error()) {
		t.Fatalf("log entry missing the push error: %q", logs[0])
	}
}

type noopUserPusher struct{}

func (noopUserPusher) PushToUser(context.Context, uuid.UUID, string, any) error { return nil }

func TestWSNotifier_PushValuation_NoLogOnSuccess(t *testing.T) {
	var logs []string
	logger := slog.New(captureHandler{records: &logs})
	notifier := NewWSNotifier(noopUserPusher{}, logger)

	notifier.PushValuation(uuid.New(), portfolio.Valuation{})

	if len(logs) != 0 {
		t.Fatalf("expected no log records on a successful push, got %d: %v", len(logs), logs)
	}
}

func TestNewWSNotifier_NilLoggerDoesNotPanic(t *testing.T) {
	notifier := NewWSNotifier(noopUserPusher{}, nil)
	notifier.PushValuation(uuid.New(), portfolio.Valuation{})
}
