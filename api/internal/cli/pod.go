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

// PodCommands handles pod management commands
type PodCommands struct {
	client *Client
}

// NewPodCommands creates pod command handler
func NewPodCommands(client *Client) *PodCommands {
	return &PodCommands{client: client}
}

// PodListResponse represents the API response for listing pods
type PodListResponse struct {
	Pods []*models.Pod `json:"pods"`
}

// List lists all pods
func (p *PodCommands) List(ctx context.Context, owner string) error {
	path := "/api/v1/pods"
	if owner != "" {
		path += "?owner=" + owner
	}

	var response PodListResponse
	if err := p.client.Get(ctx, path, &response); err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	if len(response.Pods) == 0 {
		fmt.Println(T("labctl.pod.list.empty", nil))
		return nil
	}

	// Column headers are locale-aware; underline row is generated from the
	// rendered widths so the rule matches even when translations change
	// header length. Same approach as cli/lab.go List.
	headers := []string{
		T("labctl.pod.list.colId", nil),
		T("labctl.pod.list.colTemplate", nil),
		T("labctl.pod.list.colOwner", nil),
		T("labctl.pod.list.colStatus", nil),
		T("labctl.pod.list.colPlatform", nil),
		T("labctl.pod.list.colVms", nil),
		T("labctl.pod.list.colCreated", nil),
		T("labctl.pod.list.colExpires", nil),
	}
	dashes := make([]string, len(headers))
	for i, h := range headers {
		dashes[i] = strings.Repeat("-", runewidth(h))
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(dashes, "\t"))

	for _, pod := range response.Pods {
		expires := T("labctl.pod.list.expiresNever", nil)
		if pod.ExpiresAt != nil {
			expires = pod.ExpiresAt.Format("Jan 02 15:04")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			truncateID(pod.ID),
			pod.LabTemplate,
			pod.Owner,
			pod.Status,
			pod.Platform,
			len(pod.VMs),
			pod.CreatedAt.Format("Jan 02 15:04"),
			expires,
		)
	}
	w.Flush()

	return nil
}

// Create creates a new pod
func (p *PodCommands) Create(ctx context.Context, templateName, owner string, duration time.Duration) error {
	request := map[string]any{
		"labTemplate": templateName,
		"owner":       owner,
	}

	if duration > 0 {
		request["duration"] = duration.String()
	}

	var response struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	if err := p.client.Post(ctx, "/api/v1/pods", request, &response); err != nil {
		return fmt.Errorf("creating pod: %w", err)
	}

	fmt.Println(T("labctl.pod.create.success", nil))
	fmt.Println(T("labctl.pod.create.idLine", map[string]any{"Id": response.ID}))
	fmt.Println(T("labctl.pod.create.statusLine", map[string]any{"Status": response.Status}))

	return nil
}

// Status shows detailed pod status
func (p *PodCommands) Status(ctx context.Context, podID string) error {
	var pod models.Pod
	if err := p.client.Get(ctx, "/api/v1/pods/"+podID, &pod); err != nil {
		return fmt.Errorf("getting pod status: %w", err)
	}

	fmt.Println(T("labctl.pod.status.heading", map[string]any{"Id": pod.ID}))
	fmt.Printf("================================================================================\n\n")
	fmt.Println(T("labctl.pod.status.fieldTemplate", map[string]any{"Value": pod.LabTemplate}))
	fmt.Println(T("labctl.pod.status.fieldOwner", map[string]any{"Value": pod.Owner}))
	fmt.Println(T("labctl.pod.status.fieldPlatform", map[string]any{"Value": pod.Platform}))
	fmt.Println(T("labctl.pod.status.fieldStatus", map[string]any{"Value": statusIcon(string(pod.Status))}))
	fmt.Println(T("labctl.pod.status.fieldCreated", map[string]any{"Value": pod.CreatedAt.Format(time.RFC3339)}))

	if pod.ExpiresAt != nil {
		remaining := time.Until(*pod.ExpiresAt)
		if remaining > 0 {
			fmt.Println(T("labctl.pod.status.expiresIn", map[string]any{
				"When": pod.ExpiresAt.Format(time.RFC3339), "Remaining": formatDuration(remaining),
			}))
		} else {
			fmt.Println(T("labctl.pod.status.expired", map[string]any{"When": pod.ExpiresAt.Format(time.RFC3339)}))
		}
	}

	if len(pod.VMs) > 0 {
		fmt.Println()
		fmt.Println(T("labctl.pod.status.vmsHeading", nil))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		vmHeaders := []string{
			T("labctl.pod.status.vmColName", nil),
			T("labctl.pod.status.vmColStatus", nil),
			T("labctl.pod.status.vmColIp", nil),
			T("labctl.pod.status.vmColPlatformId", nil),
			T("labctl.pod.status.vmColSnapshot", nil),
		}
		fmt.Fprintln(w, strings.Join(vmHeaders, "\t"))

		placeholder := T("labctl.pod.status.emptyPlaceholder", nil)
		for _, vm := range pod.VMs {
			ip := vm.IPAddress
			if ip == "" {
				ip = placeholder
			}
			snapshot := vm.CurrentSnapshot
			if snapshot == "" {
				snapshot = placeholder
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
				vm.Name,
				statusIcon(vm.Status),
				ip,
				truncateID(vm.PlatformID),
				snapshot,
			)
		}
		w.Flush()
	}

	if len(pod.Networks) > 0 {
		fmt.Println()
		fmt.Println(T("labctl.pod.status.networksHeading", nil))
		for _, net := range pod.Networks {
			fmt.Println(T("labctl.pod.status.networkLine", map[string]any{
				"Name": net.Name, "Vlan": net.VLAN, "Subnet": net.Subnet,
			}))
		}
	}

	return nil
}

// Destroy destroys a pod
func (p *PodCommands) Destroy(ctx context.Context, podID string, force bool) error {
	path := "/api/v1/pods/" + podID
	if force {
		path += "?force=true"
	}

	var response struct {
		Status string `json:"status"`
	}

	if err := p.client.Delete(ctx, path, &response); err != nil {
		return fmt.Errorf("destroying pod: %w", err)
	}

	fmt.Println(T("labctl.pod.destroy.initiated", map[string]any{"Id": truncateID(podID)}))
	fmt.Println(T("labctl.pod.destroy.status", map[string]any{"Status": response.Status}))

	return nil
}

// Reset resets a pod to a snapshot
func (p *PodCommands) Reset(ctx context.Context, podID, snapshot string) error {
	request := map[string]string{}
	if snapshot != "" {
		request["snapshot"] = snapshot
	}

	var response struct {
		Status string `json:"status"`
	}

	if err := p.client.Post(ctx, "/api/v1/pods/"+podID+"/reset", request, &response); err != nil {
		return fmt.Errorf("resetting pod: %w", err)
	}

	fmt.Println(T("labctl.pod.reset.initiated", map[string]any{"Id": truncateID(podID)}))
	if snapshot != "" {
		fmt.Println(T("labctl.pod.reset.snapshotLine", map[string]any{"Name": snapshot}))
	}
	fmt.Println(T("labctl.pod.reset.status", map[string]any{"Status": response.Status}))

	return nil
}

// ResetVM resets a specific VM in a pod
func (p *PodCommands) ResetVM(ctx context.Context, podID, vmName, snapshot string) error {
	request := map[string]string{}
	if snapshot != "" {
		request["snapshot"] = snapshot
	}

	var response struct {
		Status string `json:"status"`
	}

	path := fmt.Sprintf("/api/v1/pods/%s/vms/%s/reset", podID, vmName)
	if err := p.client.Post(ctx, path, request, &response); err != nil {
		return fmt.Errorf("resetting VM: %w", err)
	}

	fmt.Println(T("labctl.pod.resetVm.initiated", map[string]any{"Name": vmName}))
	fmt.Println(T("labctl.pod.resetVm.status", map[string]any{"Status": response.Status}))

	return nil
}

// Helper functions

func truncateID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func statusIcon(status string) string {
	switch status {
	case "running", "active":
		return "● " + status
	case "provisioning", "creating":
		return "◐ " + status
	case "stopped":
		return "○ " + status
	case "error", "failed":
		return "✗ " + status
	case "destroying":
		return "◌ " + status
	case "destroyed":
		return "⊘ " + status
	default:
		return status
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
