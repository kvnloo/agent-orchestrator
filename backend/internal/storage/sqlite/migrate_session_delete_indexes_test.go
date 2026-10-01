package sqlite

import (
	"database/sql"
	"strings"
	"testing"
)

func TestSessionDeleteIndexesAvoidLargeSessionScans(t *testing.T) {
	db := openMigratedDatabaseCopy(t, 170)
	if err := migrate(db); err != nil {
		t.Fatalf("apply session-delete indexes: %v", err)
	}

	assertQueryUsesIndex(
		t,
		db,
		"idx_conversation_provider_events_session",
		"SELECT id FROM conversation_provider_events WHERE session_id = 'seed-session'",
	)
	assertQueryUsesIndex(
		t,
		db,
		"idx_change_log_session",
		"DELETE FROM change_log WHERE session_id = 'seed-session'",
	)
}

func assertQueryUsesIndex(t *testing.T, db *sql.DB, indexName, statement string) {
	t.Helper()
	rows, err := db.Query("EXPLAIN QUERY PLAN " + statement)
	if err != nil {
		t.Fatalf("explain %s: %v", indexName, err)
	}
	defer func() { _ = rows.Close() }()

	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan for %s: %v", indexName, err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read plan for %s: %v", indexName, err)
	}
	plan := strings.Join(details, "\n")
	if !strings.Contains(plan, indexName) {
		t.Fatalf("query plan does not use %s:\n%s", indexName, plan)
	}
}
