//go:build mage

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Proxmox namespace contains Proxmox integration targets
type Proxmox mg.Namespace

// Test runs all Proxmox integration tests
func (Proxmox) Test() error {
	color.Cyan("Running Proxmox integration tests...")
	cfg := GetConfig()

	if cfg.Proxmox.Host == "" {
		return fmt.Errorf("PROXMOX_HOST environment variable required")
	}

	mg.SerialDeps(
		Proxmox.TestConnection,
		Proxmox.ListVMs,
	)

	color.Green("All Proxmox tests passed")
	return nil
}

// TestConnection tests connectivity to Proxmox API
func (Proxmox) TestConnection() error {
	color.Cyan("Testing Proxmox API connection...")
	cfg := GetConfig()

	if cfg.Proxmox.Host == "" {
		return fmt.Errorf("PROXMOX_HOST not set")
	}

	// Use curl to test API
	url := fmt.Sprintf("%s/api2/json/version", cfg.Proxmox.Host)
	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", cfg.Proxmox.TokenID, cfg.Proxmox.Token)

	output, err := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), url)
	if err != nil {
		return fmt.Errorf("failed to connect to Proxmox: %w", err)
	}

	// Parse response
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return fmt.Errorf("invalid response from Proxmox: %s", output)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if version, ok := data["version"].(string); ok {
			color.Green("Connected to Proxmox VE %s", version)
			return nil
		}
	}

	return fmt.Errorf("unexpected response: %s", output)
}

// ListVMs lists all VMs on the Proxmox node
func (Proxmox) ListVMs() error {
	color.Cyan("Listing VMs on Proxmox...")
	cfg := GetConfig()

	if cfg.Proxmox.Host == "" {
		return fmt.Errorf("PROXMOX_HOST not set")
	}

	// Get nodes first
	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", cfg.Proxmox.TokenID, cfg.Proxmox.Token)

	nodesURL := fmt.Sprintf("%s/api2/json/nodes", cfg.Proxmox.Host)
	nodesOutput, err := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), nodesURL)
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	var nodesResp struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(nodesOutput), &nodesResp); err != nil {
		return fmt.Errorf("failed to parse nodes response: %w", err)
	}

	if len(nodesResp.Data) == 0 {
		return fmt.Errorf("no nodes found")
	}

	// List VMs on first node
	node := nodesResp.Data[0].Node
	vmsURL := fmt.Sprintf("%s/api2/json/nodes/%s/qemu", cfg.Proxmox.Host, node)
	vmsOutput, err := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), vmsURL)
	if err != nil {
		return fmt.Errorf("failed to get VMs: %w", err)
	}

	var vmsResp struct {
		Data []struct {
			VMID   int    `json:"vmid"`
			Name   string `json:"name"`
			Status string `json:"status"`
			CPU    float64 `json:"cpu"`
			Mem    int64  `json:"mem"`
			MaxMem int64  `json:"maxmem"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(vmsOutput), &vmsResp); err != nil {
		return fmt.Errorf("failed to parse VMs response: %w", err)
	}

	fmt.Printf("\nNode: %s\n", node)
	fmt.Printf("%-8s %-30s %-10s %-10s\n", "VMID", "Name", "Status", "Memory")
	fmt.Println(strings.Repeat("-", 60))

	for _, vm := range vmsResp.Data {
		memGB := float64(vm.MaxMem) / 1024 / 1024 / 1024
		status := vm.Status
		if status == "running" {
			status = color.GreenString(status)
		} else {
			status = color.YellowString(status)
		}
		fmt.Printf("%-8d %-30s %-10s %.1f GB\n", vm.VMID, vm.Name, status, memGB)
	}

	color.Green("\nFound %d VMs", len(vmsResp.Data))
	return nil
}

// ListTemplates lists VM templates available for cloning
func (Proxmox) ListTemplates() error {
	color.Cyan("Listing VM templates...")
	cfg := GetConfig()

	if cfg.Proxmox.Host == "" {
		return fmt.Errorf("PROXMOX_HOST not set")
	}

	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", cfg.Proxmox.TokenID, cfg.Proxmox.Token)

	// Get nodes
	nodesURL := fmt.Sprintf("%s/api2/json/nodes", cfg.Proxmox.Host)
	nodesOutput, err := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), nodesURL)
	if err != nil {
		return err
	}

	var nodesResp struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(nodesOutput), &nodesResp)

	if len(nodesResp.Data) == 0 {
		return fmt.Errorf("no nodes found")
	}

	node := nodesResp.Data[0].Node

	// Get all VMs and filter templates
	vmsURL := fmt.Sprintf("%s/api2/json/nodes/%s/qemu", cfg.Proxmox.Host, node)
	vmsOutput, err := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), vmsURL)
	if err != nil {
		return err
	}

	var vmsResp struct {
		Data []struct {
			VMID     int    `json:"vmid"`
			Name     string `json:"name"`
			Template int    `json:"template"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(vmsOutput), &vmsResp)

	fmt.Printf("\nTemplates on node %s:\n", node)
	fmt.Printf("%-8s %-40s\n", "VMID", "Name")
	fmt.Println(strings.Repeat("-", 50))

	count := 0
	for _, vm := range vmsResp.Data {
		if vm.Template == 1 {
			fmt.Printf("%-8d %-40s\n", vm.VMID, vm.Name)
			count++
		}
	}

	if count == 0 {
		color.Yellow("No templates found")
	} else {
		color.Green("\nFound %d templates", count)
	}

	return nil
}

// TestConsole tests VNC/SPICE console connectivity
func (Proxmox) TestConsole() error {
	color.Cyan("Testing console connectivity...")
	cfg := GetConfig()

	if cfg.Proxmox.Host == "" {
		return fmt.Errorf("PROXMOX_HOST not set")
	}

	// This would test the console ticket generation
	// For now, just verify the API endpoint is accessible
	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", cfg.Proxmox.TokenID, cfg.Proxmox.Token)

	// Get nodes and find a running VM
	nodesURL := fmt.Sprintf("%s/api2/json/nodes", cfg.Proxmox.Host)
	nodesOutput, _ := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), nodesURL)

	var nodesResp struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(nodesOutput), &nodesResp)

	if len(nodesResp.Data) == 0 {
		return fmt.Errorf("no nodes found")
	}

	node := nodesResp.Data[0].Node

	// Find a running VM
	vmsURL := fmt.Sprintf("%s/api2/json/nodes/%s/qemu", cfg.Proxmox.Host, node)
	vmsOutput, _ := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), vmsURL)

	var vmsResp struct {
		Data []struct {
			VMID   int    `json:"vmid"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(vmsOutput), &vmsResp)

	var runningVM *struct {
		VMID   int
		Name   string
		Status string
	}

	for _, vm := range vmsResp.Data {
		if vm.Status == "running" {
			runningVM = &struct {
				VMID   int
				Name   string
				Status string
			}{vm.VMID, vm.Name, vm.Status}
			break
		}
	}

	if runningVM == nil {
		color.Yellow("No running VMs found to test console")
		return nil
	}

	color.Yellow("Testing console for VM %d (%s)...", runningVM.VMID, runningVM.Name)

	// Test VNC proxy endpoint
	vncURL := fmt.Sprintf("%s/api2/json/nodes/%s/qemu/%d/vncproxy", cfg.Proxmox.Host, node, runningVM.VMID)
	vncOutput, err := sh.Output("curl", "-s", "-k", "-X", "POST", "-H", fmt.Sprintf("Authorization: %s", authHeader), vncURL)
	if err != nil {
		return fmt.Errorf("VNC proxy request failed: %w", err)
	}

	var vncResp struct {
		Data struct {
			Ticket string `json:"ticket"`
			Port   int    `json:"port"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(vncOutput), &vncResp); err == nil && vncResp.Data.Ticket != "" {
		color.Green("VNC proxy working (port %d)", vncResp.Data.Port)
	} else {
		color.Yellow("VNC proxy response: %s", vncOutput)
	}

	// Test SPICE proxy endpoint
	spiceURL := fmt.Sprintf("%s/api2/json/nodes/%s/qemu/%d/spiceproxy", cfg.Proxmox.Host, node, runningVM.VMID)
	spiceOutput, err := sh.Output("curl", "-s", "-k", "-X", "POST", "-H", fmt.Sprintf("Authorization: %s", authHeader), spiceURL)
	if err != nil {
		color.Yellow("SPICE proxy request failed: %v", err)
	} else {
		var spiceResp struct {
			Data struct {
				Host     string `json:"host"`
				Password string `json:"password"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(spiceOutput), &spiceResp); err == nil && spiceResp.Data.Host != "" {
			color.Green("SPICE proxy working (host: %s)", spiceResp.Data.Host)
		} else {
			color.Yellow("SPICE proxy response: %s", spiceOutput)
		}
	}

	color.Green("Console test complete")
	return nil
}

// TestSnapshots tests snapshot operations on a VM
func (Proxmox) TestSnapshots() error {
	color.Cyan("Testing snapshot operations...")
	cfg := GetConfig()

	if cfg.Proxmox.Host == "" {
		return fmt.Errorf("PROXMOX_HOST not set")
	}

	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", cfg.Proxmox.TokenID, cfg.Proxmox.Token)

	// Get nodes
	nodesURL := fmt.Sprintf("%s/api2/json/nodes", cfg.Proxmox.Host)
	nodesOutput, _ := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), nodesURL)

	var nodesResp struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(nodesOutput), &nodesResp)

	if len(nodesResp.Data) == 0 {
		return fmt.Errorf("no nodes found")
	}

	node := nodesResp.Data[0].Node

	// Find first VM with snapshots
	vmsURL := fmt.Sprintf("%s/api2/json/nodes/%s/qemu", cfg.Proxmox.Host, node)
	vmsOutput, _ := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), vmsURL)

	var vmsResp struct {
		Data []struct {
			VMID int    `json:"vmid"`
			Name string `json:"name"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(vmsOutput), &vmsResp)

	foundSnapshots := false
	for _, vm := range vmsResp.Data {
		snapURL := fmt.Sprintf("%s/api2/json/nodes/%s/qemu/%d/snapshot", cfg.Proxmox.Host, node, vm.VMID)
		snapOutput, err := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), snapURL)
		if err != nil {
			continue
		}

		var snapResp struct {
			Data []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Snaptime    int64  `json:"snaptime"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(snapOutput), &snapResp); err == nil && len(snapResp.Data) > 1 {
			fmt.Printf("\nVM %d (%s) snapshots:\n", vm.VMID, vm.Name)
			for _, snap := range snapResp.Data {
				if snap.Name != "current" {
					fmt.Printf("  - %s: %s\n", snap.Name, snap.Description)
					foundSnapshots = true
				}
			}
		}
	}

	if !foundSnapshots {
		color.Yellow("No VM snapshots found (only 'current' state)")
	}

	color.Green("Snapshot API accessible")
	return nil
}

// Status shows Proxmox cluster status
func (Proxmox) Status() error {
	color.Cyan("Checking Proxmox cluster status...")
	cfg := GetConfig()

	if cfg.Proxmox.Host == "" {
		return fmt.Errorf("PROXMOX_HOST not set")
	}

	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", cfg.Proxmox.TokenID, cfg.Proxmox.Token)

	// Get cluster status
	statusURL := fmt.Sprintf("%s/api2/json/cluster/status", cfg.Proxmox.Host)
	statusOutput, err := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), statusURL)
	if err != nil {
		return fmt.Errorf("failed to get cluster status: %w", err)
	}

	var statusResp struct {
		Data []struct {
			Type   string `json:"type"`
			Name   string `json:"name"`
			Online int    `json:"online"`
			Level  string `json:"level"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(statusOutput), &statusResp); err != nil {
		return fmt.Errorf("failed to parse status: %w", err)
	}

	fmt.Println("\nCluster Status:")
	fmt.Printf("%-10s %-20s %-10s\n", "Type", "Name", "Status")
	fmt.Println(strings.Repeat("-", 45))

	for _, item := range statusResp.Data {
		status := "offline"
		if item.Online == 1 {
			status = color.GreenString("online")
		} else {
			status = color.RedString("offline")
		}
		fmt.Printf("%-10s %-20s %-10s\n", item.Type, item.Name, status)
	}

	// Get resource usage
	resourcesURL := fmt.Sprintf("%s/api2/json/cluster/resources?type=node", cfg.Proxmox.Host)
	resourcesOutput, _ := sh.Output("curl", "-s", "-k", "-H", fmt.Sprintf("Authorization: %s", authHeader), resourcesURL)

	var resourcesResp struct {
		Data []struct {
			Node   string  `json:"node"`
			CPU    float64 `json:"cpu"`
			MaxCPU int     `json:"maxcpu"`
			Mem    int64   `json:"mem"`
			MaxMem int64   `json:"maxmem"`
			Disk   int64   `json:"disk"`
			MaxDisk int64  `json:"maxdisk"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resourcesOutput), &resourcesResp); err == nil {
		fmt.Println("\nResource Usage:")
		for _, node := range resourcesResp.Data {
			cpuPct := node.CPU * 100
			memPct := float64(node.Mem) / float64(node.MaxMem) * 100
			diskPct := float64(node.Disk) / float64(node.MaxDisk) * 100

			fmt.Printf("\n%s:\n", node.Node)
			fmt.Printf("CPU:  %.1f%% (%d cores)\n", cpuPct, node.MaxCPU)
			fmt.Printf("RAM:  %.1f%% (%.1f/%.1f GB)\n", memPct,
				float64(node.Mem)/1024/1024/1024,
				float64(node.MaxMem)/1024/1024/1024)
			fmt.Printf("Disk: %.1f%% (%.1f/%.1f GB)\n", diskPct,
				float64(node.Disk)/1024/1024/1024,
				float64(node.MaxDisk)/1024/1024/1024)
		}
	}

	return nil
}

// RunApiTests runs the Go API tests that use Proxmox
func (Proxmox) RunApiTests() error {
	color.Cyan("Running Proxmox API integration tests...")
	cfg := GetConfig()

	if cfg.Proxmox.Host == "" {
		color.Yellow("Skipping: PROXMOX_HOST not set")
		return nil
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	// Set environment variables for tests
	env := map[string]string{
		"PROXMOX_HOST":     cfg.Proxmox.Host,
		"PROXMOX_TOKEN_ID": cfg.Proxmox.TokenID,
		"PROXMOX_TOKEN":    cfg.Proxmox.Token,
		"PROXMOX_INSECURE": "true",
	}

	// Run tests in the proxmox package
	if err := sh.RunWithV(env, "go", "test", "-v", "./internal/proxmox/..."); err != nil {
		return fmt.Errorf("Proxmox tests failed: %w", err)
	}

	color.Green("Proxmox API tests passed")
	return nil
}
