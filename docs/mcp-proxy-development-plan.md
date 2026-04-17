# MCP 代理聚合开发方案

## 1. 背景与目标

### 1.1 现状

Octopus 目前是一个 LLM API 聚合与负载均衡服务，支持 OpenAI Chat/Responses/Embedding、Anthropic、Gemini、火山引擎、GitHub Copilot 等多种渠道的协议转换和负载均衡。

### 1.2 问题

随着 MCP（Model Context Protocol）的快速普及，各 Coding 工具（Cursor、Claude Code、Codex、Windsurf 等）都提供了专属的 MCP Server，但这些 Server 存在使用痛点：

- **客户端资源占用**：每个 stdio MCP Server 都需要客户端启动一个子进程，内存和进程管理开销大
- **Token 分散管理**：各厂商的 MCP Server 需要各自的 Token/Key，用户需要分别配置和维护
- **缺乏统一管控**：无法统一监控、限流、审计 MCP 工具调用

### 1.3 目标

在 Octopus 中新增 **MCP 代理聚合模块**，实现：

1. **MCP 流量代理**：Octopus 作为 MCP Streamable HTTP Server 对外暴露，后端连接多个上游 MCP Server，统一鉴权和路由
2. **stdio→HTTP 转换**：将仅支持 stdio 的 MCP Server 桥接为 Streamable HTTP，客户端不再需要本地启动子进程
3. **工具聚合**：聚合多个上游 MCP Server 的工具列表，统一暴露给客户端
4. **统一鉴权**：复用 Octopus 现有 API Key 体系，一个 Token 访问所有 MCP 服务

## 2. 整体架构

### 2.1 架构图

```
                     ┌───────────────────────────────────────┐
                     │            Octopus (Go)                │
                     │                                       │
  MCP Client ───────►│  ┌────────────────────────────┐      │
  (Streamable HTTP)  │  │  MCP Inbound Handler        │      │
  Bearer: sk-xxx     │  │  (Streamable HTTP Server)   │      │
                     │  └─────────────┬──────────────┘      │
                     │                │                      │
                     │  ┌─────────────▼──────────────┐      │
                     │  │  MCP Session Manager         │      │
                     │  │  - 会话生命周期管理            │      │
                     │  │  - 工具聚合与路由             │      │
                     │  │  - API Key 鉴权              │      │
                     │  └─────────────┬──────────────┘      │
                     │                │                      │
                     │     ┌──────────┼──────────┐           │
                     │     ▼          ▼          ▼           │
                     │  ┌───────┐ ┌───────┐ ┌───────┐       │
                     │  │stdio  │ │stdio  │ │ HTTP  │       │
                     │  │Bridge │ │Bridge │ │Client │       │
                     │  └───┬───┘ └───┬───┘ └───┬───┘       │
                     └──────┼─────────┼─────────┼───────────┘
                            │         │         │
                       ┌────▼───┐ ┌───▼────┐ ┌──▼─────────┐
                       │ 子进程  │ │子进程   │ │远程 HTTP    │
                       │MCP Srv │ │MCP Srv │ │MCP Server  │
                       └────────┘ └────────┘ └────────────┘
```

### 2.2 与现有模块的关系

| 现有模块 | MCP 新增模块 | 关系 |
|----------|-------------|------|
| Channel + Outbound | MCPServer + MCPTransport | 平行关系，MCP 是新的协议层 |
| Group + Balancer | MCPGroup + ToolRouter | 思路类似，路由基于 tool name 前缀 |
| API Key Auth | 完全复用 | MCP 端点复用现有 API Key 鉴权体系 |
| Web 管理面板 | 新增 MCP 管理页 | 新增 Tab，UI 风格复用 |
| Stats | 扩展 | 新增 MCP 调用统计 |

### 2.3 关键协议：MCP Streamable HTTP

MCP 使用 JSON-RPC 2.0 编码，支持两种传输：

- **stdio**：通过标准输入/输出通信，客户端启动子进程
- **Streamable HTTP**：通过 HTTP POST/GET 通信，服务端可返回 SSE 流

Streamable HTTP 核心交互：

| 方法 | 用途 |
|------|------|
| `POST /mcp` | 客户端发送 JSON-RPC 消息（请求/通知/响应） |
| `GET /mcp` | 客户端打开 SSE 监听服务端推送 |
| `DELETE /mcp` | 客户端关闭会话 |

会话通过 `Mcp-Session-Id` 头标识。

核心 JSON-RPC 方法：

| 方法 | 说明 |
|------|------|
| `initialize` | 能力协商和协议版本协商 |
| `notifications/initialized` | 客户端确认初始化完成 |
| `tools/list` | 列出可用工具 |
| `tools/call` | 调用工具 |
| `resources/list` | 列出可用资源 |
| `resources/read` | 读取资源 |
| `prompts/list` | 列出可用提示模板 |
| `prompts/get` | 获取提示模板 |

## 3. 数据模型

### 3.1 MCP Server 表

```go
type MCPTransportType int

const (
    MCPTransportStdio          MCPTransportType = 0 // stdio 子进程
    MCPTransportStreamableHTTP MCPTransportType = 1 // Streamable HTTP 远程服务
    MCPTransportSSE            MCPTransportType = 2 // SSE (旧版 HTTP+SSE)
)

type MCPServer struct {
    ID        int              `json:"id" gorm:"primaryKey"`
    Name      string           `json:"name" gorm:"unique;not null"`
    Transport MCPTransportType `json:"transport"`
    Enabled   bool             `json:"enabled" gorm:"default:true"`

    // stdio 配置
    Command  *string           `json:"command"`       // 如 "npx", "uvx", "node"
    Args     StringSlice       `json:"args"`          // 如 ["-y", "@anthropic/mcp-server-filesystem"]
    Env      JSONMap           `json:"env"`           // 环境变量 {"KEY": "value"}
    WorkDir  *string           `json:"work_dir"`      // 工作目录

    // HTTP 配置
    BaseURL  *string           `json:"base_url"`      // 如 "https://mcp.example.com/mcp"
    Headers  JSONMap           `json:"headers"`       // 认证头 {"Authorization": "Bearer xxx"}

    // 工具名前缀（避免不同 Server 的工具名冲突）
    ToolPrefix string          `json:"tool_prefix"`   // 如 "cursor", "copilot"

    // 健康检查
    HealthCheckInterval int    `json:"health_check_interval"` // 秒，默认 30
    MaxRestartCount     int    `json:"max_restart_count"`     // 最大重启次数，默认 3
}
```

### 3.2 MCP Server 统计表

```go
type StatsMCPServer struct {
    ID               int   `json:"id" gorm:"primaryKey"`
    MCPServerID      int   `json:"mcp_server_id"`
    ToolCallCount    int64 `json:"tool_call_count"`
    ToolCallSuccess  int64 `json:"tool_call_success"`
    ToolCallFailed   int64 `json:"tool_call_failed"`
    LastActiveTime   int64 `json:"last_active_time"`
    ProcessStartTime int64 `json:"process_start_time"` // stdio 进程启动时间
    RestartCount     int   `json:"restart_count"`
}
```

### 3.3 MCP 调用日志表

```go
type MCPLog struct {
    ID          int    `json:"id" gorm:"primaryKey"`
    APIKeyID    int    `json:"api_key_id"`
    MCPServerID int    `json:"mcp_server_id"`
    Method      string `json:"method"`       // tools/call, tools/list
    ToolName    string `json:"tool_name"`
    Duration    int64  `json:"duration"`      // 毫秒
    Success     bool   `json:"success"`
    Error       string `json:"error,omitempty"`
    CreatedAt   int64  `json:"created_at"`
}
```

## 4. 模块设计

### 4.1 目录结构

```
internal/
├── mcp/                          # MCP 代理模块（新增）
│   ├── inbound/                  # 入站处理（Streamable HTTP Server）
│   │   ├── handler.go            # HTTP 端点处理（POST/GET/DELETE /mcp）
│   │   ├── session.go            # 会话管理
│   │   └── transport.go          # Streamable HTTP 传输层实现
│   ├── outbound/                 # 出站处理（连接上游 MCP Server）
│   │   ├── stdio.go              # stdio 桥接（子进程管理）
│   │   ├── http.go               # Streamable HTTP Client
│   │   ├── registry.go           # 出站适配器注册
│   │   └── process_manager.go    # 子进程生命周期管理
│   ├── router/                   # 工具路由
│   │   ├── router.go             # 基于 tool prefix 的路由
│   │   └── aggregator.go         # 工具/资源/提示聚合
│   ├── model/                    # 数据模型
│   │   └── mcp_server.go
│   ├── op/                       # 数据库操作
│   │   └── mcp_server.go
│   └── handler/                  # 管理 API
│       └── mcp_server.go
```

### 4.2 MCP Inbound Handler

对外暴露 Streamable HTTP 端点，实现 MCP 协议的服务端传输层：

```
POST /mcp          → 处理客户端 JSON-RPC 请求
GET  /mcp          → 打开 SSE 流推送服务端消息
DELETE /mcp        → 关闭会话
```

核心职责：

- 解析 JSON-RPC 消息
- 维护 `Mcp-Session-Id` 会话映射
- 根据 JSON-RPC method 分发到对应处理器
- 将响应通过 JSON 或 SSE 返回客户端

### 4.3 MCP Session Manager

管理每个客户端连接对应的会话状态：

```go
type MCPSession struct {
    SessionID     string
    APIKeyID      int
    Initialized   bool
    ClientCaps    *mcp.ClientCapabilities
    ServerCaps    *mcp.ServerCapabilities
    CreatedAt     time.Time
    LastActiveAt  time.Time
    UpstreamConns map[int]*UpstreamConnection  // MCPServerID → 连接
}

type SessionManager struct {
    sessions sync.Map  // sessionID → *MCPSession
    mu       sync.RWMutex
}
```

会话生命周期：

```
1. 客户端 POST initialize → 创建 Session，返回 Mcp-Session-Id
2. 客户端 POST notifications/initialized → 标记会话就绪
3. 客户端正常使用 tools/list, tools/call 等
4. 客户端 DELETE /mcp → 清理会话和上游连接
```

### 4.4 stdio→HTTP Bridge

将 stdio MCP Server 桥接为内部可调用的接口：

```go
type StdioBridge struct {
    serverID  int
    command   string
    args      []string
    env       map[string]string
    workDir   string

    process   *os.Process
    stdin     io.WriteCloser
    stdout    io.ReadCloser
    stderr    io.ReadCloser

    mu        sync.Mutex
    pending   map[jsonrpc.ID]chan *jsonrpc.Response  // 等待中的请求
}
```

核心流程：

1. **启动**：执行 command + args，建立 stdin/stdout 管道
2. **请求**：将 JSON-RPC 消息写入子进程 stdin
3. **响应**：从子进程 stdout 读取 JSON-RPC 响应，匹配 ID 返回
4. **重启**：子进程崩溃时自动重启，最多重试 maxRestartCount 次

### 4.5 HTTP MCP Client

连接远程 Streamable HTTP MCP Server：

```go
type HTTPMCPClient struct {
    serverID   int
    baseURL    string
    headers    map[string]string
    sessionID  string  // 上游的 Mcp-Session-Id
    httpClient *http.Client
}
```

核心流程：

1. 发送 `initialize` 到上游，获取 `Mcp-Session-Id`
2. 后续请求携带 `Mcp-Session-Id`
3. 处理 SSE 流式响应

### 4.6 工具聚合与路由

将多个上游 MCP Server 的工具聚合为统一列表：

**工具列表聚合**（`tools/list`）：

```
上游 A (prefix: "cursor")  →  cursor__search, cursor__edit_file
上游 B (prefix: "copilot") →  copilot__code_search, copilot__suggest
聚合结果                    →  以上四个工具统一返回给客户端
```

**工具调用路由**（`tools/call`）：

```
客户端调用 cursor__search
    → 解析前缀 "cursor"
    → 路由到上游 A
    → 去掉前缀，调用 "search"
    → 返回结果
```

```go
type ToolRouter struct {
    servers map[string]*UpstreamConnection  // prefix → upstream
}

func (r *ToolRouter) ListTools(ctx context.Context) ([]Tool, error)
func (r *ToolRouter) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error)
```

## 5. API 设计

### 5.1 MCP 代理端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/mcp` | MCP Streamable HTTP 入口 |
| GET | `/mcp` | SSE 服务端推送通道 |
| DELETE | `/mcp` | 关闭 MCP 会话 |

鉴权方式：`Authorization: Bearer sk-octopus-xxx`（复用现有 API Key）

### 5.2 MCP Server 管理 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/mcp-server` | 列出所有 MCP Server |
| POST | `/api/v1/mcp-server` | 创建 MCP Server |
| PUT | `/api/v1/mcp-server/:id` | 更新 MCP Server |
| DELETE | `/api/v1/mcp-server/:id` | 删除 MCP Server |
| POST | `/api/v1/mcp-server/:id/start` | 启动 MCP Server |
| POST | `/api/v1/mcp-server/:id/stop` | 停止 MCP Server |
| POST | `/api/v1/mcp-server/:id/restart` | 重启 MCP Server |
| GET | `/api/v1/mcp-server/:id/tools` | 预览 MCP Server 的工具列表 |
| GET | `/api/v1/mcp-server/:id/status` | 获取 MCP Server 运行状态 |

### 5.3 客户端配置示例

**Cursor**：

```json
{
  "mcpServers": {
    "octopus": {
      "type": "streamableHttp",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "Authorization": "Bearer sk-octopus-P48ROljwJmWBYVARjwQM8Nkiezlg7WOrXXOWDYY8TI5p9Mzg"
      }
    }
  }
}
```

**Claude Code**（`~/.claude/settings.json`）：

```json
{
  "mcpServers": {
    "octopus": {
      "type": "streamableHttp",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "Authorization": "Bearer sk-octopus-P48ROljwJmWBYVARjwQM8Nkiezlg7WOrXXOWDYY8TI5p9Mzg"
      }
    }
  }
}
```

## 6. Web 管理界面

### 6.1 新增页面

在现有管理面板新增 **MCP 服务** Tab 页，包含：

**MCP Server 列表页**：
- 展示所有 MCP Server 及其状态（运行中/已停止/异常）
- 支持启停、编辑、删除操作
- 显示工具数量和最近调用统计

**MCP Server 编辑页**：
- 基本配置（名称、传输类型、启停状态）
- stdio 配置（命令、参数、环境变量、工作目录）
- HTTP 配置（URL、认证头）
- 工具前缀配置
- 健康检查配置

**MCP Server 详情页**：
- 工具列表预览（工具名、描述、参数 Schema）
- 调用统计（调用次数、成功率、平均耗时）
- 运行日志

### 6.2 页面扩展

- 首页 Dashboard：新增 MCP 调用统计卡片
- API Key 管理：新增 MCP 访问权限配置

## 7. 开发阶段

### Phase 1：最小可用版本

**目标**：单个 MCP Server 能被代理访问

**工作内容**：

| 任务 | 说明 | 涉及文件 |
|------|------|----------|
| 数据模型与迁移 | 创建 `mcp_servers`、`stats_mcp_servers`、`mcp_logs` 表 | `internal/model/`, `internal/db/migrate/` |
| Streamable HTTP Inbound | 实现 `/mcp` 端点，处理 POST/GET/DELETE | `internal/mcp/inbound/` |
| JSON-RPC 消息处理 | 解析/分发 JSON-RPC 请求 | `internal/mcp/inbound/handler.go` |
| stdio Bridge | 启动子进程，stdin/stdout 双向通信 | `internal/mcp/outbound/stdio.go` |
| Session Manager | 会话创建、维护、清理 | `internal/mcp/inbound/session.go` |
| API Key 鉴权 | MCP 端点复用现有 API Key 中间件 | `internal/server/middleware/auth.go` |
| 核心 JSON-RPC 方法 | `initialize`、`tools/list`、`tools/call` | `internal/mcp/inbound/handler.go` |
| 管理 API | CRUD + 启停 MCP Server | `internal/mcp/handler/` |
| 前端基础页面 | MCP Server 列表 + 编辑 | `web/app/` |

**验收标准**：
- 通过配置一个 stdio MCP Server（如 `@anthropic/mcp-server-filesystem`），客户端能通过 Octopus 的 `/mcp` 端点发现并调用其工具
- API Key 鉴权生效

### Phase 2：多上游聚合

**目标**：支持多个 MCP Server 的工具聚合和路由

**工作内容**：

| 任务 | 说明 |
|------|------|
| 工具聚合 | 聚合多个上游的 `tools/list`，加前缀避免冲突 |
| 工具路由 | `tools/call` 根据前缀路由到对应上游 |
| HTTP MCP Client | 连接远程 Streamable HTTP MCP Server |
| 子进程生命周期 | 进程管理器，统一管理所有 stdio 子进程 |
| 前端完善 | 工具列表预览、MCP Server 状态展示 |

**验收标准**：
- 配置 2+ 个 MCP Server，客户端 `tools/list` 能看到聚合后的工具列表
- `tools/call` 能正确路由到对应上游

### Phase 3：生产化

**目标**：稳定性、可观测性、高级特性

**工作内容**：

| 任务 | 说明 |
|------|------|
| 健康检查 | 定期 ping 上游，自动摘除不健康的 Server |
| 自动重启 | 子进程崩溃后自动重启，超过阈值标记异常 |
| 调用统计 | MCP 工具调用次数、耗时、成功率统计 |
| 调用日志 | 记录每次 tools/call 的详情 |
| 超时与熔断 | 工具调用超时控制、连续失败熔断 |
| 资源/提示聚合 | 支持 `resources/list`、`prompts/list` 聚合 |
| Dashboard 扩展 | 首页展示 MCP 统计 |
| API Key 细粒度控制 | 指定 API Key 可访问的 MCP Server 范围 |

**验收标准**：
- 子进程崩溃后能自动恢复
- 统计和日志数据准确
- 工具调用超时不阻塞其他请求

## 8. 技术选型

| 组件 | 方案 | 说明 |
|------|------|------|
| MCP SDK | 自研 JSON-RPC + SSE 实现 | 比依赖第三方 SDK 更轻量可控，Octopus 已有成熟的 HTTP/SSE 处理能力 |
| 子进程管理 | `os/exec` + `syscall` | Go 标准库足够，无需额外依赖 |
| 会话存储 | 内存 + 可选持久化 | MCP 会话是临时的，内存存储即可 |
| 工具缓存 | 内存缓存 | `tools/list` 结果缓存，定期刷新 |
| 路由 | 基于 tool prefix 的 map 查找 | 简单高效 |

## 9. 关键风险与应对

| 风险 | 影响 | 应对策略 |
|------|------|----------|
| stdio 子进程不稳定 | 工具调用失败 | 健康检查 + 自动重启 + 超时熔断 + 多次重试 |
| MCP 协议快速迭代 | 兼容性问题 | 先支持核心子集（initialize + tools），按需扩展 resources/prompts |
| 有状态会话管理 | 内存泄漏、会话残留 | 会话超时自动清理、限制最大会话数 |
| 内存开销从客户端转移到服务端 | 服务端资源压力 | 用户可按需启用、配置最大子进程数 |
| 工具调用安全风险 | 文件操作、API 调用等副作用 | API Key 权限控制 + 审计日志 + 工具白名单 |
| 并发会话竞争 | 多客户端同时调用同一上游 | 上游连接池化，按需复用 stdio 进程或为每个会话独立进程 |

## 10. 配置示例

### 10.1 全局配置

`data/config.json` 新增：

```json
{
  "mcp": {
    "enabled": true,
    "max_sessions": 100,
    "session_timeout": 3600,
    "max_subprocesses": 20,
    "default_health_check_interval": 30,
    "default_max_restart_count": 3,
    "tool_call_timeout": 60
  }
}
```

### 10.2 MCP Server 配置示例

**stdio 类型（Cursor FileSystem MCP Server）**：

```json
{
  "name": "cursor-filesystem",
  "transport": 0,
  "enabled": true,
  "command": "npx",
  "args": ["-y", "@anthropic/mcp-server-filesystem", "/home/user/projects"],
  "env": {},
  "tool_prefix": "cursor"
}
```

**stdio 类型（百炼 MCP Server）**：

```json
{
  "name": "dashscope-mcp",
  "transport": 0,
  "enabled": true,
  "command": "npx",
  "args": ["-y", "@alicloud/dashscope-mcp-server"],
  "env": {
    "DASHSCOPE_API_KEY": "sk-xxxxx"
  },
  "tool_prefix": "dashscope"
}
```

**Streamable HTTP 类型（远程 MCP Server）**：

```json
{
  "name": "remote-mcp",
  "transport": 1,
  "enabled": true,
  "base_url": "https://mcp.example.com/mcp",
  "headers": {
    "Authorization": "Bearer upstream-token-xxx"
  },
  "tool_prefix": "remote"
}
```

## 11. 数据库迁移

新增迁移文件 `internal/db/migrate/00x_mcp.go`：

```go
func init() {
    migrations = append(migrations, Migration{
        ID:   "00x_mcp",
        Desc: "Add MCP server related tables",
        Up: func(db *gorm.DB) error {
            return db.AutoMigrate(
                &model.MCPServer{},
                &model.StatsMCPServer{},
                &model.MCPLog{},
            )
        },
        Down: func(db *gorm.DB) error {
            return db.Migrator().DropTable(
                "mcp_servers",
                "stats_mcp_servers",
                "mcp_logs",
            )
        },
    })
}
```
