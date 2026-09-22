# Instructor Guide

This guide is for instructors and course administrators using Kootenai to deliver hands-on cybersecurity and network operations training.

## Overview

As an instructor, you can:

- **Manage lab templates** - Create and customize labs for your courses
- **Monitor student progress** - Track completion, scores, and time spent
- **Create learning pathways** - Structured sequences of labs
- **Manage organizations** - Set up courses and teams
- **View analytics** - Access dashboard and leaderboards

---

## Getting Started

### 1. Instructor Role

To access instructor features, your account needs the `instructor` role. Contact your administrator to have this assigned.

Instructor privileges include:
- Creating/editing lab templates
- Viewing all student sessions (within your organization)
- Creating pathways and modules
- Managing teams and enrollments

### 2. Accessing the Dashboard

After logging in:

1. Navigate to the **Dashboard** (`/dashboard`)
2. Instructors see additional tabs:
   - **My Students** - Progress overview
   - **Lab Analytics** - Usage statistics
   - **Manage** - Template and pathway management

---

## Managing Lab Templates

### Creating a New Lab Template

1. Go to **Manage > Lab Templates > Create New**

2. Fill in metadata:
   ```yaml
   Name: linux-security-101
   Description: Introduction to Linux security fundamentals
   Version: 1.0.0
   Platform: proxmox
   Difficulty: beginner
   Duration: 60 minutes
   Max Points: 100
   Pass Threshold: 70
   ```

3. Define VMs and Networks:
   ```yaml
   vms:
     - name: student-workstation
       template: ubuntu-22-wazuh
       resources:
         cpu: 2
         memory: 4096
       networks:
         - name: lab-net
           ip: 10.0.100.10/24

   networks:
     - name: lab-net
       type: isolated
       subnet: 10.0.100.0/24
       gateway: 10.0.100.1
   ```

4. Configure checkpoints (objectives):
   ```yaml
   checkpoints:
     - id: create-user
       name: Create security user
       description: Create a new user account named 'secadmin'
       points: 25
       trigger:
         type: wazuh_syscheck
         match:
           path: /etc/passwd
           content: secadmin

     - id: set-permissions
       name: Configure file permissions
       description: Set correct permissions on /etc/shadow
       points: 25
       trigger:
         type: wazuh_syscheck
         match:
           path: /etc/shadow
           permissions: "640"
   ```

5. Click **Save & Validate**

### Lab Template Best Practices

| Practice | Description |
|----------|-------------|
| Clear objectives | Each checkpoint should have specific, measurable criteria |
| Scaffolded difficulty | Order checkpoints from easiest to hardest |
| Reasonable time | Allow 1.5x expected completion time |
| Helpful instructions | Include hints file in VM |
| Test thoroughly | Complete the lab yourself before publishing |

### Publishing Labs

Labs have three states:

- **Draft** - Not visible to students
- **Active** - Available in catalog
- **Archived** - Hidden but history preserved

To publish:
1. Edit the lab template
2. Toggle **Active** status to `true`
3. Save changes

---

## Creating Learning Pathways

Pathways group related labs into structured learning sequences.

### Pathway Structure

```
Pathway: Cybersecurity Fundamentals
├── Module 1: Linux Basics
│   ├── Lab: Linux Commands 101
│   └── Lab: File Permissions
├── Module 2: Network Security
│   ├── Lab: Firewall Configuration
│   └── Lab: Network Scanning
└── Module 3: Security Monitoring
    ├── Lab: Log Analysis
    └── Lab: Incident Response
```

### Creating a Pathway

1. Go to **Manage > Pathways > Create New**

2. Fill in pathway metadata:
   - Name
   - Description
   - Cover image
   - Estimated duration
   - Target audience

3. Add modules:
   - Click **Add Module**
   - Set name, description, and unlock requirements
   - Add labs to the module (in order)

4. Configure unlock rules:
   ```yaml
   # Sequential unlock - must complete previous module
   unlockRule:
     type: sequential

   # Prerequisite unlock - specific modules required
   unlockRule:
     type: prerequisite
     modules: [module-1, module-2]

   # Point threshold unlock
   unlockRule:
     type: points
     minimum: 150
   ```

5. Publish the pathway

### Enrollment Management

Students can self-enroll in published pathways, or you can:

1. **Bulk enroll** students from a team
2. **Send invitations** via email
3. **Assign via LTI** from Canvas

---

## Monitoring Student Progress

### Class Overview

The **My Students** dashboard shows:

| Metric | Description |
|--------|-------------|
| Active Sessions | Currently running labs |
| Completion Rate | % of enrolled students who passed |
| Average Score | Mean points earned |
| Time Spent | Total hours in labs |

### Individual Progress

Click on a student to see:

- Enrolled pathways with progress %
- Session history (all labs attempted)
- Achievements earned
- Time spent per lab

### Progress Alerts

Configure alerts for:
- Students stuck (no progress in X hours)
- Low scores (below threshold)
- Deadline approaching (Canvas integration)

---

## Grading and Assessment

### Automatic Grading

The platform automatically grades based on checkpoint completion:

```
Score = Σ(checkpoint_points for completed checkpoints)
Grade = Score / Max_Points * 100
Passed = Grade >= Pass_Threshold
```

### Manual Review

For some labs, you may want to review work manually:

1. Go to **Sessions > [Student Session]**
2. Click **Review Submission**
3. View checkpoint status and timestamps
4. Optionally adjust score
5. Add instructor comments

### Canvas Grade Sync

:::warning Gated off in this build
The LTI routes return 403 in any build from this repository, so none of the steps below
reach Canvas. See [Enterprise Edition](../enterprise.md).
:::

When integrated with Canvas LMS:

1. Student launches lab from Canvas assignment
2. Session linked to Canvas submission
3. Upon lab submit, grade pushes to Canvas gradebook
4. Comments sync as submission feedback

---

## Analytics Dashboard

### Available Reports

| Report | Description |
|--------|-------------|
| Lab Completion | Pass/fail rates per lab |
| Time Analysis | Average time vs. expected time |
| Checkpoint Heatmap | Which objectives are most challenging |
| Student Rankings | Leaderboard by points |
| Activity Timeline | When students are most active |

### Exporting Data

Export reports in CSV or JSON for:
- External analysis
- Accreditation documentation
- Research purposes

---

## Canvas LMS Integration

Kootenai integrates with Canvas LMS via LTI 1.3 for seamless lab delivery and grade synchronization.

> **Note:** For detailed setup instructions, see the [Canvas LMS Integration Guide](../admin/canvas-lms-integration.md).

### Features

| Feature | Description |
|---------|-------------|
| Single Sign-On | Students launch labs directly from Canvas |
| Grade Passback | Scores sync automatically to Canvas gradebook |
| Course Mapping | Canvas courses map to Kootenai organizations |
| Section Sync | Canvas sections can sync to Kootenai teams |

### Setting Up LTI 1.3

Contact your Kootenai administrator to configure the LTI integration. They will need:

1. Your Canvas instance URL
2. Admin access to create Developer Keys
3. Course IDs for tool installation

The administrator will provide:
- **Client ID** for your Canvas instance
- **Installation instructions** for your courses

### Local Development Setup

For testing LTI integration locally, you can run Canvas in Docker:

```bash
# Clone Canvas
git clone https://github.com/instructure/canvas-lms.git
cd canvas-lms

# Start Canvas (see admin guide for full setup)
docker compose up -d

# Access at http://localhost:3001
# Login: admin@example.com / password123
```

See the [Canvas LMS Integration Guide](../admin/canvas-lms-integration.md) for complete local setup instructions.

### Creating Canvas Assignments

1. In Canvas, create new assignment
2. Choose "External Tool" submission type
3. Select Kootenai from the tool list
4. Choose specific lab from the picker (if deep linking enabled)
5. Set due date and point value

### Grade Passback

When students submit their lab:
- Score automatically posted to Canvas gradebook
- Submission marked as "submitted"
- Instructor can view detailed progress in Kootenai

### Course-to-Organization Mapping

When students launch from Canvas:
- Canvas course maps to a Kootenai organization
- Students are auto-enrolled in the organization
- Canvas sections can map to teams (if enabled)

| Canvas | Kootenai |
|--------|-------------|
| Course | Organization |
| Section | Team |
| Teacher/TA | Instructor role |
| Student | Member role |

### Troubleshooting Canvas Integration

**"Tool not appearing in course"**
1. Verify the Developer Key is active
2. Check the tool is installed in the specific course
3. Confirm course navigation placement is enabled

**"Grade not syncing"**
1. Ensure assignment was created with External Tool submission type
2. Verify student launched lab from Canvas (not direct URL)
3. Check API logs for grade passback errors

**"Student sees wrong organization"**
1. Verify course mapping in Kootenai admin
2. Check LTI launch includes correct course context

---

## Organization Management

### Setting Up Your Organization

Organizations isolate data between courses/institutions:

1. Go to **Admin > Organizations**
2. Click **Create Organization**
3. Configure:
   - Name
   - Slug (URL-friendly)
   - Settings (session limits, default quotas)

### Managing Teams

Teams group students for collaborative work:

1. Go to **Organizations > [Org] > Teams**
2. Create team with name and description
3. Add students (by email or bulk import)
4. Assign pathway enrollments to team

### Quota Management

Set resource limits per organization:

| Quota | Description |
|-------|-------------|
| Max Active Pods | Concurrent lab instances |
| Max Session Duration | Hours before auto-expire |
| Storage Per Pod | GB limit for VM snapshots |
| Monthly Hours | Total compute time budget |

---

## Common Tasks

### Resetting Student Progress

If a student needs to restart:

1. Find their session in **Sessions**
2. Click **Reset Progress**
3. Optionally delete pod for fresh environment

### Extending Session Time

If a student needs more time:

1. Find active session
2. Click **Extend**
3. Add additional minutes (up to max duration)

### Viewing Replay Data

For debugging checkpoint detection:

1. Go to **Sessions > [Session] > Replay**
2. View timeline of detected events
3. See what triggered (or didn't trigger) checkpoints

### Generating Reports

1. Go to **Analytics**
2. Select report type
3. Filter by:
   - Date range
   - Lab/pathway
   - Student/team
4. Export or view in-app

---

## Best Practices for Instructors

### Before Class

- [ ] Verify lab templates are active and tested
- [ ] Check Proxmox templates exist for all VMs
- [ ] Review checkpoint detection is working
- [ ] Set up pathway if using structured course

### During Class

- [ ] Monitor active sessions dashboard
- [ ] Watch for stuck students
- [ ] Be available for technical issues
- [ ] Note common problems for lab improvement

### After Class

- [ ] Review completion rates
- [ ] Check for outlier scores (too high/low)
- [ ] Gather student feedback
- [ ] Update labs based on issues found

---

## Troubleshooting

### "Students can't see my lab"

1. Verify lab is set to **Active**
2. Check organization permissions
3. Ensure students are enrolled in pathway (if applicable)

### "Checkpoints not detecting"

1. Check Wazuh agent running in VM: `systemctl status wazuh-agent`
2. Verify trigger configuration matches actual paths
3. Review event replay for what was detected
4. Test checkpoint manually in fresh pod

### "Grades not syncing to Canvas"

1. Verify LTI configuration is correct
2. Check session was launched from Canvas (has Canvas IDs)
3. Review API logs for grade passback errors
4. Ensure Canvas assignment points match lab points

### "Pod provisioning fails"

1. Check Proxmox connectivity
2. Verify templates exist
3. Check resource quotas not exceeded
4. Review API error logs

---

## If something does not work

- **Your institution's Kootenai administrator** — the right first contact for anything
  about your own courses, pods or templates
- **Documentation**: this guide and the references it links
- [Common issues](../troubleshooting/common-issues.md) for platform-level failures

The upstream repository is archived and accepts no issues, so an operator at your
institution owns the platform outcomes.

---

## Quick Reference

### Role Permissions

| Action | Student | Instructor | Admin |
|--------|---------|------------|-------|
| Complete labs | Yes | Yes | Yes |
| View own progress | Yes | Yes | Yes |
| View others' progress | No | Org only | All |
| Create/edit labs | No | Yes | Yes |
| Manage pathways | No | Yes | Yes |
| Manage users | No | No | Yes |
| System settings | No | No | Yes |

### API Endpoints for Instructors

```bash
# List all sessions in organization
GET /api/v1/sessions?organizationId={orgId}

# Get student progress
GET /api/v1/users/{userId}/sessions

# Get pathway enrollments
GET /api/v1/enrollments?pathwayId={pathwayId}

# Get lab analytics
GET /api/v1/labs/{labId}/stats
```

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+K` | Quick search |
| `G then D` | Go to dashboard |
| `G then S` | Go to sessions |
| `?` | Show all shortcuts |
