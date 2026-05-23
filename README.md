<div align="right">
  <a href="#english">English</a> | <a href="#中文">中文</a>
</div>

---

<h2 id="english">llmux — LLM API Gateway</h2>

A lightweight LLM API gateway that proxies and load-balances requests across multiple upstream providers (OpenAI, Anthropic, Gemini, and compatible APIs). Single binary, SQLite-backed, ships with a built-in web dashboard.

### Features

- **Multi-provider routing** — OpenAI, Anthropic, Gemini, and any OpenAI-compatible endpoint
- **Groups with load balancing** — round-robin, random, failover, weighted, least-cost, least-latency
- **Circuit breaker** — automatic failover on upstream errors with configurable threshold and reset timeout
- **Session stickiness** — route repeated requests from the same session to the same channel
- **Channel model management** — sync models from upstream `/v1/models`, manage custom model lists
- **API key management** — create scoped API keys with RPM/TPM rate limits
- **Request audit log** — full request/response logging with retention policy
- **Real-time dashboard** — channel health, circuit state, token/cost stats

### Quick Start

**Requirements:** Go 1.23+, Node.js 18+ (for frontend build)

```bash
# Clone
git clone https://github.com/liuguoyuan/llmux.git
cd llmux

# Build frontend (embedded into binary)
cd web && pnpm install && pnpm build && cd ..

# Build binary
go build -o llmux .

# Run (creates data/ with default config on first run)
./llmux start
```

Open `http://localhost:9090` — default login: `admin` / `admin`

> **Change the default password and `jwt_secret` in `data/config.yaml` before exposing to a network.**

### Configuration

`data/config.yaml` is auto-created on first run. Key fields:

```yaml
server:
  host: "0.0.0.0"
  port: 9090
auth:
  jwt_secret: "change-me"       # JWT signing secret
  admin_user: admin
  admin_pass: admin              # Change this
  key_prefix: "sk-llmux-"
circuit:
  threshold: 3                   # failures before tripping
  reset_timeout_sec: 60
log:
  audit_retention_days: 30
```

### Project Structure

```
.
├── cmd/              # CLI entry point (start command)
├── internal/
│   ├── config/       # Config loading
│   ├── gateway/      # Core routing, circuit breaker, session store
│   ├── model/        # GORM models (Channel, Group, APIKey, ...)
│   ├── server/       # HTTP server, handlers, middleware
│   ├── task/         # Background tasks (model price sync, audit cleanup)
│   └── transformer/  # Request/response format adapters (OpenAI ↔ Anthropic)
└── web/              # Next.js dashboard (static export, embedded into binary)
```

### API Usage

Use llmux as an OpenAI-compatible proxy. Point your client to `http://your-host:9090` with a group name as the model:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:9090/v1",
    api_key="sk-llmux-your-key",
)

response = client.chat.completions.create(
    model="my-group",   # routes to the group named "my-group"
    messages=[{"role": "user", "content": "Hello"}],
)
```

### License

MIT

---

<h2 id="中文">llmux — LLM API 网关</h2>

轻量级 LLM API 网关，支持将请求代理并负载均衡到多个上游提供商（OpenAI、Anthropic、Gemini 及兼容接口）。单一二进制文件，SQLite 存储，内置 Web 管理面板。

### 功能特性

- **多提供商路由** — 支持 OpenAI、Anthropic、Gemini 及所有 OpenAI 兼容接口
- **Group 负载均衡** — 轮询、随机、故障转移、加权、最低成本、最低延迟
- **熔断器** — 上游出错时自动切换，支持自定义阈值和重置超时
- **会话粘滞** — 同一会话的请求持续路由到同一 channel
- **Channel 模型管理** — 从上游 `/v1/models` 同步模型，支持自定义模型列表
- **API Key 管理** — 创建带 RPM/TPM 限速的 API Key
- **请求审计日志** — 完整的请求/响应记录，支持定期清理
- **实时仪表盘** — Channel 健康状态、熔断状态、Token/费用统计

### 快速开始

**依赖：** Go 1.23+，Node.js 18+（用于构建前端）

```bash
# 克隆仓库
git clone https://github.com/liuguoyuan/llmux.git
cd llmux

# 构建前端（会被嵌入二进制）
cd web && pnpm install && pnpm build && cd ..

# 编译
go build -o llmux .

# 运行（首次运行自动创建 data/ 目录和默认配置）
./llmux start
```

打开 `http://localhost:9090`，默认账号：`admin` / `admin`

> **在对外暴露服务前，请修改 `data/config.yaml` 中的默认密码和 `jwt_secret`。**

### 配置说明

首次运行会自动生成 `data/config.yaml`，主要配置项：

```yaml
server:
  host: "0.0.0.0"
  port: 9090
auth:
  jwt_secret: "请修改此项"      # JWT 签名密钥
  admin_user: admin
  admin_pass: admin             # 请修改此项
  key_prefix: "sk-llmux-"
circuit:
  threshold: 3                  # 触发熔断的连续失败次数
  reset_timeout_sec: 60         # 熔断重置超时（秒）
log:
  audit_retention_days: 30      # 审计日志保留天数
```

### 项目结构

```
.
├── cmd/              # CLI 入口（start 命令）
├── internal/
│   ├── config/       # 配置加载
│   ├── gateway/      # 核心路由、熔断器、会话存储
│   ├── model/        # GORM 数据模型（Channel、Group、APIKey 等）
│   ├── server/       # HTTP 服务器、Handler、中间件
│   ├── task/         # 后台任务（模型价格同步、审计日志清理）
│   └── transformer/  # 请求/响应格式转换（OpenAI ↔ Anthropic）
└── web/              # Next.js 管理面板（静态导出，嵌入二进制）
```

### 使用方式

将 llmux 作为 OpenAI 兼容代理，客户端指向 `http://your-host:9090`，model 字段填写 Group 名称：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:9090/v1",
    api_key="sk-llmux-your-key",
)

response = client.chat.completions.create(
    model="my-group",   # 路由到名为 "my-group" 的分组
    messages=[{"role": "user", "content": "你好"}],
)
```

### 开源协议

MIT
