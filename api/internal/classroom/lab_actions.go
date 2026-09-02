package classroom

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// LabExecutor invokes the labtest simulator subprocess for VM SSH execution.
type LabExecutor struct {
	labtestPath string
	logger      *slog.Logger
}

// NewLabExecutor creates a new lab executor.
// labtestPath is the path to the labtest CLI binary (e.g., "labtest" if on PATH).
func NewLabExecutor(labtestPath string, logger *slog.Logger) *LabExecutor {
	if labtestPath == "" {
		labtestPath = "labtest"
	}
	return &LabExecutor{
		labtestPath: labtestPath,
		logger:      logger,
	}
}

// ExecuteLabResult holds the result of a VM lab execution.
type ExecuteLabResult struct {
	Success           bool     `json:"success"`
	Output            string   `json:"output"`
	ExitCode          int      `json:"exitCode"`
	LabName           string   `json:"labName"`
	PodID             string   `json:"podId"`
	DurationMs        int64    `json:"durationMs"`
	CheckpointsPassed int      `json:"checkpointsPassed"`
	CheckpointsTotal  int      `json:"checkpointsTotal"`
	FailedCheckpoints []string `json:"failedCheckpoints,omitempty"`
	Errors            []string `json:"errors,omitempty"`
}

// ExecuteLab runs the labtest simulator for a specific lab and pod.
// Command: labtest simulate --lab <template> --pod-id <pod> --ssh-user <user>
func (e *LabExecutor) ExecuteLab(ctx context.Context, labTemplate, podID, sshUser string) (*ExecuteLabResult, error) {
	args := []string{
		"simulate",
		"--lab", labTemplate,
		"--pod-id", podID,
		"--ssh-user", sshUser,
	}

	e.logger.Info("Executing lab via labtest",
		"lab", labTemplate,
		"pod", podID,
		"user", sshUser,
	)

	cmd := exec.CommandContext(ctx, e.labtestPath, args...)
	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	durationMs := time.Since(startTime).Milliseconds()

	result := &ExecuteLabResult{
		LabName:    labTemplate,
		PodID:      podID,
		Output:     strings.TrimSpace(string(output)),
		DurationMs: durationMs,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Success = false
		e.logger.Warn("Lab execution failed",
			"lab", labTemplate,
			"pod", podID,
			"exitCode", result.ExitCode,
			"error", err,
		)
		return result, fmt.Errorf("lab execution failed: %w", err)
	}

	result.Success = true
	e.logger.Info("Lab execution completed",
		"lab", labTemplate,
		"pod", podID,
	)

	return result, nil
}
