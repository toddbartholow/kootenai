package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// SnapshotCommands handles snapshot management commands
type SnapshotCommands struct {
	client *Client
}

// NewSnapshotCommands creates snapshot command handler
func NewSnapshotCommands(client *Client) *SnapshotCommands {
	return &SnapshotCommands{client: client}
}

// SnapshotInfo represents a VM snapshot
type SnapshotInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parent      string `json:"parent,omitempty"`
}

// SnapshotListResponse represents the API response for listing snapshots
type SnapshotListResponse struct {
	PodID     string         `json:"podId"`
	VMName    string         `json:"vmName"`
	Snapshots []SnapshotInfo `json:"snapshots"`
	Count     int            `json:"count"`
}

// List lists all snapshots for a VM in a pod
func (s *SnapshotCommands) List(ctx context.Context, podID, vmName string) error {
	path := fmt.Sprintf("/api/v1/pods/%s/vms/%s/snapshots", podID, vmName)

	var response SnapshotListResponse
	if err := s.client.Get(ctx, path, &response); err != nil {
		return fmt.Errorf("listing snapshots: %w", err)
	}

	if len(response.Snapshots) == 0 {
		fmt.Println(T("labctl.snapshot.list.empty", map[string]any{"Vm": vmName, "Pod": podID}))
		return nil
	}

	fmt.Println(T("labctl.snapshot.list.heading", map[string]any{"Vm": vmName, "Pod": podID}))
	fmt.Println()

	headers := []string{
		T("labctl.snapshot.list.colName", nil),
		T("labctl.snapshot.list.colDescription", nil),
		T("labctl.snapshot.list.colParent", nil),
	}
	dashes := make([]string, len(headers))
	for i, h := range headers {
		dashes[i] = strings.Repeat("-", runewidth(h))
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(dashes, "\t"))

	placeholder := T("labctl.snapshot.list.emptyPlaceholder", nil)
	for _, snap := range response.Snapshots {
		parent := snap.Parent
		if parent == "" {
			parent = placeholder
		}
		desc := snap.Description
		if desc == "" {
			desc = placeholder
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", snap.Name, desc, parent)
	}
	w.Flush()

	fmt.Println()
	fmt.Println(T("labctl.snapshot.list.totalLine", map[string]any{"Count": len(response.Snapshots)}))
	return nil
}

// Create creates a new snapshot for a VM
func (s *SnapshotCommands) Create(ctx context.Context, podID, vmName, snapshotName, description string, includeRAM bool) error {
	path := fmt.Sprintf("/api/v1/pods/%s/vms/%s/snapshots", podID, vmName)

	request := map[string]any{
		"name": snapshotName,
	}
	if description != "" {
		request["description"] = description
	}
	if includeRAM {
		request["includeRam"] = true
	}

	var response struct {
		Status   string `json:"status"`
		PodID    string `json:"podId"`
		VMName   string `json:"vmName"`
		Snapshot string `json:"snapshot"`
	}

	if err := s.client.Post(ctx, path, request, &response); err != nil {
		return fmt.Errorf("creating snapshot: %w", err)
	}

	fmt.Println(T("labctl.snapshot.create.success", nil))
	fmt.Println(T("labctl.snapshot.create.vmLine", map[string]any{"Vm": response.VMName}))
	fmt.Println(T("labctl.snapshot.create.snapshotLine", map[string]any{"Name": response.Snapshot}))
	fmt.Println(T("labctl.snapshot.create.statusLine", map[string]any{"Status": response.Status}))

	return nil
}

// Revert reverts a VM to a snapshot
func (s *SnapshotCommands) Revert(ctx context.Context, podID, vmName, snapshotName string) error {
	path := fmt.Sprintf("/api/v1/pods/%s/vms/%s/reset", podID, vmName)

	request := map[string]string{
		"snapshot": snapshotName,
	}

	var response struct {
		Status   string `json:"status"`
		VMName   string `json:"vmName"`
		Snapshot string `json:"snapshot"`
	}

	if err := s.client.Post(ctx, path, request, &response); err != nil {
		return fmt.Errorf("reverting to snapshot: %w", err)
	}

	fmt.Println(T("labctl.snapshot.revert.success", nil))
	fmt.Println(T("labctl.snapshot.revert.vmLine", map[string]any{"Vm": response.VMName}))
	fmt.Println(T("labctl.snapshot.revert.snapshotLine", map[string]any{"Name": response.Snapshot}))
	fmt.Println(T("labctl.snapshot.revert.statusLine", map[string]any{"Status": response.Status}))

	return nil
}

// Delete deletes a snapshot from a VM
func (s *SnapshotCommands) Delete(ctx context.Context, podID, vmName, snapshotName string) error {
	path := fmt.Sprintf("/api/v1/pods/%s/vms/%s/snapshots/%s", podID, vmName, snapshotName)

	var response struct {
		Status   string `json:"status"`
		PodID    string `json:"podId"`
		VMName   string `json:"vmName"`
		Snapshot string `json:"snapshot"`
	}

	if err := s.client.Delete(ctx, path, &response); err != nil {
		return fmt.Errorf("deleting snapshot: %w", err)
	}

	fmt.Println(T("labctl.snapshot.delete.success", nil))
	fmt.Println(T("labctl.snapshot.delete.vmLine", map[string]any{"Vm": response.VMName}))
	fmt.Println(T("labctl.snapshot.delete.snapshotLine", map[string]any{"Name": response.Snapshot}))
	fmt.Println(T("labctl.snapshot.delete.statusLine", map[string]any{"Status": response.Status}))

	return nil
}
