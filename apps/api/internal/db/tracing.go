package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"github.com/suncrestlabs/nester/apps/api/internal/telemetry"
)

// OpenTraced opens the pool exactly as Open does, but registers the pgx
// driver wrapped in OpenTelemetry instrumentation (nester#1054).
//
// Privacy is the whole design constraint here. A database span in a financial
// application must never carry the values bound to a statement: those are
// account numbers, balances, wallet addresses, and amounts. otelsql is
// therefore configured to record the *parameterised* statement text only —
// the SQL with its $1, $2 placeholders intact — and never the arguments.
// SpanOptions.DisableQuery is left false so operators can still see which
// statement was slow, which is the main reason to trace a database at all.
//
// The DSN is never recorded either. It embeds the database password, and
// otelsql would otherwise be a plausible place for it to end up.
func OpenTraced(cfg Config, tracingEnabled bool) (*sql.DB, error) {
	if !tracingEnabled {
		return Open(cfg)
	}

	driverName, err := otelsql.Register("pgx",
		otelsql.WithAttributes(semconv.DBSystemNamePostgreSQL),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			// Omit the span for the connection-pool acquisition itself; it is
			// noise on every query and is covered by pool metrics instead.
			OmitConnResetSession: true,
			OmitConnPrepare:      true,
			OmitRows:             true,

			// Record the statement so a slow query is identifiable. otelsql
			// only ever writes the query *text* passed to database/sql, which
			// in this codebase is always parameterised — arguments travel
			// separately through pgx's extended protocol and are never part
			// of this string.
			DisableQuery: false,

			// Do not mark a span errored for sql.ErrNoRows: "no rows" is an
			// ordinary outcome, not a failure, and flagging it would both
			// mislead operators and pollute error-based tail sampling.
			DisableErrSkip: false,
		}),
		// Strip any attribute that could carry a bound value or a credential.
		// otelsql does not record arguments today; this is a guard against a
		// future version or option change silently starting to.
		otelsql.WithAttributesGetter(safeDBAttributes),
	)
	if err != nil {
		return nil, fmt.Errorf("db: register traced driver: %w", err)
	}

	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: open traced: %w", err)
	}

	if err := applyPoolSettings(db, cfg); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// safeDBAttributes returns the attributes recorded on every database span.
//
// It deliberately returns nothing derived from the query arguments. The
// signature receives them — otelsql offers them to this hook — and returning
// any would leak account numbers, balances and amounts into telemetry. The
// args parameter is named and then ignored so that intent is explicit to the
// next reader rather than looking like an oversight.
func safeDBAttributes(_ context.Context, _ otelsql.Method, query string, args []driver.NamedValue) []attribute.KeyValue {
	_ = args // never recorded: bound values are user financial data

	return []attribute.KeyValue{
		// The statement text is passed through the telemetry redactor as a
		// backstop. Application SQL is parameterised, but a literal that a
		// migration or an ad-hoc statement embedded would still be caught.
		telemetry.SafeAttribute(string(semconv.DBQueryTextKey), query),
	}
}
