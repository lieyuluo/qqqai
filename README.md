# QQQAI

QQQAI 是一个基于 Go、GoFrame、CloudWeGo Eino、RAG 和 OneBot 的多端智能 Agent 项目。它同时支持 QQ OneBot 机器人接入和 Web 聊天控制台，Web 端与 QQ 端复用同一套 `ChatService` 和 Eino `FinalGraph`，让普通聊天、RAG 文件问答、SQL 生成与安全执行都走统一的核心能力。

## 1. 项目定位

QQQAI 的定位是一个可扩展的多端 AI 助手系统：

- QQ 场景：通过 NapCat / OneBot WebSocket 接入 QQ 群聊和私聊，保留原有 `/ws` 能力。
- Web 场景：通过 GoFrame Web API + Next.js 前端提供登录、会话、聊天、文件上传、RAG 索引、SQL 工具和管理员统计。
- Agent 核心：通过 CloudWeGo Eino `FinalGraph` 在普通聊天、RAG、SQL 分析之间自动路由。
- 高并发执行：通过 goroutine、channel、worker pool 控制多用户 AI 请求和文件索引任务。

项目重点不是简单聊天，而是把 QQ Bot、Web Chat、RAG、SQL、安全执行和异步任务调度整合成一个统一后端。

## 2. 系统架构

```text
                       ┌────────────────────┐
                       │    Next.js Web     │
                       │ login/chat/files   │
                       └─────────┬──────────┘
                                 │ HTTP / SSE
                                 ▼
┌──────────────┐        ┌────────────────────┐
│ QQ OneBot WS │───────▶│  GoFrame Server    │
│    /ws       │        │ /api/* + /ws       │
└──────────────┘        └─────────┬──────────┘
                                  │
                                  ▼
                         ┌──────────────────┐
                         │   ChatService    │
                         │ unified gateway  │
                         └────────┬─────────┘
                                  │
                         ┌────────▼─────────┐
                         │ Eino FinalGraph  │
                         │ Chat / RAG / SQL │
                         └─────┬─────┬──────┘
                               │     │
                 ┌─────────────┘     └─────────────┐
                 ▼                                 ▼
       ┌──────────────────┐              ┌──────────────────┐
       │ RAG Index/Retrieve│             │ SQL Generate/Exec │
       │ Milvus + ES       │             │ SELECT Guard + MCP│
       └──────────────────┘              └──────────────────┘

              ┌────────────────────────────────────┐
              │ MySQL: users/conversations/messages │
              │ files/model_call_logs/admin stats   │
              └────────────────────────────────────┘
```

核心链路：

- Web 用户发送消息：`Next.js -> /api/chat 或 /api/chat/stream -> ChatService -> FinalGraph`。
- QQ 用户发送消息：`OneBot /ws -> handler -> ChatTaskPool -> ChatService -> FinalGraph`。
- 文件上传：`/api/files/upload -> LocalStorage -> FileIndexWorker -> rag_flow.GetIndexingGraph()`。
- SQL 工具：`/api/sql/generate` 只生成，`/api/sql/execute` 经过 SELECT 安全检查后执行。

## 3. 技术栈

后端：

- Go
- GoFrame v2
- CloudWeGo Eino Graph
- gorilla/websocket
- MySQL
- Redis
- Milvus
- Elasticsearch
- MCP MySQL Server
- JWT + bcrypt

前端：

- Next.js App Router
- React
- TypeScript
- SSE / Streaming Response
- lucide-react

AI / RAG：

- Eino `FinalGraph`
- RAG Chat Flow
- RAG Indexing Graph
- Milvus 向量检索
- Elasticsearch 关键词检索
- RRF 融合排序

## 4. Web 端启动方式

### 4.1 后端启动

先复制环境变量：

```bash
cp .env.example .env
```

编辑 `.env` 中的模型、MySQL、Redis、Milvus、Elasticsearch、JWT 和管理员配置。

启动后端：

```bash
go run .
```

默认后端地址：

```text
http://127.0.0.1:8080
```

### 4.2 前端启动

进入 Web 目录：

```bash
cd web
npm install
npm run dev
```

默认前端地址：

```text
http://localhost:3000
```

前端通过以下环境变量访问后端：

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

## 5. QQ OneBot 启动方式

QQ OneBot 接入保留原有 `/ws` 路由。

后端启动后，NapCat / OneBot 连接：

```text
ws://127.0.0.1:8080/ws
```

需要在 `.env` 中配置：

```env
BOT_QQ=你的机器人QQ号
BOT_PORT=8080
WEBSOCKET_ALLOWED_ORIGINS=*
WEBSOCKET_READ_TIMEOUT=300
WEBSOCKET_WRITE_TIMEOUT=10
NAPCAT_HTTP_BASE_URL=http://127.0.0.1:3000
```

群聊中需要 @ 机器人后才会触发处理；私聊会直接处理。OneBot 端不会绕过 Web 端能力，而是和 Web 端一样复用 `ChatService` 与 `FinalGraph`。

## 6. 数据库初始化

项目使用 MySQL 持久化 Web 用户、会话、消息、文件和模型调用日志。

初始化 schema：

```bash
mysql -h 127.0.0.1 -P 3306 -u root -p qqqai < manifest/schema.sql
```

主要数据表：

- `users`：用户、角色、密码哈希。
- `conversations`：Web 会话。
- `messages`：Web 消息记录。
- `files`：上传文件与索引状态。
- `model_call_logs`：聊天、SQL 生成、SQL 执行日志。

管理员账号由后端启动时根据 `.env` 自动创建或更新：

```env
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=change-me
```

## 7. 文件上传与 RAG 索引

Web 端支持上传文件：

```text
POST /api/files/upload
```

当前文件存储流程：

```text
用户上传文件
  -> Storage interface
  -> LocalStorage
  -> uploads/{user_id}/{uuid}_{safe_name}
  -> files.status = pending
  -> FileIndexWorker queue
  -> rag_flow.GetIndexingGraph()
  -> Milvus + Elasticsearch
  -> files.status = indexed / failed
```

重点设计：

1. 文件上传接口只负责保存文件和提交索引任务，不阻塞等待完整索引完成。
2. `FileIndexWorker` 使用 goroutine + channel + worker pool 控制索引并发。
3. 单个文件索引失败只更新该文件 `files.status=failed`，不会影响其他任务。
4. 文件存储抽象为 `Storage interface`，当前实现为 `LocalStorage`。
5. 后续可以扩展为 MinIO、OSS、S3 等对象存储，只需要替换 Storage 实现。

## 8. SQL 安全执行机制

SQL 分为两个阶段：

```text
/api/sql/generate
  -> 只生成 SQL
  -> 返回给用户确认

/api/sql/execute
  -> 要求 confirm=true
  -> 服务端安全检查
  -> 包装 LIMIT
  -> 调用 MCP MySQL 工具执行
```

安全规则：

- 只允许单条 `SELECT`。
- 拒绝多语句。
- 剥离 SQL 注释后再检查。
- 拒绝危险关键字：
  - `INSERT`
  - `UPDATE`
  - `DELETE`
  - `DROP`
  - `ALTER`
  - `TRUNCATE`
  - `CREATE`
  - `GRANT`
  - `REVOKE`
  - `CALL`
  - `LOAD`
  - `OUTFILE`
  - `DUMPFILE`
  - `FOR UPDATE`
- 执行前包装：

```sql
SELECT * FROM (<user_select_sql>) AS qqqai_safe LIMIT SQL_MAX_ROWS
```

行数上限由 `.env` 控制：

```env
SQL_MAX_ROWS=100
```

这个设计保证模型不能直接执行写操作，也不能绕过后端确认和 SELECT 白名单。

## 9. Go 语言优势设计

### 9.1 高并发对话调度

`ChatTaskPool` 基于 goroutine + channel 实现：

```text
QQ/Web 请求
  -> ChatTask
  -> channel queue
  -> worker goroutines
  -> ChatService
  -> FinalGraph
```

优势：

- 控制多用户 AI 请求并发，避免瞬时请求打爆模型服务。
- channel 队列满时可以快速返回错误。
- worker 数量可配置：

```env
CHAT_WORKER_COUNT=4
CHAT_QUEUE_SIZE=64
```

### 9.2 长连接流式输出

Web 聊天流式接口：

```text
POST /api/chat/stream
```

它使用 SSE / Streaming Response 将 AI 回复增量推给浏览器：

```text
ChatService.Stream()
  -> chunk channel
  -> SSE data event
  -> 浏览器实时追加显示
```

同时 handler 会监听 request context：

```text
客户端断开
  -> ctx.Done()
  -> 停止写入
  -> 结束流式响应
```

### 9.3 RAG 文件并发索引

文件上传后不会同步阻塞索引，而是进入 `FileIndexWorker`：

```text
FileIndexTask
  -> channel queue
  -> worker pool
  -> RAG IndexingGraph
```

优势：

- 多文件上传时可控并发。
- 大文件索引不会阻塞 Web 请求。
- 失败状态可持久化到 MySQL。

配置：

```env
FILE_INDEX_WORKER_COUNT=2
FILE_INDEX_QUEUE_SIZE=32
```

### 9.4 存储扩展

文件存储层使用接口：

```go
type Storage interface {
    Save(ctx context.Context, userID int64, file *multipart.FileHeader) (*StoredFile, error)
    Delete(ctx context.Context, path string) error
}
```

当前实现：

```text
LocalStorage -> uploads/
```

后续扩展：

```text
MinIOStorage
S3Storage
OSSStorage
```

业务层不关心文件最终存储在哪里，只依赖 `Storage interface`。

### 9.5 多端统一

Web 端和 QQ OneBot 端共用：

```text
ChatService
  -> Eino FinalGraph
  -> Chat / RAG / SQL
```

这意味着：

- Web 和 QQ 使用同一套意图识别。
- Web 和 QQ 使用同一套 RAG 问答。
- Web 和 QQ 使用同一套 SQL Agent 核心。
- 后续修改 Agent 能力时不需要分别维护两套逻辑。

## 10. API 列表

### Auth

| Method | Path | 说明 |
|---|---|---|
| `POST` | `/api/auth/register` | 注册 |
| `POST` | `/api/auth/login` | 登录并返回 JWT |
| `GET` | `/api/auth/me` | 获取当前用户 |

### Conversations

| Method | Path | 说明 |
|---|---|---|
| `POST` | `/api/conversations` | 创建会话 |
| `GET` | `/api/conversations` | 会话列表 |
| `GET` | `/api/conversations/{id}/messages` | 消息列表 |
| `DELETE` | `/api/conversations/{id}` | 删除会话 |

### Chat

| Method | Path | 说明 |
|---|---|---|
| `POST` | `/api/chat` | 普通 Web 聊天 |
| `POST` | `/api/chat/stream` | SSE 流式聊天 |

### Files

| Method | Path | 说明 |
|---|---|---|
| `POST` | `/api/files/upload` | 上传文件并提交 RAG 索引 |
| `GET` | `/api/files` | 文件列表 |
| `DELETE` | `/api/files/{id}` | 删除文件记录和本地文件 |

### SQL

| Method | Path | 说明 |
|---|---|---|
| `POST` | `/api/sql/generate` | 生成 SQL，不执行 |
| `POST` | `/api/sql/execute` | 安全检查后执行 SELECT |

### Admin

| Method | Path | 说明 |
|---|---|---|
| `GET` | `/api/admin/stats` | 管理员统计 |
| `GET` | `/api/admin/users` | 用户列表 |
| `GET` | `/api/admin/conversations` | 会话列表 |
| `GET` | `/api/admin/files` | 文件列表 |

### OneBot

| Method | Path | 说明 |
|---|---|---|
| `GET` | `/ws` | OneBot WebSocket 连接 |

## 11. 前端页面说明

Next.js 前端位于 `web/`。

页面：

- `/login`：用户登录，成功后保存 JWT。
- `/register`：用户注册。
- `/chat`：Web 聊天页，包含会话列表、消息区、SSE 流式回复。
- `/files`：文件上传、文件列表、索引状态展示、删除文件。
- `/admin`：管理员统计页面，展示用户数、会话数、消息数、文件数、模型调用数等。

前端请求统一通过 `web/lib/api.ts`：

- 自动附带 Bearer Token。
- 统一处理后端 `{code,message,data,request_id}` 响应。
- 401 时清理 token 并跳转登录。
- `streamChat()` 使用 Fetch Streaming 读取 SSE chunk 并实时追加到消息区。

## 运行检查

后端测试：

```bash
go test ./...
```

前端构建：

```bash
cd web
npm run build
```

## 项目亮点

- 高并发对话调度：`ChatTaskPool` 使用 goroutine + channel 控制 AI 请求并发。
- 长连接流式输出：SSE / Streaming Response 支持 Web 端 AI 回复实时增量显示。
- RAG 文件并发索引：`FileIndexWorker` 使用 worker pool 异步执行索引任务。
- 存储扩展：`Storage interface` 当前接 LocalStorage，后续可扩展 MinIO。
- 多端统一：Web 与 QQ OneBot 共用 `ChatService` 和 Eino `FinalGraph`。
