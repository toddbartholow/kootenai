# Objective Tracking and Progress System

This document describes the architecture for tracking student progress through ordered lab objectives, course completion, and platform-wide achievements.

## Overview

The objective tracking system enables:

- **Ordered task completion** - Students must complete objectives in a predefined sequence
- **Multi-level progress tracking** - Platform, course, lab, and objective-level progress
- **Achievement system** - Gamification through badges, points, and leaderboards
- **Real-time updates** - Immediate feedback when objectives are completed

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                                    KOOTENAI PLATFORM                                     │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐    │
│  │                           WEB INTERFACE (Vue.js)                                 │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐                  │    │
│  │  │ PLATFORM STATS  │  │  COURSE VIEW    │  │   LAB VIEW      │                  │    │
│  │  │                 │  │                 │  │                 │                  │    │
│  │  │ Total Users: 247│  │ Course: SecOps  │  │ Lab: Firewall   │                  │    │
│  │  │ Active Labs: 89 │  │ Progress: ████░ │  │ Config 101      │                  │    │
│  │  │ Completions: 1.2k│ │ 78%             │  │                 │                  │    │
│  │  │                 │  │                 │  │ Objectives:     │                  │    │
│  │  │ Leaderboard:    │  │ Labs:           │  │ ✓ 1. Login      │                  │    │
│  │  │ 1. Alice  2450  │  │ ✓ Lab 1 ████    │  │ ✓ 2. Add rule   │                  │    │
│  │  │ 2. Bob    2100  │  │ ✓ Lab 2 ████    │  │ ▶ 3. Test rule  │                  │    │
│  │  │ 3. Carol  1950  │  │ ▶ Lab 3 ██░░    │  │ ○ 4. Block ICMP │                  │    │
│  │  │                 │  │ ○ Lab 4 ░░░░    │  │ ○ 5. Verify     │                  │    │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────┘                  │    │
│  │                                                                                  │    │
│  │  ┌───────────────────────────────────────────────────────────────────────────┐  │    │
│  │  │                        ACHIEVEMENTS PANEL                                  │  │    │
│  │  │  🏆 First Blood    🔥 10-Day Streak   🛡️ Security Pro   🌟 All Labs Done  │  │    │
│  │  │  ✓ Unlocked        ✓ Unlocked         ▶ 8/10 tasks      ○ 12/20 labs      │  │    │
│  │  └───────────────────────────────────────────────────────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────────────┘    │
│                                          │                                               │
│                                          ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐    │
│  │                         ORCHESTRATION API (Go)                                   │    │
│  │                                                                                  │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │    │
│  │  │  Progress    │  │  Objective   │  │ Achievement  │  │   Event      │         │    │
│  │  │  Service     │  │  Tracker     │  │   Engine     │  │   Handler    │         │    │
│  │  │              │  │              │  │              │  │              │         │    │
│  │  │ - User stats │  │ - Order      │  │ - Rules      │  │ - Wazuh      │         │    │
│  │  │ - Course %   │  │ - Prereqs    │  │ - Triggers   │  │ - Webhooks   │         │    │
│  │  │ - Lab %      │  │ - Validation │  │ - Awards     │  │ - Canvas LTI │         │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘         │    │
│  │         │                  │                 │                 ▲                 │    │
│  │         └──────────────────┴─────────────────┴─────────────────┤                 │    │
│  │                                    │                           │                 │    │
│  │                                    ▼                           │                 │    │
│  │                          ┌──────────────────┐                  │                 │    │
│  │                          │    PostgreSQL    │                  │                 │    │
│  │                          │                  │                  │                 │    │
│  │                          │ • user_progress  │                  │                 │    │
│  │                          │ • objectives     │                  │                 │    │
│  │                          │ • achievements   │                  │                 │    │
│  │                          │ • events_log     │                  │                 │    │
│  │                          └──────────────────┘                  │                 │    │
│  └─────────────────────────────────────────────────────────────────────────────────┘    │
│                                                                   │                      │
│ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│
│                              MONITORING LAYER                     │                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐    │
│  │                          WAZUH MANAGER                         │                 │    │
│  │                                                                │                 │    │
│  │   ┌────────────────────────────────────────────────────────────┴───┐             │    │
│  │   │                     EVENT PROCESSING                           │             │    │
│  │   │                                                                │             │    │
│  │   │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │             │    │
│  │   │  │  Decoders   │ -> │   Rules     │ -> │  Integrator │────────┼─────────────┘    │
│  │   │  │             │    │             │    │             │        │                   │
│  │   │  │ • syslog    │    │ • file_mon  │    │ • webhook   │   Objective               │
│  │   │  │ • auth      │    │ • cmd_exec  │    │ • API call  │   Complete!               │
│  │   │  │ • custom    │    │ • custom    │    │             │                           │
│  │   │  └─────────────┘    └─────────────┘    └─────────────┘                           │
│  │   └────────────────────────────────────────────────────────────────┘                 │
│  │                                    ▲                                                  │
│  │                                    │ Agent Reports                                    │
│  │                                    │                                                  │
│  └────────────────────────────────────┼──────────────────────────────────────────────┘  │
│                                       │                                                  │
│ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│
│                              LAB ENVIRONMENT                                             │
│                                                                                          │
│  ┌────────────────────────────── POD (Student A) ──────────────────────────────────┐    │
│  │                                                                                  │    │
│  │   ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐             │    │
│  │   │   VM: Kali      │    │  VM: Firewall   │    │  VM: Target     │             │    │
│  │   │   Linux         │    │  pfSense        │    │  Windows Server │             │    │
│  │   │                 │    │                 │    │                 │             │    │
│  │   │ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │             │    │
│  │   │ │Wazuh Agent  │ │    │ │Wazuh Agent  │ │    │ │Wazuh Agent  │ │             │    │
│  │   │ │             │ │    │ │             │ │    │ │             │ │             │    │
│  │   │ │ Monitors:   │ │    │ │ Monitors:   │ │    │ │ Monitors:   │ │             │    │
│  │   │ │ • ~/.bash_  │ │    │ │ • /conf/    │ │    │ │ • EventLog  │ │             │    │
│  │   │ │   history   │ │    │ │   config.xml│ │    │ │ • Services  │ │             │    │
│  │   │ │ • /etc/     │ │    │ │ • Rules log │ │    │ │ • Registry  │ │             │    │
│  │   │ │ • Processes │ │    │ │ • Interfaces│ │    │ │ • Files     │ │             │    │
│  │   │ └──────┬──────┘ │    │ └──────┬──────┘ │    │ └──────┬──────┘ │             │    │
│  │   └────────┼────────┘    └────────┼────────┘    └────────┼────────┘             │    │
│  │            │                      │                      │                       │    │
│  │            └──────────────────────┴──────────────────────┘                       │    │
│  │                                   │                                              │    │
│  │                          VLAN 100 (Isolated)                                     │    │
│  └───────────────────────────────────┼──────────────────────────────────────────────┘    │
│                                      │                                                   │
│  ┌────────────────────────────── POD (Student B) ───────────────────────────────────┐   │
│  │                          VLAN 101 (Isolated)                                      │   │
│  │   [ Similar structure - each student gets isolated pod ]                          │   │
│  └───────────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                          │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

## Data Flow: Objective Completion

```
    Student Action          Wazuh Detection           API Processing          UI Update
    ──────────────          ───────────────           ──────────────          ─────────
         │                        │                         │                      │
         │  1. Adds firewall      │                         │                      │
         │     rule               │                         │                      │
         ▼                        │                         │                      │
    ┌─────────┐                   │                         │                      │
    │ pfSense │                   │                         │                      │
    │ config  │                   │                         │                      │
    │ changed │                   │                         │                      │
    └────┬────┘                   │                         │                      │
         │                        │                         │                      │
         │  2. Agent detects      │                         │                      │
         │     file change        │                         │                      │
         └───────────────────────>│                         │                      │
                                  ▼                         │                      │
                           ┌─────────────┐                  │                      │
                           │ Rule Match: │                  │                      │
                           │ "firewall_  │                  │                      │
                           │  rule_add"  │                  │                      │
                           └──────┬──────┘                  │                      │
                                  │                         │                      │
                                  │  3. Webhook fires       │                      │
                                  └────────────────────────>│                      │
                                                            ▼                      │
                                                     ┌─────────────┐               │
                                                     │ Validate:   │               │
                                                     │ • Prereqs   │               │
                                                     │ • Criteria  │               │
                                                     │ • Award pts │               │
                                                     └──────┬──────┘               │
                                                            │                      │
                                                            │  4. Check            │
                                                            │     achievements     │
                                                            ▼                      │
                                                     ┌─────────────┐               │
                                                     │ Achievement │               │
                                                     │ Unlocked!   │               │
                                                     │ "First      │               │
                                                     │  Firewall"  │               │
                                                     └──────┬──────┘               │
                                                            │                      │
                                                            │  5. Push update      │
                                                            └─────────────────────>│
                                                                                   ▼
                                                                            ┌─────────────┐
                                                                            │ Real-time   │
                                                                            │ Progress    │
                                                                            │ Update +    │
                                                                            │ Achievement │
                                                                            │ Toast!      │
                                                                            └─────────────┘
```

## Detection Methods

### Wazuh Agent Monitoring

Wazuh agents on each lab VM detect objective completion through:

| Detection Type | Use Case | Example |
|----------------|----------|---------|
| File Integrity Monitoring | Config file changes | `/etc/ssh/sshd_config` modified |
| Log Analysis | Command execution | `useradd labuser` in auth log |
| Process Monitoring | Service running | `httpd` process active |
| Command Monitoring | Specific commands | `iptables -A INPUT...` executed |

### Custom Wazuh Rules

```xml
<!-- Example: Detect firewall rule creation -->
<rule id="100001" level="3">
  <decoded_as>pfsense</decoded_as>
  <field name="action">pass|block</field>
  <description>Firewall rule created on pfSense</description>
  <group>lab_objective,firewall</group>
</rule>

<!-- Example: Detect user creation -->
<rule id="100002" level="3">
  <if_sid>5901</if_sid>
  <match>useradd</match>
  <description>New user account created</description>
  <group>lab_objective,user_mgmt</group>
</rule>
```

## Lab Template with Objectives

```yaml
name: firewall-basics
platform: proxmox
description: Learn to configure pfSense firewall rules

objectives:
  - id: obj-1
    name: "Login to firewall"
    description: "Access the pfSense web interface"
    order: 1
    points: 100
    detection:
      type: log_match
      vm: firewall
      pattern: "Successful login.*admin"

  - id: obj-2
    name: "Create allow rule"
    description: "Allow HTTP traffic from LAN to WAN"
    order: 2
    points: 200
    requires: [obj-1]
    detection:
      type: file_change
      vm: firewall
      path: /conf/config.xml
      pattern: "<pass>.*<destination>.*80"

  - id: obj-3
    name: "Block ICMP"
    description: "Create a rule to block ping requests"
    order: 3
    points: 300
    requires: [obj-2]
    detection:
      type: file_change
      vm: firewall
      path: /conf/config.xml
      pattern: "<block>.*<protocol>icmp"

  - id: obj-4
    name: "Verify configuration"
    description: "Test that rules work correctly"
    order: 4
    points: 200
    requires: [obj-3]
    detection:
      type: command_exec
      vm: attacker
      pattern: "ping.*-c.*target.*100% packet loss"
```

## Progress Hierarchy

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           PLATFORM LEVEL                                         │
│                                                                                  │
│   Total Progress: ████████████████████░░░░░░░░░░░░░░░░░░░░  42%                 │
│   Points: 15,420 / 50,000                                                        │
│   Achievements: 12/45 unlocked                                                   │
│                                                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                        COURSE LEVEL                                      │   │
│   │                                                                          │   │
│   │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐       │   │
│   │  │ Network Security │  │ Incident Response│  │ Cloud Security   │       │   │
│   │  │ ████████████░░░░ │  │ ████████░░░░░░░░ │  │ ░░░░░░░░░░░░░░░░ │       │   │
│   │  │ 75% Complete     │  │ 50% Complete     │  │ Not Started      │       │   │
│   │  │                  │  │                  │  │                  │       │   │
│   │  │ Labs: 6/8        │  │ Labs: 4/8        │  │ Labs: 0/6        │       │   │
│   │  │ Points: 8,200    │  │ Points: 5,100    │  │ Points: 0        │       │   │
│   │  └────────┬─────────┘  └──────────────────┘  └──────────────────┘       │   │
│   │           │                                                              │   │
│   │   ┌───────┴───────────────────────────────────────────────────────┐     │   │
│   │   │                        LAB LEVEL                               │     │   │
│   │   │                                                                │     │   │
│   │   │  Lab 1: Firewall Basics         Lab 2: IDS Configuration      │     │   │
│   │   │  ████████████████████ 100%      ████████████████░░░░ 80%      │     │   │
│   │   │  ✓ All 5 objectives             ▶ 4/5 objectives              │     │   │
│   │   │  Points: 1,000                  Points: 800/1,000             │     │   │
│   │   │  Time: 45 min                   Time: 38 min (in progress)    │     │   │
│   │   │                                                                │     │   │
│   │   │  ┌─────────────────────────────────────────────────────────┐  │     │   │
│   │   │  │                   OBJECTIVE LEVEL                        │  │     │   │
│   │   │  │                                                          │  │     │   │
│   │   │  │  ✓ Obj 1: Login to firewall              [100 pts]      │  │     │   │
│   │   │  │  ✓ Obj 2: Create allow rule              [200 pts]      │  │     │   │
│   │   │  │  ✓ Obj 3: Create deny rule               [200 pts]      │  │     │   │
│   │   │  │  ✓ Obj 4: Test with ping                 [200 pts]      │  │     │   │
│   │   │  │  ▶ Obj 5: Block specific IP (CURRENT)    [300 pts]      │  │     │   │
│   │   │  │      └── Prereq: Obj 1-4 ✓                               │  │     │   │
│   │   │  │                                                          │  │     │   │
│   │   │  └─────────────────────────────────────────────────────────┘  │     │   │
│   │   │                                                                │     │   │
│   │   └────────────────────────────────────────────────────────────────┘     │   │
│   │                                                                          │   │
│   └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

## Achievement System

### Achievement Categories

| Category | Description | Examples |
|----------|-------------|----------|
| **Progression** | Based on completing content | First lab, all tutorials, course champion |
| **Skill-Based** | Demonstrating specific skills | Firewall master, threat hunter, crypto king |
| **Special** | Unique behaviors | Speed runs, night owl, no hints used |

### Achievement Triggers

Achievements are evaluated whenever:

1. An objective is completed
2. A lab is finished
3. A course is completed
4. Daily login occurs
5. Specific time-based events occur

### Example Achievements

```yaml
achievements:
  - id: first-blood
    name: "First Blood"
    description: "Complete your first lab"
    icon: "trophy"
    points: 50
    trigger:
      type: lab_complete
      count: 1

  - id: firewall-master
    name: "Firewall Master"
    description: "Configure 50 firewall rules across all labs"
    icon: "shield"
    points: 500
    trigger:
      type: objective_tag
      tag: "firewall"
      count: 50

  - id: speed-demon
    name: "Speed Demon"
    description: "Complete any lab in under 15 minutes"
    icon: "lightning"
    points: 200
    trigger:
      type: lab_complete
      max_duration_minutes: 15

  - id: streak-7
    name: "On Fire"
    description: "Complete labs 7 days in a row"
    icon: "fire"
    points: 350
    trigger:
      type: daily_streak
      days: 7
```

## API Endpoints

### Progress Endpoints

```
GET  /api/v1/progress/platform          # Platform-wide stats
GET  /api/v1/progress/user/:userId      # User's overall progress
GET  /api/v1/progress/course/:courseId  # Course progress for user
GET  /api/v1/progress/lab/:labId        # Lab progress for user
```

### Objective Endpoints

```
GET  /api/v1/objectives/lab/:labId      # List objectives for a lab
GET  /api/v1/objectives/:id             # Get objective details
POST /api/v1/objectives/:id/complete    # Mark objective complete (webhook)
```

### Achievement Endpoints

```
GET  /api/v1/achievements               # List all achievements
GET  /api/v1/achievements/user/:userId  # User's unlocked achievements
GET  /api/v1/leaderboard                # Platform leaderboard
```

## Database Schema

```sql
-- Objective definitions
CREATE TABLE objectives (
    id UUID PRIMARY KEY,
    lab_id UUID REFERENCES labs(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    order_index INTEGER NOT NULL,
    points INTEGER DEFAULT 0,
    detection_config JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Objective prerequisites
CREATE TABLE objective_prerequisites (
    objective_id UUID REFERENCES objectives(id),
    requires_objective_id UUID REFERENCES objectives(id),
    PRIMARY KEY (objective_id, requires_objective_id)
);

-- User progress on objectives
CREATE TABLE user_objectives (
    user_id UUID REFERENCES users(id),
    objective_id UUID REFERENCES objectives(id),
    completed_at TIMESTAMP,
    points_earned INTEGER DEFAULT 0,
    PRIMARY KEY (user_id, objective_id)
);

-- Achievement definitions
CREATE TABLE achievements (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    icon VARCHAR(50),
    points INTEGER DEFAULT 0,
    trigger_config JSONB NOT NULL,
    category VARCHAR(50)
);

-- User achievements
CREATE TABLE user_achievements (
    user_id UUID REFERENCES users(id),
    achievement_id UUID REFERENCES achievements(id),
    unlocked_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (user_id, achievement_id)
);

-- Event log for objective detection
CREATE TABLE objective_events (
    id UUID PRIMARY KEY,
    pod_id UUID REFERENCES pods(id),
    objective_id UUID REFERENCES objectives(id),
    event_type VARCHAR(50),
    event_data JSONB,
    processed BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW()
);
```

## Integration with Canvas LMS

Progress can be synced to Canvas gradebook:

1. Objective completion updates assignment score
2. Course completion triggers Canvas module completion
3. Achievements can be displayed as Canvas badges

## Related Documentation

- [Platform Comparison](platform-comparison.md)
- [API Reference](../api/reference.md)
