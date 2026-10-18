---
title: Platform Overview
description: Introduction to the Kootenai platform for students
tags:
  - getting-started
  - overview
  - students
---

# Platform Overview

Welcome to Kootenai, a platform for hands-on cybersecurity and network operations training through interactive virtual environments.

## What is Kootenai?

Kootenai provides isolated, pre-configured lab environments where you can:

- Practice networking concepts with real routers and switches
- Learn cybersecurity techniques in safe, sandboxed environments
- Complete hands-on exercises that are automatically tracked
- Earn achievements as you progress through learning pathways

## Key Concepts

### Labs

A **lab** is a template that defines a virtual environment. Each lab includes:

- One or more virtual machines (VMs)
- Network configurations and topology
- Checkpoints (objectives) to complete
- Instructions and learning materials

### Pods

When you start a lab, you get your own **pod** - a private instance of the lab environment. Your pod is:

- **Isolated** - Only you can access your VMs
- **Persistent** - Your work is saved until you destroy the pod
- **Resettable** - You can reset VMs to their original state

### Sessions

A **session** tracks your progress within a lab. When you start a session:

1. Your checkpoints are monitored
2. Progress is automatically saved
3. You can submit when complete for grading

### Checkpoints

**Checkpoints** are objectives you need to complete. They're automatically detected when you:

- Create specific files
- Install required packages
- Configure services correctly
- Complete other defined tasks

## Getting Started

<div class="card-grid">

<div class="card">
<h3>1. Browse Labs</h3>
<p>Explore available labs from the catalog. Filter by difficulty, category, or platform.</p>
</div>

<div class="card">
<h3>2. Launch a Pod</h3>
<p>Click "Launch" on any lab to create your own private environment.</p>
</div>

<div class="card">
<h3>3. Complete Objectives</h3>
<p>Follow the instructions and complete checkpoints. Your progress is tracked automatically.</p>
</div>

<div class="card">
<h3>4. Submit & Earn</h3>
<p>Submit your work for grading and earn achievements for your accomplishments.</p>
</div>

</div>

## User Interface

### Dashboard

Your dashboard shows:

- **Active Pods** - Labs you're currently working on
- **Recent Sessions** - Your lab history
- **Achievements** - Badges and points earned
- **Pathways** - Learning tracks you're enrolled in

### Lab Catalog

Browse and search available labs by:

- **Category** - Networking, Security, Cloud, etc.
- **Difficulty** - Beginner, Intermediate, Advanced
- **Platform** - Proxmox or CloudStack
- **Duration** - Estimated time to complete

### VM Console

Access your virtual machines through the browser-based console:

- Full terminal access via VNC
- Copy/paste support
- Keyboard shortcuts work as expected

## Next Steps

- [Your First Lab](../tutorials/first-lab.md) - Step-by-step tutorial
- [Working with Pods](working-with-pods.md) - Pod management guide
- [Sessions & Progress](sessions-progress.md) - Track your work
