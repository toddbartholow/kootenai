---
title: Managing Students
description: Guide for instructors on managing students, teams, and class progress
tags:
  - instructor
  - students
  - teams
  - management
---

# Managing Students

This guide covers how instructors can manage students, organize teams, and track class progress in Kootenai.

## Student Overview

### Viewing Your Students

Access your students through the instructor dashboard:

1. Go to **Instructor** → **Students**
2. View all students in your organization
3. Filter by team, course, or activity status

### Student Information

For each student, you can see:

| Field | Description |
|-------|-------------|
| Name | Display name |
| Email | Contact email |
| Teams | Team memberships |
| Labs Completed | Total completed labs |
| Active Sessions | Current in-progress labs |
| Last Active | Most recent activity |

## Teams

Teams help organize students into groups (classes, sections, cohorts).

### Creating a Team

1. Go to **Instructor** → **Teams**
2. Click **Create Team**
3. Fill in:
    - **Name** - e.g., "CYB 101 - Fall 2025"
    - **Slug** - URL-friendly identifier
    - **Description** - Optional details

### Adding Students to Teams

=== "Individual Add"
    1. Open the team
    2. Click **Add Member**
    3. Search by name or email
    4. Select role (member/lead)

=== "Bulk Import"
    1. Open the team
    2. Click **Import Members**
    3. Upload CSV with columns: `email,role`
    4. Review and confirm

### Team Roles

| Role | Permissions |
|------|-------------|
| **Member** | View team content, participate in labs |
| **Lead** | Member permissions + manage team members |

### Team-Specific Labs

Assign labs to specific teams:

1. Go to lab settings
2. Under **Visibility**, select **Team Only**
3. Choose which teams can access

## Progress Tracking

### Class Dashboard

The class dashboard shows aggregate progress:

```
┌─────────────────────────────────────────────────┐
│ CYB 101 - Fall 2025                             │
├─────────────────────────────────────────────────┤
│ Students: 45  │ Active: 12  │ Completed: 156    │
├─────────────────────────────────────────────────┤
│                                                 │
│  Lab Completion Rate                            │
│  ████████████████████░░░░░░  78%               │
│                                                 │
│  Average Score                                  │
│  ██████████████████░░░░░░░░  72%               │
│                                                 │
└─────────────────────────────────────────────────┘
```

### Individual Progress

Track individual student progress:

1. Click on a student's name
2. View their:
    - Completed labs and scores
    - Active sessions
    - Time spent
    - Checkpoint history

### Progress Reports

Generate progress reports:

1. Go to **Reports** → **Progress**
2. Select team or all students
3. Choose date range
4. Export as CSV or PDF

## Session Management

### Viewing Active Sessions

See all active student sessions:

1. Go to **Sessions** → **Active**
2. Filter by lab, team, or student
3. View real-time progress

### Helping Stuck Students

If a student is stuck:

1. Find their active session
2. View which checkpoint they're on
3. Use the console view to see their VM state
4. Offer guidance based on their progress

!!! note "Privacy"
    Instructors can view session progress but should respect student privacy when viewing console screens.

### Extending Sessions

If a student needs more time:

1. Find their session
2. Click **Extend Time**
3. Choose extension duration

## Assignments

### Canvas LMS Integration

If using Canvas:

1. Labs appear as assignments in Canvas
2. Students launch labs directly from Canvas
3. Grades sync automatically after submission

### Manual Assignment

Without Canvas:

1. Share lab URLs with students
2. Set due dates in Kootenai
3. Track completions manually

### Due Dates

Configure due dates:

1. Go to lab settings
2. Set **Due Date** and **Time**
3. Choose whether to:
    - Prevent late submissions
    - Allow with penalty
    - Allow without penalty

## Grading

### Auto-Grading

Most grading is automatic:

- Checkpoints are detected automatically
- Points are calculated based on completions
- Pass/fail determined by threshold

### Manual Adjustments

Sometimes you may need to adjust grades:

1. Find the student's session
2. Click **Adjust Grade**
3. Add/remove points
4. Add a note explaining the adjustment

### Grade Export

Export grades for your records:

1. Go to **Reports** → **Grades**
2. Select team and date range
3. Choose format (CSV, Excel)
4. Download

## Communication

### Announcements

Post announcements for your team:

1. Go to team page
2. Click **Post Announcement**
3. Write your message
4. Choose visibility (team/all)

### Individual Messages

Contact individual students:

- Use the email link on their profile
- Or integrate with your institution's messaging system

## Best Practices

### Setting Up a Course

1. **Create a team** for your course section
2. **Add students** via bulk import
3. **Configure labs** with appropriate visibility
4. **Set due dates** aligned with your syllabus
5. **Test labs yourself** before assigning

### During the Semester

1. **Monitor progress** regularly
2. **Identify struggling students** early
3. **Provide office hours** for lab help
4. **Collect feedback** on lab difficulty

### End of Semester

1. **Export grades** for records
2. **Archive team** (optional)
3. **Review analytics** for next semester
4. **Update labs** based on feedback

## Troubleshooting

### Student Can't Access Lab

1. Verify they're in the correct team
2. Check lab visibility settings
3. Confirm their account is active

### Grades Not Syncing to Canvas

1. Verify Canvas integration is configured
2. Check that the student launched from Canvas
3. Manually sync if needed

### Student Lost Progress

1. Check if their pod was destroyed
2. Look for previous session history
3. Consider allowing a re-attempt

## Related Guides

- [Grading & Assessment](grading.md) - Detailed grading guide
- [Creating Labs](../lab-templates/template-creation-guide.md) - Create custom labs
- [Canvas LMS Integration](../admin/canvas-lms-integration.md) - Setup Canvas
