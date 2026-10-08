package gen

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestPruneChangeLogToMaxRowsPlanHasNoFullScan(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, statement := range []string{
		`CREATE TABLE change_log (seq INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, session_id TEXT, event_type TEXT NOT NULL, payload TEXT NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE INDEX idx_change_log_project ON change_log (project_id, seq)`,
		`CREATE INDEX idx_change_log_created_at_seq ON change_log (created_at, seq)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.Query("EXPLAIN QUERY PLAN "+pruneChangeLogToMaxRows, int64(10_000), int64(100_000))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(plan) == 0 {
		t.Fatal("empty query plan")
	}
	for _, detail := range plan {
		if strings.HasPrefix(detail, "SCAN ") {
			t.Fatalf("prune plan contains a full scan: %q\nfull plan:\n  %s", detail, strings.Join(plan, "\n  "))
		}
	}
}
