---
phase: 1
plan: "01"
subsystem: gateway/session stickiness
tags: [bug-fix, session, relay, unit-test]
dependency_graph:
  requires: []
  provides: [session.Store.Delete, stale-session-eviction]
  affects: [internal/gateway/session, internal/gateway/relay]
tech_stack:
  added: []
  patterns: [stale-session-eviction-on-route-change]
key_files:
  created:
    - internal/gateway/relay/gateway_test.go
  modified:
    - internal/gateway/session/session.go
    - internal/gateway/relay/gateway.go
decisions:
  - "Clear stale session immediately when moveToFront confirms sticky channel absent from current candidates, rather than waiting for TTL expiry."
metrics:
  duration: "~10 minutes"
  completed: "2026-05-22"
---

# Phase 1 Plan 01: Fix Session Stickiness Bug Summary

Session stickiness bug fixed: stale session entry is now evicted immediately when the sticky channel is absent from the current group candidates, preventing perpetual mis-routing after a channel is removed from a group.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Add `Delete` method to `session.Store` | 94b2861 | internal/gateway/session/session.go |
| 2 | Fix stickiness validation in `HandleRelay` | 6a78cb2 | internal/gateway/relay/gateway.go |
| 3 | Add unit tests for `moveToFront` and session stickiness | 10b4dec | internal/gateway/relay/gateway_test.go |

## What Was Built

- **`session.Store.Delete(apiKeyID uint, model string)`** — new public method that removes a session entry under a write lock. Uses the existing `sessionKey` helper to derive the map key.

- **Stickiness validation in `HandleRelay`** (lines 178–190 of gateway.go) — after calling `moveToFront`, the code now checks whether the sticky channel ID actually landed at position 0. If it did not (channel no longer in group), the stale session is deleted via `g.sessions.Delete` and a `[RELAY]` log line is emitted. The normal load-balancing candidate list is used unchanged. If the channel is still present, behaviour is identical to before.

- **Four unit tests** in `gateway_test.go`:
  - `TestMoveToFront_ChannelExists` — sticky channel promoted to front, original slice not mutated.
  - `TestMoveToFront_ChannelNotFound` — absent channel ID; original order returned.
  - `TestSessionStickiness_ChannelRemovedFromGroup` — replicates the HandleRelay logic; asserts session deleted and candidates unmodified.
  - `TestSessionStickiness_ChannelStillInGroup` — asserts session preserved and channel promoted when still present.

All four tests pass (`go test ./internal/gateway/relay/ -run "TestMoveToFront|TestSessionStickiness"`).

## Deviations from Plan

None — plan executed exactly as written. The only deviation was renaming the test constant `model` to `requestModel` (Rule 1 auto-fix: the identifier `model` shadowed the `model` package import, causing a build failure).

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes introduced.

## Self-Check: PASSED

- internal/gateway/session/session.go — FOUND
- internal/gateway/relay/gateway.go — FOUND
- internal/gateway/relay/gateway_test.go — FOUND
- Commit 94b2861 — FOUND
- Commit 6a78cb2 — FOUND
- Commit 10b4dec — FOUND
