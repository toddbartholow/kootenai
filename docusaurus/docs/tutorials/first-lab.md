# Tutorial: Your First Lab

This tutorial walks you through the complete lifecycle of a lab - from creating a pod to completing objectives and submitting your work.

## Prerequisites

- Kootenai platform running (see [Quick Start](../quickstart-demo.md))
- Logged into the web interface
- (For real VMs) Proxmox connected with lab templates loaded

## Overview

The lab workflow has 5 steps:

```
1. Browse Lab Catalog → 2. Create Pod → 3. Start Session → 4. Complete Objectives → 5. Submit
```

---

## Step 1: Browse the Lab Catalog

1. Click **"Lab Catalog"** in the sidebar (or navigate to `/labs`)

2. You'll see available labs with:
   - Name and description
   - Difficulty level (Beginner, Intermediate, Advanced)
   - Duration estimate
   - Points available

3. Click on a lab to see details:
   - Learning objectives
   - Prerequisites
   - VM configuration
   - Checkpoint requirements

**Example**: Let's use "Linux Foundations" - a beginner lab teaching basic commands.

---

## Step 2: Create a Pod

A **Pod** is your personal instance of the lab - isolated VMs just for you.

### From the Lab Detail Page:

1. Click the **"Start Lab"** button

2. The system will:
   - Clone VM templates
   - Configure networking
   - Set up your isolated environment

3. Wait for provisioning (1-3 minutes for real VMs, instant in demo mode)

4. Status will change from "Provisioning" → "Running"

### What Gets Created:

```
Pod: linux-foundations-abc123
├── VM: student-vm (Ubuntu 22.04)
│   ├── IP: 10.0.100.10
│   └── Status: Running
└── Network: VLAN 100 (10.0.100.0/24)
```

---

## Step 3: Start a Session

A **Session** tracks your progress, time, and earned points.

1. From your pod, click **"Start Session"**

2. A timer begins counting your lab time

3. The sidebar shows:
   - Session timer
   - Checkpoint progress (0/X completed)
   - Current points (0/100)

### Session States:

| State | Meaning |
|-------|---------|
| Active | You're working on the lab |
| Ended | Session stopped, not graded |
| Graded | Submitted and scored |

---

## Step 4: Access the VM Console

Now you need to connect to your VM to complete objectives.

### Option A: Web Console (Recommended)

1. Click the **"Console"** tab in your session view

2. Select your VM (e.g., "student-vm")

3. Click **"Open Console"**

4. A VNC viewer opens in your browser

5. Log in with credentials from the lab instructions:
   ```
   Username: student
   Password: student123
   ```

### Option B: SSH Access

If the VM has an accessible IP:

```bash
ssh student@10.0.100.10
# Password: student123
```

### Console Tips:

- **Ctrl+Alt+Delete**: Send to VM (not your computer)
- **Paste**: Right-click or Shift+Insert
- **Full Screen**: Click the expand icon
- **Refresh**: If display freezes, click refresh

---

## Step 5: Complete Objectives

Each lab has **checkpoints** (objectives) worth points. The system automatically detects completion.

### Example Objectives:

#### Objective 1: Create a File (25 points)
```bash
# In the VM terminal:
touch ~/hello.txt
echo "Hello Kootenai" > ~/hello.txt
```

**How it's detected**: The Wazuh agent monitors file creation and reports to the platform.

#### Objective 2: Install a Package (25 points)
```bash
sudo apt update
sudo apt install -y cowsay
```

**How it's detected**: Package installation events are captured.

#### Objective 3: Create a User (25 points)
```bash
sudo useradd -m labuser
sudo passwd labuser
# Enter password twice
```

**How it's detected**: User creation events are monitored.

#### Objective 4: Run a Command (25 points)
```bash
cowsay "I completed the lab!"
```

**How it's detected**: Command execution is logged.

### Watching Progress:

As you complete objectives, the sidebar updates in real-time:

```
Checkpoints: 2/4 completed
Points: 50/100
Progress: ████████░░░░░░ 50%
```

---

## Step 6: Submit Your Work

When you've completed objectives (or want to end the session):

1. Click the **"Submit"** button in the session view

2. Review the submission dialog:
   - Points earned
   - Checkpoints completed
   - Pass/fail status

3. Click **"Confirm Submit"**

### Grading:

```
Your Score: 75/100 (75%)
Pass Threshold: 70%
Status: PASSED ✓

Checkpoints:
✓ Create a file (25 pts)
✓ Install package (25 pts)
✓ Create user (25 pts)
✗ Run command (0 pts)
```

### What Happens After Submit:

1. **Session locks** - No more changes
2. **Grade recorded** - Stored in your history
3. **Achievements checked** - You might earn badges!
4. **Pod remains** - You can review (but not change progress)

---

## Step 7: Claim Achievements

After submitting, check if you earned any achievements:

1. A notification appears if you earned something new

2. Go to **"Achievements"** in the sidebar

3. New achievements show with celebration animation

### Example Achievements:

| Achievement | Requirement | Points |
|-------------|-------------|--------|
| First Steps | Complete 1 lab | 10 |
| Quick Learner | Complete lab under time limit | 25 |
| Perfect Score | Get 100% on any lab | 50 |

---

## Step 8: Clean Up

When you're done:

1. **End Session** (if not submitted): Click "End Session"

2. **Delete Pod**: Click "Delete Pod" to free resources
   - VMs are destroyed
   - Network is released
   - Cannot be undone!

3. Or **Keep Pod**: Leave running to continue later
   - Note: Pods may auto-expire after 2-24 hours

---

## Troubleshooting

### "Pod stuck in Provisioning"

- Wait 2-3 minutes for real VMs
- Check Proxmox connectivity: `curl http://localhost:8080/ready`
- View logs: `docker compose logs api`

### "Console won't connect"

- Ensure VM is in "Running" state
- Try refreshing the page
- Check browser console for WebSocket errors
- Verify Proxmox VNC proxy is accessible

### "Checkpoint not completing"

- Ensure you're in the correct VM
- Check exact syntax (case-sensitive paths)
- Wait 10-30 seconds for detection
- Some checkpoints have dependencies

### "Can't log into VM"

- Check lab instructions for correct credentials
- Try default: `student` / `student123`
- Some labs use: `root` / `toor`

---

## Video Walkthrough

*(Coming soon: Screen recording of complete lab workflow)*

---

## Next Steps

Now that you've completed your first lab:

1. **Try more labs**: Browse the catalog for your skill level
2. **Follow a pathway**: Structured learning tracks
3. **Earn achievements**: Collect badges and points
4. **Create your own lab**: See [Lab Template Guide](../lab-templates/template-creation-guide.md)

---

## Quick Reference

### Keyboard Shortcuts (in Console)

| Shortcut | Action |
|----------|--------|
| Ctrl+Alt+Delete | Send to VM |
| Shift+Insert | Paste |
| F11 | Toggle fullscreen |

### Common Commands

```bash
# Check your progress from VM
echo "Check the web UI for live progress"

# Find files you need to create
cat /home/student/objectives.txt

# Get hints
cat /home/student/hints.txt
```

### Status Icons

| Icon | Meaning |
|------|---------|
| 🟢 | Running/Healthy |
| 🟡 | Provisioning/Pending |
| 🔴 | Stopped/Error |
| ✓ | Checkpoint Complete |
| ○ | Checkpoint Pending |
