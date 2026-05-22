# Phase 2: Group 页面 Channel 状态增强 — PLAN

**Goal:** Group 列表中每个 channel 卡片实时展示熔断状态、剩余失败次数、熔断恢复倒计时。

---

## Context

### 后端现状

`internal/gateway/circuit/breaker.go` 中 `StatusEntry` 已有字段：
- `Key`, `ChannelID`, `State` (`"closed"/"open"/"half_open"`)
- `Failures`, `LastFailure`, `NextRetry`
- **缺少** `Threshold` 字段，无法在前端计算 "X fails / threshold"

`Manager.Status()` 方法已正确填充所有现有字段，但未暴露 `threshold`。

### 前端现状

`web/src/app/(dashboard)/groups/page.tsx`：
- `circuitMap` 已建立，已有三态显示（closed/half_open/open）
- open 状态：`"Open · X fails"`，next_retry 仅用于 tooltip title，无实时倒计时
- 无 threshold 概念，无橙色 "warns" 态
- 轮询间隔：5000ms（每 5 秒调一次 `/api/circuit/status`）

---

## Tasks

### Task 1 — 后端：`StatusEntry` 增加 `threshold` 字段

**文件：** `internal/gateway/circuit/breaker.go`

**改动：**

1. `StatusEntry` struct 新增字段：
   ```go
   Threshold int `json:"threshold"`
   ```

2. `Manager.Status()` 中填充该字段（`b.threshold` 已存在于 `Breaker`）：
   ```go
   out = append(out, StatusEntry{
       // ...existing fields...
       Threshold: b.threshold,
   })
   ```

**验证：** `GET /api/circuit/status` 响应中每条 entry 包含 `threshold` 字段（值为 3）。

---

### Task 2 — 前端：倒计时 hook + 状态逻辑

**文件：** `web/src/app/(dashboard)/groups/page.tsx`

#### 2a. 添加每秒更新的 tick state

在组件顶层添加：
```tsx
const [tick, setTick] = useState(0);
useEffect(() => {
  const t = setInterval(() => setTick((n) => n + 1), 1000);
  return () => clearInterval(t);
}, []);
```

`tick` 变化触发重渲染，倒计时显示即每秒更新，无需额外 state。

#### 2b. 更新 `CircuitEntry` 接口

```tsx
interface CircuitEntry {
  key: string;
  channel_id: number;
  state: "closed" | "open" | "half_open";
  failures: number;
  threshold: number;           // 新增
  last_failure: string;
  next_retry: string;
}
```

#### 2c. 替换 channel 卡片内状态显示区

定位到文件第 426-444 行的三态渲染块，**完整替换**为以下逻辑：

```tsx
{/* closed + no failures → green OK */}
{cbState === "closed" && (!cb || cb.failures === 0) && (
  <span className="inline-flex items-center gap-0.5 text-emerald-600 dark:text-emerald-400">
    <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 inline-block" />
    OK
  </span>
)}
{/* closed + some failures (< threshold) → orange warn */}
{cbState === "closed" && cb && cb.failures > 0 && (
  <span className="inline-flex items-center gap-0.5 text-orange-500 dark:text-orange-400">
    <span className="h-1.5 w-1.5 rounded-full bg-orange-500 inline-block" />
    {cb.failures} / {cb.threshold} fails
  </span>
)}
{/* half_open → amber Testing */}
{cbState === "half_open" && (
  <span className="inline-flex items-center gap-0.5 text-amber-600 dark:text-amber-400">
    <span className="h-1.5 w-1.5 rounded-full bg-amber-500 inline-block" />
    Testing
  </span>
)}
{/* open → red 熔断 · 剩余 Xs 倒计时 */}
{cbState === "open" && (() => {
  const secsLeft = cb?.next_retry
    ? Math.max(0, Math.ceil((new Date(cb.next_retry).getTime() - Date.now()) / 1000))
    : 0;
  return (
    <span className="inline-flex items-center gap-0.5 text-destructive">
      <span className="h-1.5 w-1.5 rounded-full bg-destructive inline-block" />
      熔断 · 剩余 {secsLeft}s
    </span>
  );
})()}
```

**注意：**
- `tick` 在 `secsLeft` 计算中通过 `Date.now()` 隐式依赖（每秒 re-render 即重新计算）
- 不需要在 JSX 中显式引用 `tick`，但需确保 `tick` state 在组件 scope 内以触发 re-render

---

## Constraints

- 不引入新依赖
- 只改 `breaker.go` 和 `groups/page.tsx`，不动其他文件
- 前端不需要重新构建静态资源（本地开发模式验证即可）；若需要 embed，`make build-web` 后重跑

---

## Verification

1. 启动服务（`go run ./cmd/... start`）
2. 在 Groups 页面添加一个会失败的 channel（如错误 API key）
3. 发送请求触发失败：
   - 1-2 次失败 → 卡片变橙色，显示 "1 / 3 fails"、"2 / 3 fails"
   - 第 3 次失败 → 熔断，变红，显示 "熔断 · 剩余 30s" 并实时倒数
   - 30s 后 → 变为 "Testing"（amber）
   - 下次请求成功 → 变回绿色 "OK"
