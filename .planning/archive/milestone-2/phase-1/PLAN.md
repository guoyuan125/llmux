# Phase 1 Plan — Fix Session Stickiness Bug

**Phase goal:** channel 从 group 移除后，session stickiness 不再将请求路由到该 channel。

---

## Root Cause Analysis

`gateway.go:179-183` session stickiness 逻辑：

```go
if group.SessionKeepTime > 0 {
    if chID, _, ok := g.sessions.Get(apiKeyID, requestModel); ok {
        candidates = moveToFront(candidates, chID)
    }
}
```

`moveToFront` 遍历当前 group 的 `candidates`（来自最新 DB 的 `items`）。当 sticky channel 已从 group 移除时：
- `candidates` 中不含该 channel ✓
- `moveToFront` 找不到 chID，原样返回 candidates ✓（路由本身不出错）
- **但 session 未被清除** — 下一次请求仍会命中 `sessions.Get`，再次尝试 moveToFront

此外，`moveToFront` 目前找不到 chID 时静默返回原 candidates；若 channel 存在于 items 但已被禁用（`Enabled=false`），则该 channel 虽被 `moveToFront` 排到最前，但会在迭代时被 `continue` 跳过，其他 channel 接管请求 — session 仍不会被清除，持续尝试直到 TTL 到期。

**Fix：** moveToFront 后判断 sticky channel 是否仍在 candidates 中，不在则清除 session，正常走负载均衡。

---

## Tasks

### Task 1 — Add `Delete` to `session.Store`

**File:** `internal/gateway/session/session.go`

添加 `Delete(apiKeyID uint, model string)` 方法，用于主动清除指定 session。

```go
// Delete removes a session entry for the given API key and model.
func (s *Store) Delete(apiKeyID uint, model string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.entries, sessionKey(apiKeyID, model))
}
```

---

### Task 2 — Fix stickiness validation in `HandleRelay`

**File:** `internal/gateway/relay/gateway.go`

修改 session stickiness 逻辑（当前 line 179-183）：

**Before:**
```go
if group.SessionKeepTime > 0 {
    if chID, _, ok := g.sessions.Get(apiKeyID, requestModel); ok {
        candidates = moveToFront(candidates, chID)
    }
}
```

**After:**
```go
if group.SessionKeepTime > 0 {
    if chID, _, ok := g.sessions.Get(apiKeyID, requestModel); ok {
        reordered := moveToFront(candidates, chID)
        if len(reordered) > 0 && reordered[0].ChannelID == chID {
            candidates = reordered
        } else {
            // Sticky channel no longer in group; clear stale session.
            log.Printf("[RELAY] sticky channel %d not in group %s, clearing session", chID, group.Name)
            g.sessions.Delete(apiKeyID, requestModel)
        }
    }
}
```

---

### Task 3 — Unit test for stickiness with removed channel

**File:** `internal/gateway/relay/gateway_test.go`（新建或追加）

验证：当 sticky channel 不在当前 candidates 中时，`moveToFront` 正确返回原 candidates（channel 不被提前），且行为与预期一致。

测试覆盖：
1. `moveToFront` — channel 存在时，正确移到最前
2. `moveToFront` — channel 不存在时，返回原 slice 不变
3. Session stickiness 集成：sticky channel 不在 group items 中，session 被清除，请求按正常 candidates 顺序路由

---

## Verification

按 ROADMAP.md 手工验证步骤：

1. 将 channel A 加入 group，发送几次请求建立 session stickiness
2. 将 channel A 从 group 移除
3. 后续请求不再路由到 channel A（日志出现 `clearing session` 字样后，路由切换到其他 channel）

---

## Constraints

- 不引入新依赖
- 不修改任何无关代码
- `session.Store.Delete` 是本次新增的唯一公开方法

---
*Generated: 2026-05-22*
