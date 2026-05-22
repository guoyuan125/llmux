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

### Active

- [ ] Bug: Session stickiness 导致 channel 挪出分组后仍被路由（moveToFront 使用旧 channel ID，未校验该 channel 是否仍在当前 group 的 items 中）
- [ ] 功能: Group 页面 channel 卡片增强 — 熔断剩余次数、熔断恢复倒计时（实时倒数）、服务状态颜色
- [ ] 功能: Logs 页面增强 — error 行红色背景突出显示，展开查看格式化请求/响应全文

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

---
*Last updated: 2026-05-22 after initialization*

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
