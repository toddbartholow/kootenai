package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// SessionCommands handles session management commands
type SessionCommands struct {
	client *Client
}

// NewSessionCommands creates session command handler
func NewSessionCommands(client *Client) *SessionCommands {
	return &SessionCommands{client: client}
}

// SessionListResponse represents the API response for listing sessions
type SessionListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

// SessionInfo represents session information from API
type SessionInfo struct {
	ID           string     `json:"id"`
	PodID        string     `json:"podId"`
	UserID       string     `json:"userId"`
	LabTemplate  string     `json:"labTemplate"`
	EarnedPoints int        `json:"earnedPoints"`
	MaxPoints    int        `json:"maxPoints"`
	Percentage   float64    `json:"percentage"`
	Passed       bool       `json:"passed"`
	StartedAt    time.Time  `json:"startedAt"`
	EndedAt      *time.Time `json:"endedAt,omitempty"`
}

// ProgressResponse represents the API response for session progress
type ProgressResponse struct {
	SessionID    string                   `json:"sessionId"`
	EarnedPoints int                      `json:"earnedPoints"`
	MaxPoints    int                      `json:"maxPoints"`
	Percentage   float64                  `json:"percentage"`
	Checkpoints  []models.CheckpointState `json:"checkpoints"`
}

// List lists all sessions
func (s *SessionCommands) List(ctx context.Context, userID string, active bool) error {
	path := "/api/v1/sessions"
	if userID != "" {
		path += "?userId=" + userID
	}

	var response SessionListResponse
	if err := s.client.Get(ctx, path, &response); err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	sessions := response.Sessions
	if active {
		var filtered []SessionInfo
		for _, sess := range sessions {
			if sess.EndedAt == nil {
				filtered = append(filtered, sess)
			}
		}
		sessions = filtered
	}

	if len(sessions) == 0 {
		fmt.Println(T("labctl.session.list.empty", nil))
		return nil
	}

	headers := []string{
		T("labctl.session.list.colId", nil),
		T("labctl.session.list.colUser", nil),
		T("labctl.session.list.colTemplate", nil),
		T("labctl.session.list.colProgress", nil),
		T("labctl.session.list.colStatus", nil),
		T("labctl.session.list.colStarted", nil),
		T("labctl.session.list.colDuration", nil),
	}
	dashes := make([]string, len(headers))
	for i, h := range headers {
		dashes[i] = strings.Repeat("-", runewidth(h))
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(dashes, "\t"))

	for _, sess := range sessions {
		status := T("labctl.session.list.statusActive", nil)
		duration := time.Since(sess.StartedAt)
		if sess.EndedAt != nil {
			status = T("labctl.session.list.statusEnded", nil)
			duration = sess.EndedAt.Sub(sess.StartedAt)
		}
		if sess.Passed {
			status = T("labctl.session.list.statusPassed", nil)
		}

		progress := T("labctl.session.list.progressFormat", map[string]any{
			"Earned": sess.EarnedPoints,
			"Max":    sess.MaxPoints,
			"Pct":    fmt.Sprintf("%.0f", sess.Percentage),
		})

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			truncateID(sess.ID),
			sess.UserID,
			sess.LabTemplate,
			progress,
			status,
			sess.StartedAt.Format("Jan 02 15:04"),
			formatDuration(duration),
		)
	}
	w.Flush()

	return nil
}

// Start starts a new session
func (s *SessionCommands) Start(ctx context.Context, podID, userID, templateName string) error {
	request := map[string]string{
		"podId":       podID,
		"userId":      userID,
		"labTemplate": templateName,
	}

	var response struct {
		SessionID string `json:"sessionId"`
		Status    string `json:"status"`
	}

	if err := s.client.Post(ctx, "/api/v1/sessions", request, &response); err != nil {
		return fmt.Errorf("starting session: %w", err)
	}

	fmt.Println(T("labctl.session.start.success", nil))
	fmt.Println(T("labctl.session.start.idLine", map[string]any{"Id": response.SessionID}))
	fmt.Println(T("labctl.session.start.statusLine", map[string]any{"Status": response.Status}))

	return nil
}

// Status shows detailed session status
func (s *SessionCommands) Status(ctx context.Context, sessionID string) error {
	var progress ProgressResponse
	if err := s.client.Get(ctx, "/api/v1/sessions/"+sessionID, &progress); err != nil {
		return fmt.Errorf("getting session status: %w", err)
	}

	fmt.Println(T("labctl.session.status.heading", map[string]any{"Id": progress.SessionID}))
	fmt.Printf("================================================================================\n\n")

	// Progress bar — the █/░ glyphs are non-textual and stay verbatim.
	barWidth := 50
	filled := int(progress.Percentage / 100 * float64(barWidth))
	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	fmt.Println(T("labctl.session.status.progressLine", map[string]any{
		"Bar": bar, "Pct": fmt.Sprintf("%.1f", progress.Percentage),
	}))
	fmt.Println(T("labctl.session.status.pointsLine", map[string]any{
		"Earned": progress.EarnedPoints, "Max": progress.MaxPoints,
	}))
	fmt.Println()

	if len(progress.Checkpoints) > 0 {
		fmt.Println(T("labctl.session.status.checkpointsHeading", nil))
		for _, cp := range progress.Checkpoints {
			icon := checkpointIcon(cp.Status)
			points := fmt.Sprintf("%d/%d", cp.EarnedPoints, cp.Points)

			timestamp := ""
			if cp.PassedAt != nil {
				timestamp = T("labctl.session.status.completedAt", map[string]any{"When": cp.PassedAt.Format("15:04:05")})
			}

			fmt.Println(T("labctl.session.status.checkpointLine", map[string]any{
				"Icon": icon, "Points": points, "Id": cp.CheckpointID, "Timestamp": timestamp,
			}))
		}
	}

	return nil
}

// Progress shows just the progress bar and points
func (s *SessionCommands) Progress(ctx context.Context, sessionID string) error {
	path := "/api/v1/sessions/" + sessionID + "/progress"

	var progress ProgressResponse
	if err := s.client.Get(ctx, path, &progress); err != nil {
		return fmt.Errorf("getting progress: %w", err)
	}

	// Progress bar
	barWidth := 40
	filled := int(progress.Percentage / 100 * float64(barWidth))
	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	passed := ""
	if progress.Percentage >= 70 {
		// #nosec G101 -- UI status label, not a credential
		passed = T("labctl.session.progress.passedSuffix", nil)
	}

	fmt.Println(T("labctl.session.progress.barLine", map[string]any{
		"Bar": bar, "Pct": fmt.Sprintf("%.1f", progress.Percentage),
		"Earned": progress.EarnedPoints, "Max": progress.MaxPoints, "Passed": passed,
	}))

	// Summary of checkpoints
	pending, completed, failed := 0, 0, 0
	for _, cp := range progress.Checkpoints {
		switch cp.Status {
		case models.CheckpointStatusPassed:
			completed++
		case models.CheckpointStatusFailed:
			failed++
		default:
			pending++
		}
	}

	fmt.Print(T("labctl.session.progress.summary", map[string]any{
		"Completed": completed, "Pending": pending,
	}))
	if failed > 0 {
		fmt.Print(T("labctl.session.progress.failedSuffix", map[string]any{"Count": failed}))
	}
	fmt.Println()

	return nil
}

// End ends a session
func (s *SessionCommands) End(ctx context.Context, sessionID string) error {
	var response struct {
		Status string `json:"status"`
	}

	if err := s.client.Post(ctx, "/api/v1/sessions/"+sessionID+"/end", nil, &response); err != nil {
		return fmt.Errorf("ending session: %w", err)
	}

	fmt.Println(T("labctl.session.end.ended", map[string]any{"Id": truncateID(sessionID)}))
	fmt.Println(T("labctl.session.end.status", map[string]any{"Status": response.Status}))

	return nil
}

// Checkpoints shows checkpoint details
func (s *SessionCommands) Checkpoints(ctx context.Context, sessionID string) error {
	path := "/api/v1/sessions/" + sessionID + "/checkpoints"

	var response struct {
		Checkpoints []models.CheckpointState `json:"checkpoints"`
	}

	if err := s.client.Get(ctx, path, &response); err != nil {
		return fmt.Errorf("getting checkpoints: %w", err)
	}

	if len(response.Checkpoints) == 0 {
		fmt.Println(T("labctl.session.checkpoints.empty", nil))
		return nil
	}

	headers := []string{
		T("labctl.session.checkpoints.colStatus", nil),
		T("labctl.session.checkpoints.colId", nil),
		T("labctl.session.checkpoints.colPoints", nil),
		T("labctl.session.checkpoints.colAttempts", nil),
		T("labctl.session.checkpoints.colCompleted", nil),
	}
	dashes := make([]string, len(headers))
	for i, h := range headers {
		dashes[i] = strings.Repeat("-", runewidth(h))
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(dashes, "\t"))

	placeholder := T("labctl.session.checkpoints.emptyPlaceholder", nil)
	for _, cp := range response.Checkpoints {
		completed := placeholder
		if cp.PassedAt != nil {
			completed = cp.PassedAt.Format("Jan 02 15:04:05")
		}

		points := fmt.Sprintf("%d/%d", cp.EarnedPoints, cp.Points)

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			checkpointIcon(cp.Status),
			cp.CheckpointID,
			points,
			cp.AttemptCount,
			completed,
		)
	}
	w.Flush()

	return nil
}

// checkpointIcon returns an icon for checkpoint status
func checkpointIcon(status models.CheckpointStatus) string {
	switch status {
	case models.CheckpointStatusPassed:
		return "✓"
	case models.CheckpointStatusFailed:
		return "✗"
	case models.CheckpointStatusSkipped:
		return "⊘"
	case models.CheckpointStatusPartial:
		return "◐"
	default:
		return "○"
	}
}
