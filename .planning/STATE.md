# STATE

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-22)

**Core value:** 请求始终路由到正确的 channel，失败时自动退避，管理员能实时看到每个 channel 的健康状态。
**Current focus:** Milestone v1.2 — Channel Model Management & Group UX

## Current Status

- [x] Phase 1: Fix Session Stickiness Bug ✓
- [x] Phase 2: Group 页面 Channel 状态增强 ✓
- [x] Phase 3: Logs 页面增强 ✓
- [x] Phase 4: Group 配置简化 ✓
- [x] Phase 5: Channel 状态语义化 ✓
- [ ] Phase 6: Channel 模型管理 UI
- [ ] Phase 7: AutoSync（选择性合并）
- [ ] Phase 8: Group item 下拉选模型
- [ ] Phase 9: Channel & Group 复制

## Current Position

Phase: 6 — Channel 模型管理 UI
Plan: —
Status: Ready to plan
Last activity: 2026-05-22 — Milestone v1.2 roadmap created

## Next Action

Run `/gsd:plan-phase 6` to start Phase 6.

## Accumulated Context

### Key decisions (v1.2)

- CustomModels 字段（`custom_models`，逗号分隔）为 Channel 模型管理的唯一写入目标；`models` 字段为上游只读来源，展示时合并去重但不修改
- AutoSync 写入语义：本次选中集合完全覆盖 CustomModels（非追加），UI 预勾选已有记录给用户审阅机会
- Group item 模型下拉数据源：`custom_models ∪ models`（channel 级去重），前端从已有 channels 列表接口取，无需新接口
- Duplicate 实现在后端：读原记录 → 清 ID → 改 name → 插入，前端仅触发并刷新列表
