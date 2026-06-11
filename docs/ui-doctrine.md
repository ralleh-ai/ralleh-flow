# Ralleh Flow UI Doctrine

Ralleh Flow is an operational command surface for autonomous work.
It is not a CRUD app, dashboard template, project manager, or AI chat wrapper.

This document defines the design doctrine for the UI phase so the product stays focused on operational awareness, trust, and intervention instead of drifting into SaaS slop.

## Core Product Posture

Design for:
- command, not administration
- live operations, not static configuration
- situational awareness, not vanity metrics
- trust through visibility, not abstraction
- intervention when needed, not passive reporting

The product should feel closer to a mission control surface than a generic admin panel.

## Primary UX Goal

A user should be able to enter the product and understand the operational state in roughly ten seconds.

Every major surface should answer:
- What is happening right now?
- What needs my attention?
- What changed?
- What is blocked?
- What should happen next?

If a screen cannot answer those questions quickly, it is not doing its job.

## Information Hierarchy

Prioritize information in this order:
1. Active operations
2. Attention required
3. Agent activity
4. Outcomes
5. Historical context

This order should influence layout, typography, motion, and visual emphasis.

## Command, Not CRUD

Users are not here to manage records.
They are here to direct operations.

Avoid interfaces that feel like:
- form-heavy administration
- large undifferentiated tables
- generic analytics dashboards
- database record management
- workflow-builder spaghetti

Prefer interfaces that show:
- operational state
- relationships between work
- reasons for intervention
- immediate consequences of decisions

## Situational Awareness

Situational awareness is the north star.

Users should quickly understand:
- what agents are doing
- what runs are active
- what approvals are pending
- what repositories are changing
- what artifacts are being produced
- what outcomes are emerging

Clarity of operational state matters more than decorative polish.

## Relationship Visibility

The system’s value comes from making relationships visible.

The UI should make these relationships obvious:
- Objective -> Run
- Run -> Step
- Run -> Agent
- Run -> Approval
- Run -> Repository / Branch / Worktree
- Run -> Artifacts
- Run -> Outcome
- Asset / Intelligence -> Operation

Users should not have to reconstruct these relationships by hopping between disconnected screens.

## Git Is Trust UX

Git is a first-class trust mechanism and should remain visible in the UI.

Where relevant, expose:
- repository
- branch
- worktree
- commits
- pull requests
- diffs / change surfaces

Do not bury Git context under “advanced details.”

## Agent Activity

Agent activity should feel alive and operational.
It should feel like observing specialists at work.

Good:
- Research agent analyzing source set
- Implementation agent preparing change set
- Review gate awaiting operator decision

Avoid making agent activity feel like:
- raw logs as the primary experience
- terminal dumps
- chatbot transcript theater
- debugging tools presented as product UX

Logs can exist, but they should support the operational story, not replace it.

## Approval Doctrine

Approvals are trust events.
They should be treated as deliberate operational moments, not button rows.

Every approval surface should show as much of the following as the system can truthfully provide:
- objective
- context
- current operation
- relevant changes
- Git context
- evidence
- recommendation
- risks or consequences
- required decision
\nIf information is not yet available, do not fake it. Surface the missing context honestly and improve the system over time.

## Run Monitoring Doctrine

A run should read like a mission in progress.

Run detail should converge toward a unified operational view containing:
- current state
- progression
- agent activity
- approvals
- artifacts
- Git state
- outputs and outcomes

Users should not need to hunt across multiple disconnected surfaces to understand a single run.

## Assets As Intelligence

Assets are not a file cabinet.

Do not frame inputs primarily as:
- uploads
- folders
- documents
- directories

Prefer:
- intelligence
- context
- resources
- evidence
- knowledge packs

The interface should imply strategic use, not storage.

## Density and Tone

Ralleh Flow is a professional tool for serious operators.

Design for:
- information density
- restrained accents
- calm surfaces
- compact layouts
- readable hierarchy
- strong state signaling

Avoid:
- giant empty cards
- startup gradients
- glassmorphism
- neon AI clichés
- decorative motion
- “executive KPI dashboard” aesthetics

The visual tone should feel credible, sharp, and controlled.

## Motion Principles

Motion exists only to communicate state.
Use it for:
- progress
- status change
- agent activity
- attention shifts
- transitions that clarify context

Do not use motion as decoration.

## Design System Priorities

Build reusable UI systems for:
- operational status
- workflow / run state
- approval state
- agent identity
- Git context
- artifacts and evidence
- activity streams
- relationship visualization
- attention states
- outcomes

Consistency is more important than novelty.

## Anti-Patterns

Treat these as warning signs:
- dashboards full of low-value metrics
- large undifferentiated data tables
- BPMN-style diagram sprawl
- file-manager mental models
- chat-first AI wrapper UI
- decorative enterprise SaaS styling

If a design starts looking like a template, stop and re-evaluate from first principles.

## Product Test

A design element should stay only if it improves at least one of these:
- situational awareness
- trust
- clarity
- control

If it does not, remove it.

## One-Sentence Standard

Ralleh Flow should make the user feel:
- informed
- confident
- empowered
- in control

The target reaction is:

> I always know what my agents are doing, why they are doing it, and what I need to do next.
