# llmux

## What This Is

llmux 是一个本地运行的 LLM API 网关，支持 OpenAI / Anthropic / Gemini 协议的接入与转换，提供多 channel 负载均衡、熔断、审计日志和 Web 管理后台。面向个人开发者和小团队，用于统一管理多个上游 LLM 提供商。

## Core Value

请求始终路由到正确的 channel，失败时自动退避，管理员能实时看到每个 channel 的健康状态。

## Requirements

### Validated

- ✓ OpenAI / Anthropic / Gemini 协议接入与互转 — existing
- ✓ 多 channel 负载均衡（round_robin / random / failover / weighted / least_cost / least_latency）— existing
- ✓ 熔断器（circuit breaker）— existing
- ✓ 审计日志（AuditLog）— existing
- ✓ Web 管理后台（Groups / Channels / Logs / Stats）— existing
- ✓ Session stickiness — existing
- ✓ 优雅重启（tableflip + SIGUSR2）— existing
- ✓ Session stickiness bug 修复 — Milestone 1 Phase 1
- ✓ Group channel 卡片熔断状态展示（threshold 字段、四态 badge、倒计时）— Milestone 1 Phase 2
- ✓ Logs 页面 error 行红色背景 — Milestone 1 Phase 3
- ✓ Group 配置简化（去掉 Priority/Weight 输入，列表顺序自动决定优先级）— Milestone 2 Phase 4
- ✓ Accepted Models 精确匹配（去掉通配符）— Milestone 2 Phase 4
- ✓ Channel 状态语义化（Running / Ready / Tripped / Testing）— Milestone 2 Phase 5
- ✓ Groups 页面布局优化（Channels 列纵向排列，去掉 Accepted Models 列）— Milestone 2 Phase 5

### Active

- [ ] Channel 页面：每个 channel 展示已配模型列表，支持手动增删
- [ ] Channel AutoSync：从上游拉取模型列表，选择性合并（不覆盖）到 CustomModels
- [ ] Group 添加 item：model_name 从下拉框中选（来自 channel 的模型列表）
- [ ] Channel 复制：一键复制 channel（含 base_urls、keys）
- [ ] Group 复制：一键复制 group（含所有 items）

### Out of Scope

- 新协议支持（Gemini 原生、OpenAI Responses API）— 本次不做
- 数据库切换（PostgreSQL 等）— 本次不做

## Context

- Go 后端，SQLite 存储，Gin 框架
- 前端 Next.js + shadcn/ui，静态导出后通过 Go embed.FS 内嵌
- 熔断器状态存在内存（circuit.Manager），已有 `/api/circuit/status` 接口，前端每 5s 轮询
- Session stickiness 存在内存（session.Store），key = (apiKeyID, modelName)，value = channelID
- AuditLog 已有 RequestBody / ResponseBody 字段（仅 error 时存储）

## Constraints

- **Tech**: Go + SQLite + Next.js，不引入新依赖
- **Scope**: 最小改动，不重构无关代码

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Session stickiness 修复方式：在 moveToFront 后校验 channel 是否仍在 group items 中，不在则清除 session | 最小改动，不影响正常 stickiness 逻辑 | — Pending |
| 熔断倒计时在前端实现（JS setInterval 每秒更新）| 后端已返回 next_retry 时间戳，无需新接口 | — Pending |

## Current Milestone: v1.2 — Channel Model Management & Group UX

**Goal:** Channel 成为模型管理中心，Group 配置引用 Channel 模型列表，消除手填错误。

**Target features:**
- Channel 展示和管理已配模型（利用现有 Models + CustomModels 字段）
- AutoSync：从上游拉取模型列表，选择性合并（不覆盖已有）
- Group 添加 item 时从 channel 模型下拉选，不再手填
- Channel 和 Group 均支持一键复制

---
*Last updated: 2026-05-22 — Milestone v1.2 started*

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state
