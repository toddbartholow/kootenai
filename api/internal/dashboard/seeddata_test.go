package dashboard_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// To seed:   SEED_DASHBOARD=1 go test -v -run TestSeedDashboardData -count=1
// To remove: SEED_DASHBOARD=1 go test -v -run TestRemoveDashboardData -count=1
//
// The SEED_DASHBOARD env var is a safety guard so this never runs accidentally.

func skipUnlessSeed(t *testing.T) {
	t.Helper()
	if os.Getenv("SEED_DASHBOARD") == "" {
		t.Skip("skipping: set SEED_DASHBOARD=1 to run this test")
	}
}

const (
	// Seed for both the demo user and the real user so the dashboard
	// works regardless of which login mode is active.
	demoUserID = "00000000-0000-0000-0000-000000000001" // demo user (AUTH_DEMO_MODE)
	realUserID = "5573dd28-60d7-4d4c-82e9-ceaef280a307" // real (non-demo) user
	podID      = "eb4d134b-4592-45ce-bcab-08a79cb608c8" // existing pod

	// Marker so we can identify and cleanly remove seeded data.
	// Values must be strings — the Go model uses map[string]string.
	metadataMarker = `{"seeded": "true", "source": "dashboard_seeddata_test"}`
)

func connectDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db
}

// labPool holds templates we'll generate sessions against.
type labInfo struct {
	ID        string
	MaxPoints int
}

func loadLabs(t *testing.T, db *sql.DB) []labInfo {
	t.Helper()
	rows, err := db.Query(`
		SELECT id, max_points FROM lab_templates
		WHERE is_active = true AND tags IS NOT NULL AND array_length(tags, 1) > 0
		ORDER BY random()`)
	if err != nil {
		t.Fatalf("load labs: %v", err)
	}
	defer rows.Close()
	var labs []labInfo
	for rows.Next() {
		var l labInfo
		if err := rows.Scan(&l.ID, &l.MaxPoints); err != nil {
			t.Fatalf("scan lab: %v", err)
		}
		labs = append(labs, l)
	}
	if len(labs) == 0 {
		t.Fatal("no lab templates with tags found")
	}
	return labs
}

// TestSeedDashboardData inserts ~60 realistic sessions spread across the last
// 90 days so every dashboard panel has data to render.
func TestSeedDashboardData(t *testing.T) {
	skipUnlessSeed(t)

	db := connectDB(t)
	defer db.Close()

	labs := loadLabs(t, db)

	rng := rand.New(rand.NewSource(42)) // deterministic

	// Generate sessions spread across 90 days. Cluster more sessions in
	// recent weeks and leave some days empty for a natural-looking streak.
	now := time.Now()
	type session struct {
		labIdx    int
		startedAt time.Time
		durMins   int
		score     float64 // 0-1
		passed    bool
	}

	var sessions []session

	for day := 89; day >= 0; day-- {
		// Probability of activity increases toward present
		activityChance := 0.35 + 0.45*float64(90-day)/90.0
		if rng.Float64() > activityChance {
			continue // rest day
		}

		// 1-3 sessions per active day
		count := 1 + rng.Intn(3)
		for s := 0; s < count; s++ {
			labIdx := rng.Intn(len(labs))
			dur := 20 + rng.Intn(80)         // 20-100 min
			score := 0.4 + rng.Float64()*0.6 // 40-100%
			passed := score >= 0.5           // pass threshold ~50%
			hour := 8 + rng.Intn(12)         // 8am - 8pm
			startedAt := time.Date(now.Year(), now.Month(), now.Day()-day,
				hour, rng.Intn(60), 0, 0, now.Location())

			sessions = append(sessions, session{
				labIdx:    labIdx,
				startedAt: startedAt,
				durMins:   dur,
				score:     score,
				passed:    passed,
			})
		}
	}

	userIDs := []string{demoUserID, realUserID}
	t.Logf("Inserting %d sessions x %d users", len(sessions), len(userIDs))

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT INTO lab_sessions
			(pod_id, user_id, lab_template_id, started_at, ended_at,
			 max_points, earned_points, percentage, passed, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)`)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, uid := range userIDs {
		for _, s := range sessions {
			lab := labs[s.labIdx]
			earned := int(float64(lab.MaxPoints) * s.score)
			if earned > lab.MaxPoints {
				earned = lab.MaxPoints
			}
			pct := float64(earned) / float64(lab.MaxPoints) * 100
			endedAt := s.startedAt.Add(time.Duration(s.durMins) * time.Minute)

			_, err := stmt.Exec(
				podID, uid, lab.ID,
				s.startedAt, endedAt,
				lab.MaxPoints, earned, fmt.Sprintf("%.2f", pct),
				s.passed, metadataMarker,
			)
			if err != nil {
				t.Fatalf("insert session: %v", err)
			}
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	t.Logf("Seeded %d sessions successfully", inserted)

	// Verify
	var count int
	err = db.QueryRow(`SELECT count(*) FROM lab_sessions WHERE metadata @> '{"seeded": "true"}'`).Scan(&count)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	t.Logf("Verified: %d seeded sessions in database", count)
}

// TestRemoveDashboardData deletes all seeded sessions.
func TestRemoveDashboardData(t *testing.T) {
	skipUnlessSeed(t)

	db := connectDB(t)
	defer db.Close()

	// Delete checkpoint progress for seeded sessions first (FK)
	res, err := db.Exec(`
		DELETE FROM checkpoint_progress
		WHERE session_id IN (
			SELECT id FROM lab_sessions
			WHERE metadata @> '{"seeded": "true"}'
		)`)
	if err != nil {
		t.Fatalf("delete checkpoints: %v", err)
	}
	cpCount, _ := res.RowsAffected()

	// Delete the seeded sessions (both demo and real user)
	res, err = db.Exec(`
		DELETE FROM lab_sessions
		WHERE metadata @> '{"seeded": "true"}'`)
	if err != nil {
		t.Fatalf("delete sessions: %v", err)
	}
	count, _ := res.RowsAffected()

	t.Logf("Removed %d seeded sessions and %d checkpoint records", count, cpCount)

	// Verify clean
	var remaining int
	err = db.QueryRow(`SELECT count(*) FROM lab_sessions WHERE metadata @> '{"seeded": "true"}'`).Scan(&remaining)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining seeded sessions, got %d", remaining)
	}
	t.Log("Cleanup verified: all seeded data removed")
}
