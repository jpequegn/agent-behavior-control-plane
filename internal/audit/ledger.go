package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
	_ "modernc.org/sqlite"
)

var ErrDuplicateDecision = errors.New("decision is already recorded")

type Ledger struct {
	db *sql.DB
}

func Open(path string) (*Ledger, error) {
	dsn := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		dsn = "file:" + path
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open audit ledger: %w", err)
	}
	db.SetMaxOpenConns(1)
	ledger := &Ledger{db: db}
	if err := ledger.initialize(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return ledger, nil
}

func (l *Ledger) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}

func (l *Ledger) Append(ctx context.Context, event control.AuditEvent) error {
	if l == nil || l.db == nil {
		return errors.New("audit ledger is unavailable")
	}
	if err := event.Decision.Validate(); err != nil {
		return fmt.Errorf("validate audit decision: %w", err)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	_, err = l.db.ExecContext(ctx, `
		INSERT INTO audit_events (
			decision_id, correlation_id, run_id, decision, policy_digest, flag_snapshot,
			reason_codes, event_json, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.Decision.DecisionID, event.CorrelationID, event.RunID, event.Decision.Decision,
		event.Decision.PolicyDigest, event.Decision.FlagSnapshot,
		encodeReasonCodes(event.Decision.ReasonCodes), string(payload), event.Decision.RecordedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: audit_events.decision_id") {
			return ErrDuplicateDecision
		}
		return fmt.Errorf("append audit event: %w", err)
	}
	return nil
}

func (l *Ledger) Get(ctx context.Context, decisionID string) (control.AuditEvent, error) {
	if l == nil || l.db == nil {
		return control.AuditEvent{}, errors.New("audit ledger is unavailable")
	}
	var payload string
	err := l.db.QueryRowContext(ctx, `SELECT event_json FROM audit_events WHERE decision_id = ?`, decisionID).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return control.AuditEvent{}, fmt.Errorf("audit decision %q not found", decisionID)
		}
		return control.AuditEvent{}, fmt.Errorf("read audit event: %w", err)
	}
	var event control.AuditEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return control.AuditEvent{}, fmt.Errorf("decode audit event: %w", err)
	}
	return event, nil
}

func (l *Ledger) initialize(ctx context.Context) error {
	_, err := l.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit_events (
			decision_id TEXT PRIMARY KEY,
			correlation_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			decision TEXT NOT NULL,
			policy_digest TEXT NOT NULL,
			flag_snapshot TEXT NOT NULL,
			reason_codes TEXT NOT NULL,
			event_json TEXT NOT NULL,
			recorded_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_audit_events_run_id ON audit_events(run_id);
		CREATE TRIGGER IF NOT EXISTS audit_events_no_update
		BEFORE UPDATE ON audit_events
		BEGIN
			SELECT RAISE(ABORT, 'audit events are append-only');
		END;
		CREATE TRIGGER IF NOT EXISTS audit_events_no_delete
		BEFORE DELETE ON audit_events
		BEGIN
			SELECT RAISE(ABORT, 'audit events are append-only');
		END;
	`)
	if err != nil {
		return fmt.Errorf("initialize audit ledger: %w", err)
	}
	return nil
}

func encodeReasonCodes(reasons []string) string {
	encoded, _ := json.Marshal(reasons)
	return string(encoded)
}
