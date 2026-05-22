# ROADMAP

## Milestone 1 — Bug Fixes & Observability ✓

### Phase 1: Fix Session Stickiness Bug ✓
### Phase 2: Group 页面 Channel 状态增强 ✓
### Phase 3: Logs 页面增强 ✓

---

## Milestone 2 — Group UX Simplification & Channel States

### Phase 4: Group 配置简化

**Goal:** 去掉 Priority 输入和正则匹配规则，让 Group channel 配置更直观。

**Deliverables:**
- 前端 `web/src/app/(dashboard)/groups/page.tsx`：
  - 移除每个 channel item 的 Priority 输入框和 Weight 输入框
  - Channel 列表改为可拖拽排序（或上下箭头），顺序即优先级
  - 提交时自动按列表位置赋 priority 值（1, 2, 3, ...）
  - Accepted Models 字段去掉通配符说明文字，改为精确匹配提示
- 后端 `internal/server/handler/group.go`（如需）：
  - 路由匹配逻辑从 glob/wildcard 改为精确匹配（逗号分隔的 exact model names）

**Verification（含 Playwright e2e 测试）：**
- 新建 Group 时无 Priority/Weight 输入框
- 调整 channel 顺序后提交，验证 API 返回的 priority 字段按顺序 1/2/3 赋值
- 发送 model 名精确匹配请求，正确路由到对应 group

---

### Phase 5: Channel 状态语义化

**Goal:** Groups 页面每个 channel 卡片展示语义化状态：Running / Ready / Tripped / Testing。

**Deliverables:**
- 前端 `web/src/app/(dashboard)/groups/page.tsx`：
  - 移除 Phase 2 的四态熔断 badge，改为语义化状态：
    - **Running**（绿色）— failover 模式下第一个非 Tripped channel；其他模式下所有健康 channel
    - **Ready**（灰蓝色）— 健康且不是当前 active，排在 Running 后面
    - **Tripped**（红色）— 熔断中，显示 "熔断 · 剩余 Xs" 倒计时
    - **Testing**（黄色）— 半开探测中
  - 状态计算逻辑：
    ```
    function getChannelStatus(item, idx, items, circuitMap, groupMode):
      cb = circuitMap[item.channel_id]
      if cb?.state == "open"    → Tripped（带倒计时）
      if cb?.state == "half_open" → Testing
      // healthy: closed or no entry
      if groupMode == "failover":
        firstHealthyIdx = items.findIndex(i => circuitMap[i.channel_id]?.state != "open")
        if idx == firstHealthyIdx → Running
        else → Ready
      else:
        → Running（所有健康 channel 都参与）
    ```

**Verification（含 Playwright e2e 测试）：**
- 正常状态：第一个 channel 显示 Running，其余显示 Ready
- 手动触发第一个 channel 熔断后：第一个变 Tripped，第二个变 Running
- 验证倒计时每秒更新
- Playwright 测试覆盖以上场景（可通过后端 API 直接写入 circuit state 或 mock）

---

## Execution Order (Milestone 2)

Phase 4 → Phase 5（顺序执行，Phase 5 依赖 Phase 4 的 channel 顺序逻辑）

---

## Milestone v1.2 — Channel Model Management & Group UX

### Phases

- [ ] **Phase 6: Channel 模型管理 UI** - Channel 列表内联展示已配模型，编辑界面支持手动增删
- [ ] **Phase 7: Channel AutoSync** - Sync 按钮拉取上游模型，弹窗选择后写入 CustomModels
- [ ] **Phase 8: Group item 下拉选模型** - 选择 channel 后 model_name 从该 channel 模型列表中下拉选择
- [ ] **Phase 9: Channel & Group 复制** - 一键复制 channel 或 group（含所有配置）

---

## Phase Details

### Phase 6: Channel 模型管理 UI

**Goal:** 管理员可在 Channel 列表中看到每个 channel 已配模型，并可在编辑对话框中手动增删模型。
**Depends on:** Phase 5
**Requirements:** CHN-01, CHN-02
**Success Criteria** (what must be TRUE):
  1. Channel 列表每行内联展示模型 badge 列表（CustomModels ∪ Models，去重）
  2. 打开编辑 channel 对话框，可看到当前模型列表并逐条添加或删除
  3. 保存后模型变更持久化，刷新页面仍生效
  4. 无模型的 channel 显示空状态提示（如 "No models configured"）

**Deliverables:**
- `web/src/app/(dashboard)/channels/page.tsx`：
  - Channel 列表行新增模型 badge 展示列（解析 `custom_models` 字段，逗号分隔）
  - EditChannelDialog 新增 Models 管理区域：显示当前 custom_models，支持删除已有条目、输入新条目后添加
  - 提交时将 models 数组序列化回逗号分隔字符串写入 `custom_models`
- `internal/server/handler/channel.go`：
  - `UpdateChannel` 已支持 `custom_models` 字段写入，无需新增接口

**Plans:** TBD
**UI hint**: yes

---

### Phase 7: Channel AutoSync

**Goal:** 管理员点击 Sync 按钮后，系统从上游 `/v1/models` 拉取模型列表并弹窗展示，选择后将结果集写入 CustomModels。
**Depends on:** Phase 6
**Requirements:** CHN-03, CHN-04, CHN-05
**Success Criteria** (what must be TRUE):
  1. 点击 Sync 按钮后弹窗出现，展示从上游拉取的模型列表
  2. 弹窗中已在 CustomModels 的模型预先勾选
  3. 点击 Save 后，CustomModels 更新为本次勾选集合（覆盖旧值），弹窗关闭
  4. 上游请求失败时弹窗展示错误信息，不修改现有配置

**Deliverables:**
- `internal/server/handler/channel.go`：
  - 新增 `POST /api/admin/channels/:id/sync-models` 处理器：使用 channel 的 base_urls[0] 和 key 向上游发送 `GET /v1/models`，返回 model id 列表
- `internal/server/server.go`：
  - 注册 sync-models 路由
- `web/src/app/(dashboard)/channels/page.tsx`：
  - Channel 行新增 Sync 按钮
  - SyncModelsDialog：调用 sync-models 接口，展示 checkbox 列表（已有 custom_models 预勾选），Save 后调用 `PUT /api/admin/channels/:id` 更新 custom_models

**Plans:** TBD
**UI hint**: yes

---

### Phase 8: Group item 下拉选模型

**Goal:** 管理员在 Group 添加/编辑 item 时，选完 channel 后 model_name 字段自动变为下拉框，选项来自该 channel 的已配模型列表；无模型时降级为文本输入。
**Depends on:** Phase 6
**Requirements:** GRP-01, GRP-02
**Success Criteria** (what must be TRUE):
  1. 在 Group dialog 中选择一个已配模型的 channel 后，model_name 字段立即变为下拉框，选项为该 channel 的 custom_models ∪ models（去重）
  2. 选择无模型 channel 时，model_name 字段保持文本输入框（可手填）
  3. 通过下拉选择的 model_name 正确保存并在 group 配置中回显

**Deliverables:**
- `web/src/app/(dashboard)/groups/page.tsx`：
  - channel 下拉选中后，从 channels 列表数据中查找对应 channel 的 custom_models/models 字段
  - 若模型列表非空，model_name 渲染为 `<Select>` 组件；若为空，渲染为 `<Input>`
  - channels 列表数据在 Group 页面加载时一并获取（已有 `/api/admin/channels` 接口）

**Plans:** TBD
**UI hint**: yes

---

### Phase 9: Channel & Group 复制

**Goal:** 管理员可一键复制 channel 或 group，新记录继承全部配置，名称追加 " (copy)"。
**Depends on:** Phase 6, Phase 8
**Requirements:** CHN-06, GRP-03
**Success Criteria** (what must be TRUE):
  1. 点击 Channel 行的 Duplicate 按钮后，列表立即出现新 channel，名称为原名 + " (copy)"，base_urls、keys、custom_models 与原始一致
  2. 点击 Group 行的 Duplicate 按钮后，列表立即出现新 group，名称为原名 + " (copy)"，items 与原始完全一致
  3. 复制产生的新记录可正常编辑和删除

**Deliverables:**
- `internal/server/handler/channel.go`：
  - 新增 `POST /api/admin/channels/:id/duplicate` 处理器：读取原记录，清除 ID，name 追加 " (copy)"，写入数据库返回新记录
- `internal/server/handler/group.go`：
  - 新增 `POST /api/admin/groups/:id/duplicate` 处理器：读取原记录含 items，清除 ID，name 追加 " (copy)"，写入数据库返回新记录
- `internal/server/server.go`：
  - 注册 duplicate 路由（channel 和 group 各一条）
- `web/src/app/(dashboard)/channels/page.tsx`：
  - Channel 行操作区新增 Duplicate 按钮，调用接口后刷新列表
- `web/src/app/(dashboard)/groups/page.tsx`：
  - Group 行操作区新增 Duplicate 按钮，调用接口后刷新列表

**Plans:** TBD
**UI hint**: yes

---

## Progress Table (Milestone v1.2)

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 6. Channel 模型管理 UI | 0/1 | Not started | - |
| 7. Channel AutoSync | 0/1 | Not started | - |
| 8. Group item 下拉选模型 | 0/1 | Not started | - |
| 9. Channel & Group 复制 | 0/1 | Not started | - |

## Execution Order (Milestone v1.2)

Phase 6 → Phase 7 → Phase 8 → Phase 9

Phase 7 依赖 Phase 6（Sync 结果写入 CustomModels，列表展示需先有 UI 框架）。
Phase 8 依赖 Phase 6（模型下拉来自 channel 的 custom_models 字段）。
Phase 9 依赖 Phase 6 和 Phase 8（复制后的 channel 模型展示、group items 需要前序 UI 就绪）。

---
*Updated: 2026-05-22 — Milestone v1.2 added (Phases 6-9)*
