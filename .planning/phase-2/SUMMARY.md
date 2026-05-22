# Phase 2 SUMMARY — Group 页面 Channel 状态增强

**Status:** COMPLETE
**Commit:** c266715
**Date:** 2026-05-22

## What Was Done

### Task 1 — 后端 `breaker.go`
- `StatusEntry` struct 新增 `Threshold int \`json:"threshold"\`` 字段
- `Manager.Status()` 中填充 `Threshold: b.threshold`
- `/api/circuit/status` 响应现在每条 entry 包含 `threshold` 字段

### Task 2 — 前端 `groups/page.tsx`
- `CircuitEntry` interface 新增 `threshold: number`
- 新增 `tick` state + `useEffect` (1s interval)，触发每秒 re-render 驱动倒计时
- channel 卡片状态显示升级为四态：
  - `closed` + `failures === 0` → 绿色 "OK"
  - `closed` + `failures > 0` → 橙色 "N / threshold fails"
  - `half_open` → 黄色 "Testing"
  - `open` → 红色 "熔断 · 剩余 Xs"（每秒实时倒数，基于 `next_retry - Date.now()`）

## Files Changed
- `internal/gateway/circuit/breaker.go`
- `web/src/app/(dashboard)/groups/page.tsx`

## Verification Steps
1. 触发 channel 失败 1-2 次 → 卡片显示橙色 "N / 3 fails"
2. 第 3 次失败 → 红色 "熔断 · 剩余 30s" 并实时倒数
3. 30s 后 → 黄色 "Testing"
4. 下次请求成功 → 绿色 "OK"
