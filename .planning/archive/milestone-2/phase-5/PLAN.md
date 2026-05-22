# Phase 5 Plan — Channel 状态语义化

**Goal:** 将 Groups 页面每个 channel 卡片的状态从四态熔断 badge 换成语义化状态：Running / Ready / Tripped / Testing。

---

## Task 1 — 前端：替换 channel pill 状态逻辑

**File:** `web/src/app/(dashboard)/groups/page.tsx`

### 1a. 添加 `getChannelStatus` 帮助函数（组件外，纯函数）

```ts
type ChannelStatus =
  | { kind: "running" }
  | { kind: "ready" }
  | { kind: "tripped"; secsLeft: number }
  | { kind: "testing" };

function getChannelStatus(
  items: GroupItem[],
  idx: number,
  circuitMap: Record<number, CircuitEntry>,
  groupMode: string,
  now: number
): ChannelStatus {
  const cb = circuitMap[items[idx].channel_id];
  if (cb?.state === "open") {
    const secsLeft = cb.next_retry
      ? Math.max(0, Math.ceil((new Date(cb.next_retry).getTime() - now) / 1000))
      : 0;
    return { kind: "tripped", secsLeft };
  }
  if (cb?.state === "half_open") return { kind: "testing" };
  // healthy (closed or no entry)
  if (groupMode === "failover") {
    const firstHealthyIdx = items.findIndex(
      (it) => circuitMap[it.channel_id]?.state !== "open"
    );
    return idx === firstHealthyIdx ? { kind: "running" } : { kind: "ready" };
  }
  return { kind: "running" };
}
```

### 1b. 替换 channel pill 中的状态 badge 块

在 Groups 表格 `{g.items?.map((it, i) => {` 块内，替换当前的四段 `{cbState === ...}` 条件渲染为：

```tsx
{(() => {
  const status = getChannelStatus(g.items, i, circuitMap, g.mode, tick > 0 ? Date.now() : Date.now());
  if (status.kind === "running") return (
    <span className="inline-flex items-center gap-0.5 text-emerald-600 dark:text-emerald-400">
      <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 inline-block" />
      Running
    </span>
  );
  if (status.kind === "ready") return (
    <span className="inline-flex items-center gap-0.5 text-sky-600 dark:text-sky-400">
      <span className="h-1.5 w-1.5 rounded-full bg-sky-400 inline-block" />
      Ready
    </span>
  );
  if (status.kind === "testing") return (
    <span className="inline-flex items-center gap-0.5 text-amber-600 dark:text-amber-400">
      <span className="h-1.5 w-1.5 rounded-full bg-amber-500 inline-block" />
      Testing
    </span>
  );
  // tripped
  return (
    <span className="inline-flex items-center gap-0.5 text-destructive">
      <span className="h-1.5 w-1.5 rounded-full bg-destructive inline-block" />
      Tripped · {status.secsLeft}s
    </span>
  );
})()}
```

### 1c. 移除不再需要的变量

删除 channel pill 内的 `const cb = circuitMap[it.channel_id];` 和 `const cbState = cb?.state ?? "closed";` 两行（状态现在由 `getChannelStatus` 计算）。

---

## Task 2 — 前端构建

```bash
cd /Users/liuguoyuan/workspace/llmux/web && pnpm build
```

---

## Task 3 — 重编 Go binary & 重启

```bash
cd /Users/liuguoyuan/workspace/llmux && go build -o llmux .
pkill -f llmux || true
./llmux start &
```

---

## Task 4 — Playwright e2e 测试

**File:** `/tmp/test_phase5.py`

测试场景：
1. **All healthy (failover group)**: 第一个 channel 显示 Running，其余显示 Ready
2. **All healthy (round_robin group)**: 所有 channel 显示 Running
3. **With tripped channel (via circuit API mock)**: 验证 Tripped badge 出现
4. **Countdown updates**: tick 后 secsLeft 变化

```python
"""Phase 5 e2e: Channel semantic status badges."""
import time
import requests
from playwright.sync_api import sync_playwright

BASE = "http://localhost:9090"
ADMIN_PASS = "DbIRGJY/UC/GvX/54ZXGPz28"

def get_token():
    r = requests.post(f"{BASE}/api/auth/login",
                      json={"username": "admin", "password": ADMIN_PASS})
    return r.json()["token"]

def get_groups(token):
    return requests.get(f"{BASE}/api/groups",
                        headers={"Authorization": f"Bearer {token}"}).json()

def test_semantic_badges(page, token):
    groups = get_groups(token)
    # Find a group with 2+ items
    multi = next((g for g in groups if g.get("items") and len(g["items"]) >= 2), None)
    if not multi:
        print("  SKIP: No group with 2+ channels for semantic test")
        return

    mode = multi["mode"]
    print(f"  Testing group '{multi['name']}' mode={mode} items={len(multi['items'])}")

    page.goto(f"{BASE}/groups")
    page.wait_for_load_state("domcontentloaded")
    page.wait_for_selector("tbody tr", timeout=10000)
    page.wait_for_timeout(1000)  # let circuit poll fire

    # Collect all badge texts visible in channel pills
    badges = page.evaluate("""
        () => {
            const spans = document.querySelectorAll('tbody td span');
            const found = [];
            for (const s of spans) {
                const t = s.textContent.trim();
                if (['Running', 'Ready', 'Testing'].includes(t) || t.startsWith('Tripped')) {
                    found.push(t);
                }
            }
            return found;
        }
    """)
    print(f"  Semantic badges found: {badges}")

    assert len(badges) > 0, "FAIL: No semantic status badges (Running/Ready/Tripped/Testing)"
    assert any(b == "Running" for b in badges), f"FAIL: No 'Running' badge found: {badges}"

    if mode == "failover":
        running_count = badges.count("Running")
        assert running_count == 1, f"FAIL: failover mode should have exactly 1 Running, got {running_count}"
        ready_count = badges.count("Ready")
        assert ready_count >= 1, f"FAIL: failover mode should have Ready badges, got {ready_count}"
        print(f"  PASS: failover — 1 Running + {ready_count} Ready ✓")
    else:
        # round_robin / random / etc: all healthy = all Running
        non_running = [b for b in badges if b != "Running" and not b.startswith("Tripped")]
        print(f"  PASS: {mode} mode — Running={badges.count('Running')}, other={non_running} ✓")

    # Verify old badge styles are gone
    old_ok = page.locator("span:text-is('OK')").count()
    assert old_ok == 0, f"FAIL: Old 'OK' badge still present (count={old_ok})"
    old_fails = page.evaluate("""
        () => [...document.querySelectorAll('span')].filter(
            s => /\\d+\\s*\\/\\s*\\d+\\s*fails/.test(s.textContent)
        ).length
    """)
    assert old_fails == 0, f"FAIL: Old 'N/M fails' badge still present (count={old_fails})"
    print(f"  PASS: Old OK/fails badges gone ✓")

def main():
    print("=== Phase 5 E2E: Channel Semantic Status Badges ===")
    token = get_token()
    print(f"Login: ok")

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        ctx = browser.new_context()
        page = ctx.new_page()
        page.goto(f"{BASE}/login")
        page.wait_for_load_state("domcontentloaded")
        page.evaluate(f"localStorage.setItem('llmux_token', '{token}')")

        print("\n--- Test 1: Semantic badges present ---")
        test_semantic_badges(page, token)

        page.screenshot(path="/tmp/groups_phase5.png", full_page=True)
        print("  Screenshot: /tmp/groups_phase5.png")
        browser.close()

    print("\n✓ Phase 5 E2E complete")

if __name__ == "__main__":
    main()
```

---

## Execution Order

Task 1 (code) → Task 2 (build) → Task 3 (restart) → Task 4 (e2e)

## Commit

```
feat(5-01): Channel semantic states - Running/Ready/Tripped/Testing replace circuit badges
```
