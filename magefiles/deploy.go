//go:build mage

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Deploy namespace contains deployment targets
type Deploy mg.Namespace

// All deploys everything to the infra VM
func (Deploy) All() error {
	mg.SerialDeps(Deploy.Api, Deploy.Web, Deploy.Db)
	mg.Deps(Deploy.Restart)
	color.Green("✓ Full deployment complete")
	return nil
}

// Api builds and deploys the API binary to the infra VM using rsync (smart deploy - skips if up to date)
func (Deploy) Api() error {
	color.Cyan("Deploying API to infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	// Check if rebuild is needed
	needsRebuild, reason, err := shouldRebuildAPI(cfg)
	if err != nil {
		color.Yellow("  Warning: could not check remote version: %v", err)
		needsRebuild = true
		reason = "could not determine remote version"
	}

	if !needsRebuild {
		color.Green("✓ %s", reason)
		return nil
	}

	color.Yellow("  Rebuilding: %s", reason)

	// Sync api directory using rsync (excludes bin, vendor, .git)
	color.Yellow("  Syncing API files...")
	localAPIDir := filepath.Join(cfg.ProjectRoot, "api") + "/"
	rsyncCmd := fmt.Sprintf(
		"rsync -avz --delete --exclude 'bin' --exclude 'vendor' --exclude '.git' %s %s@%s:%s/api/",
		localAPIDir, cfg.InfraVM.User, cfg.InfraVM.Host, cfg.InfraVM.RepoPath,
	)
	if err := sh.RunV("bash", "-c", rsyncCmd); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	// Connect to infra VM
	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	// Save rollback point before deploying
	if err := saveRollbackPoint(client, cfg); err != nil {
		color.Yellow("  Warning: could not save rollback point: %v", err)
	}

	if err := buildAndRestartAPIContainer(client, cfg); err != nil {
		return err
	}

	color.Green("✓ API deployed (commit: %s)", getGitCommit())
	return nil
}

// Force forces a full rebuild and deploy regardless of version
func (Deploy) Force() error {
	color.Cyan("Force deploying all components...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	// Sync api directory using rsync (excludes bin, vendor, .git)
	color.Yellow("  Syncing API files...")
	localAPIDir := filepath.Join(cfg.ProjectRoot, "api") + "/"
	rsyncCmd := fmt.Sprintf(
		"rsync -avz --delete --exclude 'bin' --exclude 'vendor' --exclude '.git' %s %s@%s:%s/api/",
		localAPIDir, cfg.InfraVM.User, cfg.InfraVM.Host, cfg.InfraVM.RepoPath,
	)
	if err := sh.RunV("bash", "-c", rsyncCmd); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	// Save rollback point before deploying
	if err := saveRollbackPoint(client, cfg); err != nil {
		color.Yellow("  Warning: could not save rollback point: %v", err)
	}

	if err := buildAndRestartAPIContainer(client, cfg); err != nil {
		return err
	}

	color.Green("✓ Force deployment complete")
	return nil
}

// Versions displays all component versions from the infra VM
func (Deploy) Versions() error {
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	printVersionComparison(cfg)
	return nil
}

// Web deploys the web container to the infra VM using rsync
func (Deploy) Web() error {
	color.Cyan("Deploying Web to infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	// Sync web directory using rsync (excludes node_modules, .git, dist)
	color.Yellow("  Syncing web files...")
	localWebDir := filepath.Join(cfg.ProjectRoot, "web") + "/"
	rsyncCmd := fmt.Sprintf(
		"rsync -avz --delete --exclude 'node_modules' --exclude '.git' --exclude 'dist' %s %s@%s:%s/web/",
		localWebDir, cfg.InfraVM.User, cfg.InfraVM.Host, cfg.InfraVM.RepoPath,
	)
	if err := sh.RunV("bash", "-c", rsyncCmd); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	// Build and restart container using docker compose
	color.Yellow("  Building container...")
	buildCmd := fmt.Sprintf("cd %s/deploy && docker compose build web", cfg.InfraVM.RepoPath)
	if _, err := client.Run(buildCmd); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// Restart container
	color.Yellow("  Restarting container...")
	restartCmd := fmt.Sprintf("cd %s/deploy && docker compose up -d --force-recreate web", cfg.InfraVM.RepoPath)
	if _, err := client.Run(restartCmd); err != nil {
		return fmt.Errorf("container restart failed: %w", err)
	}

	color.Green("✓ Web deployed")
	return nil
}

// Db runs migrations on the infra VM
func (Deploy) Db() error {
	return DB{}.MigrateRemote()
}

// Restart restarts services on the infra VM
func (Deploy) Restart() error {
	color.Cyan("Restarting services on infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	// Restart API container
	color.Yellow("  Restarting API...")
	restartCmd := fmt.Sprintf("cd %s/deploy && docker compose restart api", cfg.InfraVM.RepoPath)
	if _, err := client.Run(restartCmd); err != nil {
		return fmt.Errorf("failed to restart API: %w", err)
	}

	color.Green("✓ Services restarted")
	return nil
}

// Status shows comprehensive service status on the infra VM
func (Deploy) Status() error {
	color.Cyan("Checking services on infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	fmt.Println("\n" + strings.Repeat("─", 60))
	color.Cyan("SERVICE STATUS REPORT")
	fmt.Println(strings.Repeat("─", 60))

	// Check labctl process
	fmt.Println("\n[API Service]")
	if output, err := client.Run("ps aux | grep 'labctl serve' | grep -v grep | head -1"); err == nil && output != "" {
		fields := strings.Fields(output)
		if len(fields) > 1 {
			color.Green("  Status: Running (PID: %s)", fields[1])
		} else {
			color.Green("  Status: Running")
		}
	} else {
		color.Red("  Status: NOT RUNNING")
	}

	// Check API version
	if output, err := client.Run("curl -sf http://localhost:8080/version 2>/dev/null"); err == nil {
		var version struct {
			Version   string `json:"version"`
			Commit    string `json:"commit"`
			BuildTime string `json:"build_time"`
		}
		if json.Unmarshal([]byte(output), &version) == nil {
			fmt.Printf("  Version: %s\n", version.Version)
			fmt.Printf("  Commit:  %s\n", version.Commit)
			if version.BuildTime != "" {
				fmt.Printf("  Built:   %s\n", version.BuildTime)
			}
		}
	}

	// Check health endpoints
	fmt.Println("\n[Health Checks]")
	if output, err := client.Run("curl -sf http://localhost:8080/health 2>/dev/null"); err == nil && strings.Contains(output, "ok") {
		color.Green("  /health: OK")
	} else {
		color.Red("  /health: FAIL")
	}

	if output, err := client.Run("curl -sf http://localhost:8080/ready 2>/dev/null"); err == nil {
		var ready struct {
			Status string `json:"status"`
		}
		if json.Unmarshal([]byte(output), &ready) == nil {
			if ready.Status == "ready" {
				color.Green("  /ready:  %s", ready.Status)
			} else {
				color.Yellow("  /ready:  %s", ready.Status)
			}
		}
	} else {
		color.Red("  /ready:  FAIL")
	}

	// Check docker containers
	fmt.Println("\n[Docker Containers]")
	output, _ := client.Run("docker ps --format 'table {{.Names}}\t{{.Status}}' 2>/dev/null")
	if output != "" {
		// Indent the output
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
	} else {
		color.Yellow("  No containers running")
	}

	// Check web UI
	fmt.Println("\n[Web UI]")
	if output, err := client.Run("curl -sf -o /dev/null -w '%{http_code}' http://localhost:3000/ 2>/dev/null"); err == nil {
		code := strings.TrimSpace(output)
		if code == "200" {
			color.Green("  HTTP %s - OK", code)
		} else {
			color.Yellow("  HTTP %s", code)
		}
	} else {
		color.Red("  Not responding")
	}

	// Check git status
	fmt.Println("\n[Git Repository]")
	if output, err := client.Run(fmt.Sprintf("cd %s && git log -1 --format='%%h %%s' 2>/dev/null", cfg.InfraVM.RepoPath)); err == nil {
		fmt.Printf("  Latest: %s", output)
	}
	if output, err := client.Run(fmt.Sprintf("cd %s && git status --short 2>/dev/null | head -5", cfg.InfraVM.RepoPath)); err == nil && strings.TrimSpace(output) != "" {
		color.Yellow("  Uncommitted changes:")
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			fmt.Printf("    %s\n", line)
		}
	}

	fmt.Println("\n" + strings.Repeat("─", 60))
	return nil
}

// Verify runs smoke tests to verify the deployment is healthy
func (Deploy) Verify() error {
	color.Cyan("Verifying deployment on infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	type checkResult struct {
		name    string
		passed  bool
		message string
	}

	results := []checkResult{}
	allPassed := true

	// Check 1: API Process Running
	if output, err := client.Run("pgrep -f 'labctl serve' > /dev/null && echo 'running'"); err == nil && strings.Contains(output, "running") {
		results = append(results, checkResult{"API Process", true, "Running"})
	} else {
		results = append(results, checkResult{"API Process", false, "Not running"})
		allPassed = false
	}

	// Check 2: Health endpoint
	if output, err := client.Run("curl -sf http://localhost:8080/health 2>/dev/null"); err == nil && strings.Contains(output, "ok") {
		results = append(results, checkResult{"Health Endpoint", true, "OK"})
	} else {
		results = append(results, checkResult{"Health Endpoint", false, "Not responding"})
		allPassed = false
	}

	// Check 3: Ready endpoint
	if output, err := client.Run("curl -sf http://localhost:8080/ready 2>/dev/null"); err == nil {
		var ready struct {
			Status string `json:"status"`
		}
		if json.Unmarshal([]byte(output), &ready) == nil && (ready.Status == "ready" || ready.Status == "degraded") {
			results = append(results, checkResult{"Ready Endpoint", true, ready.Status})
		} else {
			results = append(results, checkResult{"Ready Endpoint", false, "Invalid response"})
			allPassed = false
		}
	} else {
		results = append(results, checkResult{"Ready Endpoint", false, "Not responding"})
		allPassed = false
	}

	// Check 4: Labs API returns data
	if output, err := client.Run("curl -sf http://localhost:8080/api/v1/labs 2>/dev/null"); err == nil {
		var labs []interface{}
		if json.Unmarshal([]byte(output), &labs) == nil {
			results = append(results, checkResult{"Labs API", true, fmt.Sprintf("%d templates", len(labs))})
		} else {
			results = append(results, checkResult{"Labs API", false, "Invalid JSON"})
			allPassed = false
		}
	} else {
		results = append(results, checkResult{"Labs API", false, "Not responding"})
		allPassed = false
	}

	// Check 5: Web container
	if output, err := client.Run("docker ps --filter name=kootenai-web --format '{{.Status}}' 2>/dev/null"); err == nil && strings.Contains(output, "Up") {
		results = append(results, checkResult{"Web Container", true, "Running"})
	} else {
		results = append(results, checkResult{"Web Container", false, "Not running"})
		allPassed = false
	}

	// Check 6: Web UI responds
	if output, err := client.Run("curl -sf -o /dev/null -w '%{http_code}' http://localhost:3000/ 2>/dev/null"); err == nil && strings.Contains(output, "200") {
		results = append(results, checkResult{"Web UI", true, "HTTP 200"})
	} else {
		results = append(results, checkResult{"Web UI", false, "Not responding"})
		allPassed = false
	}

	// Check 7: Database connectivity (via API ready check)
	if output, err := client.Run("curl -sf http://localhost:8080/ready 2>/dev/null"); err == nil {
		var ready struct {
			Checks map[string]map[string]interface{} `json:"checks"`
		}
		if json.Unmarshal([]byte(output), &ready) == nil {
			if dbCheck, ok := ready.Checks["database"]; ok {
				if status, ok := dbCheck["status"].(string); ok && status == "healthy" {
					results = append(results, checkResult{"Database", true, "Connected"})
				} else {
					results = append(results, checkResult{"Database", false, "Unhealthy"})
					allPassed = false
				}
			}
		}
	}

	// Print results
	fmt.Println("\n┌────────────────────────────────────────────────────────────┐")
	fmt.Println("│                  DEPLOYMENT VERIFICATION                    │")
	fmt.Println("├────────────────────────────────────────────────────────────┤")

	for _, r := range results {
		status := color.GreenString("✓ PASS")
		if !r.passed {
			status = color.RedString("✗ FAIL")
		}
		fmt.Printf("│ %s  %-20s  %-25s │\n", status, r.name, r.message)
	}

	fmt.Println("└────────────────────────────────────────────────────────────┘")

	passed := 0
	for _, r := range results {
		if r.passed {
			passed++
		}
	}

	fmt.Printf("\nResults: %d/%d checks passed\n", passed, len(results))

	if !allPassed {
		color.Red("\n✗ Deployment verification FAILED")
		return fmt.Errorf("deployment verification failed: %d/%d checks passed", passed, len(results))
	}

	color.Green("\n✓ Deployment verification PASSED")
	return nil
}

// Logs tails logs from the infra VM
func (Deploy) Logs() error {
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	color.Cyan("Tailing logs from infra VM (Ctrl+C to stop)...")

	// Use SSHPASS env var instead of -p to avoid exposing password in process list
	os.Setenv("SSHPASS", cfg.InfraVM.Password)
	defer os.Unsetenv("SSHPASS")

	return sh.RunV("sshpass", "-e",
		"ssh", "-o", "StrictHostKeyChecking=accept-new",
		fmt.Sprintf("%s@%s", cfg.InfraVM.User, cfg.InfraVM.Host),
		"tail", "-f", "/tmp/labctl.log")
}

// Pull pulls latest code on infra VM
func (Deploy) Pull() error {
	color.Cyan("Pulling latest code on infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	output, err := client.Run(fmt.Sprintf("cd %s && git pull origin main", cfg.InfraVM.RepoPath))
	if err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	fmt.Println(output)
	color.Green("✓ Code pulled")
	return nil
}

// Sync synchronizes local changes to the infra VM (git commit + push + pull)
func (Deploy) Sync() error {
	color.Cyan("Syncing code to infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	// Check local git status
	localStatus, err := sh.Output("git", "status", "--short")
	if err != nil {
		return fmt.Errorf("git status failed: %w", err)
	}

	if strings.TrimSpace(localStatus) != "" {
		color.Yellow("  Local uncommitted changes detected:")
		lines := strings.Split(strings.TrimSpace(localStatus), "\n")
		for _, line := range lines {
			if len(line) > 60 {
				line = line[:57] + "..."
			}
			fmt.Printf("    %s\n", line)
		}
		fmt.Println()
		color.Yellow("  Please commit your changes before syncing, or use 'mage deploy:quick' for a quick update.")
		return fmt.Errorf("uncommitted local changes - commit before syncing")
	}

	// Push local changes
	color.Yellow("  Pushing local changes...")
	if err := sh.RunV("git", "push", "origin", "main"); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	// Pull on remote
	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	color.Yellow("  Pulling on infra VM...")
	output, err := client.Run(fmt.Sprintf("cd %s && git pull origin main", cfg.InfraVM.RepoPath))
	if err != nil {
		return fmt.Errorf("remote git pull failed: %w", err)
	}

	if strings.TrimSpace(output) != "" && !strings.Contains(output, "Already up to date") {
		fmt.Printf("  %s", output)
	}

	color.Green("✓ Code synced to infra VM")
	return nil
}

// Quick does a quick deployment (rsync + build + restart)
func (Deploy) Quick() error {
	color.Cyan("Quick deployment (rsync + build + restart)...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	// Sync api directory using rsync (excludes bin, vendor, .git)
	color.Yellow("  Syncing API files...")
	localAPIDir := filepath.Join(cfg.ProjectRoot, "api") + "/"
	rsyncCmd := fmt.Sprintf(
		"rsync -avz --delete --exclude 'bin' --exclude 'vendor' --exclude '.git' %s %s@%s:%s/api/",
		localAPIDir, cfg.InfraVM.User, cfg.InfraVM.Host, cfg.InfraVM.RepoPath,
	)
	if err := sh.RunV("bash", "-c", rsyncCmd); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	// Save current commit for rollback
	if err := saveRollbackPoint(client, cfg); err != nil {
		color.Yellow("  Warning: could not save rollback point: %v", err)
	}

	if err := buildAndRestartAPIContainer(client, cfg); err != nil {
		return err
	}

	color.Green("✓ Quick deployment complete")
	return nil
}

// Rollback reverts to the previously deployed version
func (Deploy) Rollback() error {
	color.Cyan("Rolling back to previous deployment...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	// Read the rollback point
	rollbackFile := fmt.Sprintf("%s/.rollback-commit", cfg.InfraVM.RepoPath)
	previousCommit, err := client.Run(fmt.Sprintf("cat %s 2>/dev/null", rollbackFile))
	if err != nil || strings.TrimSpace(previousCommit) == "" {
		return fmt.Errorf("no rollback point found - cannot rollback.\n" +
			"Rollback points are created automatically during deployments.\n" +
			"To manually rollback to a specific commit, use:\n" +
			"  mage deploy:rollbackTo <commit-hash>")
	}

	previousCommit = strings.TrimSpace(previousCommit)
	color.Yellow("  Found rollback point: %s", previousCommit)

	// Get current commit for comparison
	currentCommit, _ := client.Run(fmt.Sprintf("cd %s && git rev-parse --short HEAD", cfg.InfraVM.RepoPath))
	currentCommit = strings.TrimSpace(currentCommit)

	if currentCommit == previousCommit {
		color.Yellow("  Already at rollback point (%s) - nothing to do", previousCommit)
		return nil
	}

	// Confirm the rollback
	color.Yellow("  Current commit:  %s", currentCommit)
	color.Yellow("  Rollback target: %s", previousCommit)

	// Save current as new rollback point (so we can roll forward if needed)
	color.Yellow("  Saving current commit as new rollback point...")
	if _, err := client.Run(fmt.Sprintf("echo '%s' > %s.forward", currentCommit, rollbackFile)); err != nil {
		color.Yellow("  Warning: could not save forward point")
	}

	// Perform the rollback
	color.Yellow("  Checking out previous commit...")
	if _, err := client.Run(fmt.Sprintf("cd %s && git checkout %s", cfg.InfraVM.RepoPath, previousCommit)); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	// Rebuild and restart API container
	if err := buildAndRestartAPIContainer(client, cfg); err != nil {
		// Try to recover
		color.Red("  Build failed, attempting to recover...")
		client.Run(fmt.Sprintf("cd %s && git checkout %s", cfg.InfraVM.RepoPath, currentCommit))
		return fmt.Errorf("rollback build failed: %w", err)
	}

	// Verify the rollback worked
	color.Yellow("  Verifying rollback...")
	d := Deploy{}
	if err := d.Verify(); err != nil {
		color.Red("  Rollback verification failed!")
		color.Yellow("  You may need to manually investigate or roll forward with:")
		color.Yellow("    mage deploy:rollbackTo %s", currentCommit)
		return err
	}

	color.Green("✓ Rollback complete (now at %s)", previousCommit)
	return nil
}

// RollbackTo rolls back to a specific commit
func (Deploy) RollbackTo(commit string) error {
	if commit == "" {
		return fmt.Errorf("commit hash required. Usage: mage deploy:rollbackTo <commit>")
	}

	color.Cyan("Rolling back to commit %s...", commit)
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	// Save current commit as rollback point
	if err := saveRollbackPoint(client, cfg); err != nil {
		color.Yellow("  Warning: could not save rollback point: %v", err)
	}

	// Verify the commit exists
	if _, err := client.Run(fmt.Sprintf("cd %s && git cat-file -t %s", cfg.InfraVM.RepoPath, commit)); err != nil {
		return fmt.Errorf("commit %s not found in repository", commit)
	}

	// Checkout the specified commit
	color.Yellow("  Checking out %s...", commit)
	if _, err := client.Run(fmt.Sprintf("cd %s && git checkout %s", cfg.InfraVM.RepoPath, commit)); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	// Rebuild and restart API container
	if err := buildAndRestartAPIContainer(client, cfg); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	color.Green("✓ Rolled back to %s", commit)
	return nil
}

// buildAndRestartAPIContainer builds the API Docker image with version info and recreates the container
func buildAndRestartAPIContainer(client *SSHClient, cfg *Config) error {
	version := getVersion()
	commit := getGitCommit()
	buildTime := time.Now().Format(time.RFC3339)

	color.Yellow("  Building container...")
	buildCmd := fmt.Sprintf(
		"cd %s/deploy && docker compose build --build-arg VERSION=%s --build-arg COMMIT=%s --build-arg BUILD_TIME=%s api",
		cfg.InfraVM.RepoPath, version, commit, buildTime,
	)
	if _, err := client.Run(buildCmd); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	color.Yellow("  Restarting container...")
	restartCmd := fmt.Sprintf("cd %s/deploy && docker compose up -d --force-recreate api", cfg.InfraVM.RepoPath)
	if _, err := client.Run(restartCmd); err != nil {
		return fmt.Errorf("container restart failed: %w", err)
	}
	return nil
}

// saveRollbackPoint saves the current deployed commit for rollback
func saveRollbackPoint(client *SSHClient, cfg *Config) error {
	rollbackFile := fmt.Sprintf("%s/.rollback-commit", cfg.InfraVM.RepoPath)

	// Get current commit
	currentCommit, err := client.Run(fmt.Sprintf("cd %s && git rev-parse --short HEAD", cfg.InfraVM.RepoPath))
	if err != nil {
		return err
	}
	currentCommit = strings.TrimSpace(currentCommit)

	// Save it
	_, err = client.Run(fmt.Sprintf("echo '%s' > %s", currentCommit, rollbackFile))
	if err != nil {
		return err
	}

	color.Green("  Saved rollback point: %s", currentCommit)
	return nil
}

// copyBinaryToRemote copies the Linux binary to the remote server
func copyBinaryToRemote(client *SSHClient, localPath, remotePath string) error {
	// Read the binary
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	// Create temp file on remote
	tempPath := "/tmp/labctl.upload"

	// Use base64 encoding for reliable transfer
	// This is slower but more reliable than raw scp
	encodedData := base64.StdEncoding.EncodeToString(data)
	cmd := fmt.Sprintf("base64 -d > %s << 'EOF'\n%s\nEOF", tempPath, encodedData)
	if _, err := client.Run(cmd); err != nil {
		return fmt.Errorf("failed to transfer binary: %w", err)
	}

	// Move to final location and make executable
	if _, err := client.Run(fmt.Sprintf("mv %s %s && chmod +x %s", tempPath, remotePath, remotePath)); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	return nil
}

// ensureDir creates a directory on the remote if it doesn't exist
func ensureRemoteDir(client *SSHClient, dir string) error {
	_, err := client.Run(fmt.Sprintf("mkdir -p %s", dir))
	return err
}

// getRemoteBinaryPath returns the path to the labctl binary on the remote
func getRemoteBinaryPath(cfg *Config) string {
	return filepath.Join(cfg.InfraVM.RepoPath, "api", "bin", "labctl")
}
