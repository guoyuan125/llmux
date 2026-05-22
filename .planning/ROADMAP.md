# ROADMAP

## Milestone 1 — Bug Fixes & Observability

### Phase 1: Fix Session Stickiness Bug

**Goal:** channel 从 group 移除后，session stickiness 不再将请求路由到该 channel。

**Deliverables:**
- `internal/gateway/relay/gateway.go`: `HandleRelay` 中 moveToFront 后校验 sticky channel 是否仍在当前 group items 中，不在则清除 session 并忽略 stickiness
- 无需新接口、无需 DB 变更

**Verification:**
- 将 channel A 加入 group，发送几次请求建立 session stickiness
- 将 channel A 从 group 移除
- 后续请求不再路由到 channel A

---

### Phase 2: Group 页面 Channel 状态增强

**Goal:** Group 列表中每个 channel 卡片实时展示熔断状态、剩余失败次数、熔断恢复倒计时。

**Deliverables:**
- 后端：`/api/circuit/status` 响应增加 `threshold` 字段（当前固定为 3，需从 Manager 暴露）
- 前端 `web/src/app/(dashboard)/groups/page.tsx`：
  - closed 状态：绿色，显示 "OK"
  - open 状态：红色，显示 "熔断 · 剩余 Xs"（JS 每秒倒数 next_retry）
  - half_open 状态：黄色，显示 "Testing"
  - 失败次数 < threshold 但 > 0：橙色，显示 "X fails / threshold"

**Verification:**
- 触发 channel 失败，观察卡片颜色和计数实时变化
- 熔断后倒计时归零自动变为 Testing

---

### Phase 3: Logs 页面增强

**Goal:** error 日志行突出显示，展开可查看格式化请求/响应全文。

**Deliverables:**
- 前端 Logs 页面：
  - error 行整行红色背景
  - 每行左侧展开箭头，点击展开显示 request_body / response_body（格式化 JSON，等宽字体）
  - 展开区域支持滚动（max-height 限制）

**Verification:**
- 触发一次失败请求，Logs 页面该行显示红色背景
- 展开后能看到格式化的请求体和错误响应

---

## Execution Order

Phase 1 → Phase 2 → Phase 3（顺序执行，互不依赖）

---
*Created: 2026-05-22*
