# QQQAI — 面向 QQ 场景的智能 Agent Bot

> 一个基于 **Go + Eino Graph + NapCat OneBot** 构建的 QQ 智能机器人。  
> 它不只是“能聊天”，而是能够在 **普通对话、文件知识库问答、专业数据分析 SQL 查询** 之间自动切换的多能力 Agent。

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)
![Eino](https://img.shields.io/badge/Eino-Graph%20Workflow-7B61FF?style=flat-square)
![NapCat](https://img.shields.io/badge/NapCat-OneBot-FFB000?style=flat-square)
![RAG](https://img.shields.io/badge/RAG-Milvus%20%2B%20ES-22C55E?style=flat-square)
![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)

---

## ✨ 项目亮点

QQQAI 是一个为 QQ 群聊 / 私聊设计的智能机器人项目，核心目标是：

- **普通聊天**：直接基于短期记忆进行自然回复；
- **文件问答**：自动索引群文件，结合知识库进行 RAG 问答；
- **数据分析**：识别统计、报表、数据库类问题，自动生成 SQL 并通过 MCP 执行；
- **意图分流**：通过意图模型判断问题类型，把请求送到最合适的处理链路；
- **图式编排**：使用 CloudWeGo Eino 的 `compose.Graph` 构建清晰、可扩展的 Agent 工作流。

它适合用来打造：

- QQ 群智能助手
- 私聊 AI 机器人
- 群文件知识库助手
- 企业 / 社群数据问答机器人
- 可扩展的 Go Agent 框架样板

---

## 🧠 核心能力

### 1. 智能意图识别

用户输入会先经过总控图 `FinalGraph` 的意图模型判断，自动分为：

| 意图 | 说明 | 处理路径 |
|---|---|---|
| `SQL` | 数据库查询、SQL 生成、统计分析、报表需求 | SQL React 子图 + MCP |
| `RAG` | 文件、文档、知识库、上传资料相关问答 | RAGChatFlow |
| `Chat` | 普通闲聊、写作、翻译、常识问答 | `ai/chat.go` 短期记忆聊天 |

---

### 2. 普通聊天：轻量、快速、有短期记忆

普通聊天不走 RAG、不走 SQL、不调用 MCP，只使用当前会话的短期历史进行回复。

```text
用户消息
  -> Intent_Model
  -> Chat
  -> ai.GenerateReply()
  -> 返回回复
```

适合：

- 闲聊
- 角色对话
- 简单问答
- 翻译 / 润色 / 创作
- 不需要外部资料的问题

---

### 3. RAG 文件问答：让群文件变成知识库

当 QQ 群上传文件时，系统会尝试：

```text
群文件上传事件
  -> 获取群文件下载地址
  -> 下载文件
  -> 文档解析
  -> 文档切分
  -> 向量化
  -> 写入 Milvus / ES
```

当用户询问文件内容时：

```text
用户问题
  -> Query Rewrite
  -> Milvus 向量检索
  -> Elasticsearch 关键词检索
  -> RRF 融合重排
  -> 拼接上下文
  -> 模型生成回答
```

RAG 检索采用 **Milvus + Elasticsearch 双路召回**，再通过 **RRF** 进行融合排序，兼顾语义召回和关键词精确匹配。

---

### 4. SQL 专业数据对话

对于数据统计、业务指标、报表类需求，系统会走 SQL 专业流程：

```text
用户问题
  -> 意图识别为 SQL
  -> 检索相关表结构 / 业务知识
  -> 生成 SQL
  -> 用户确认
  -> MCP 执行 SQL
  -> 返回结果
```

这种设计避免模型直接执行危险操作，先生成 SQL，再通过确认与工具执行，提高可控性。

---

### 5. 图式工作流编排

项目使用 Eino `compose.Graph` 组织复杂流程，让每条链路都清晰可追踪。

总控图大致如下：

```text
START
  -> Intent_Model
  -> Trans_List
  -> Branch
      ├─ SQL
      │   -> React
      │   -> ToToolCall
      │   -> MCP
      │   -> END
      │
      ├─ RAG
      │   -> RAGChat
      │   -> RAGToEnd
      │   -> END
      │
      └─ Chat
          -> ai.GenerateReply
          -> ChatToEnd
          -> END
```

---

## 🏗️ 项目结构

```text
qqqai
├── adapter/              # OneBot / NapCat 事件适配与消息构造
├── ai/                   # 普通聊天能力，包含短期记忆
├── config/               # 环境变量配置加载
├── flow/                 # Eino Graph 工作流
│   ├── final_graph.go    # 总控图：意图识别与路由
│   ├── sql_react.go      # SQL 生成与审批子图
│   ├── rag_chat.go       # RAG 对话图
│   └── analyst_graph.go  # 数据分析图
├── handler/              # WebSocket 消息入口与 QQ 事件处理
├── model/                # ChatModel / EmbeddingModel 封装
├── rag/                  # RAG 索引、检索、工具
├── tool/                 # MCP、SQL、Memory、Document 等工具
├── main.go               # 服务启动入口
└── .env                  # 运行配置
```

---

## 🚀 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/<your-name>/qqqai.git
cd qqqai
```

### 2. 准备 `.env`

在项目根目录创建 `.env`：

```env
# Bot
BOT_QQ=你的机器人QQ号
BOT_PORT=8080

# 模型选择
CHAT_MODEL_TYPE=deepseek
INTENT_MODEL_TYPE=deepseek
EMBEDDING_MODEL_TYPE=qwen

# 人设
LLM_PERSONA=你是一个专业、友好、反应迅速的 QQ 群智能助手。

# DeepSeek
DEEPSEEK_KEY=你的 DeepSeek API Key
DEEPSEEK_CHAT_MODEL=deepseek-chat
DEEPSEEK_BASE_URL=https://api.deepseek.com

# Qwen Embedding
QWEN_KEY=你的通义千问 API Key
QWEN_EMBEDDING=text-embedding-v3
QWEN_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1

# NapCat HTTP
NAPCAT_HTTP_BASE_URL=http://127.0.0.1:3000

# MySQL
MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
MYSQL_USERNAME=root
MYSQL_PASSWORD=your_password
MYSQL_DATABASE=your_database

# Redis
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0

# Milvus
MILVUS_ADDR=127.0.0.1:19530
MILVUS_USERNAME=
MILVUS_PASSWORD=
MILVUS_COLLECTION_NAME=qqqai_docs
MILVUS_TOP_K=5

# Elasticsearch
ELASTICSEARCH_ADDRESSES=http://127.0.0.1:9200
ELASTICSEARCH_USERNAME=
ELASTICSEARCH_PASSWORD=
ELASTICSEARCH_INDEX=qqqai_docs

# WebSocket
WEBSOCKET_ALLOWED_ORIGINS=*
WEBSOCKET_READ_TIMEOUT=300
WEBSOCKET_WRITE_TIMEOUT=10

# Session
SESSION_MAX_MESSAGES=10

# Memory
MEMORY_DIR=data/memory
MEMORY_SUMMARY_INTERVAL=20
MEMORY_MAX_ENTRIES=20
```

> ⚠️ 不要把真实 API Key、数据库密码、NapCat Token 提交到 GitHub。  
> 建议提交 `.env.example`，并把 `.env` 加入 `.gitignore`。

---

### 3. 启动依赖服务

项目依赖以下组件：

- NapCat / OneBot
- Redis
- MySQL
- Milvus
- Elasticsearch
- DeepSeek / Ark / Qwen 等模型服务

如果你使用 Docker Compose，可以根据自己的环境编排这些服务。

---

### 4. 启动项目

```bash
go mod tidy
go run main.go
```

服务启动后会监听：

```text
/ws
```

NapCat 需要将 OneBot WebSocket 事件转发到该地址。

---

## 🔌 NapCat 接入说明

项目通过 OneBot 事件处理 QQ 消息。

支持：

- 群聊消息
- 私聊消息
- 群文件上传事件

群聊中只有 @ 机器人时才会响应，避免刷屏。

```text
群消息
  -> 判断是否 @ 机器人
  -> 清理 CQ 码
  -> 生成 sessionID
  -> FinalGraph.Invoke()
  -> 回复群消息
```

私聊会直接进入会话处理。

---

## 📂 群文件索引流程

当群成员上传文件后，机器人会尝试自动索引：

```text
group_upload notice
  -> get_group_file_url
  -> download
  -> indexing graph
  -> Milvus / ES
```

常见问题：

### 获取群文件链接 403

如果日志出现：

```text
获取群文件链接接口返回状态码 403
```

通常说明 NapCat HTTP API 拒绝了请求。请检查：

1. NapCat HTTP 地址是否正确；
2. Access Token 是否一致；
3. HTTP API 是否开启；
4. Docker 网络是否能访问 NapCat；
5. 反向代理是否正确透传 Authorization Header。

---

## 🧩 关键工作流

### FinalGraph

总控图，负责意图识别与三路分发：

```text
SQL / RAG / Chat
```

### React SQL Graph

用于数据类问题：

```text
检索表结构
  -> 生成 SQL
  -> 用户确认
  -> MCP 执行
```

### RAGChatFlow

用于文件知识库问答：

```text
读取会话
  -> Query Rewrite
  -> 检索
  -> 构造上下文
  -> 模型回答
  -> 历史保存 / 摘要压缩
```

### RetrieverGraph

混合检索图：

```text
Query
  -> MilvusRetriever
  -> ESRetriever
  -> RRF Reranker
  -> TopK Docs
```

### AI Chat

普通聊天：

```text
Persona
  -> Short-term History
  -> User Message
  -> ChatModel
  -> Save Turn
```

---

## 🛡️ 安全建议

- 不要提交 `.env`；
- 不要在代码中硬编码 Token；
- SQL 执行前建议保留人工确认；
- MCP 工具按能力拆分，不建议把 SQL 工具和天气、时间等通用工具混在一起；
- 群文件下载应限制文件大小和类型；
- WebSocket Origin 根据生产环境设置白名单；
- 数据库账号建议使用只读权限。

---

## 🧪 调试建议

### 打印意图模型输出

可以在 `FinalGraph` 的 `Intent_Model` 节点中打印：

```go
log.Printf("[FinalGraph] intent model output: query=%q, intent=%q", input.Query, intentMsg.Content)
```

### 打印最终路由

可以在 `AddBranch` 中打印：

```go
log.Printf("[FinalGraph] route selected: query=%q, intent=%q, route=%s", query, content, route)
```

这样可以快速判断问题到底被分到了 SQL、RAG 还是 Chat。

---

## 🗺️ Roadmap

- [ ] 增加时间 / 天气 / 新闻等 Tool 分支
- [ ] 群文件索引支持更多格式
- [ ] 管理后台：查看知识库、会话、索引状态
- [ ] SQL 执行权限控制与审计

---

## GoFrame Web API + Next.js Web Console

本仓库现在保留原有 NapCat / OneBot `/ws` 接入，同时新增 GoFrame Web API 层和独立 `web/` Next.js 前端。QQ 端和 Web 端都会通过 `internal/service.ChatService` 进入同一套 Eino `FinalGraph`，因此普通聊天、RAG、SQL 路由能力保持一致。

### 新增能力

- GoFrame Server：启动后同时提供 `/ws` 和 `/api/*`。
- JWT 认证：`POST /api/auth/register`、`POST /api/auth/login`、`GET /api/auth/me`。
- Web 会话与消息持久化：`conversations` / `messages` 表保存 Web 聊天记录。
- Web Chat：`POST /api/chat` 同步返回，`POST /api/chat/stream` 伪流式 SSE 输出，支持请求 context 取消。
- 文件上传：`POST /api/files/upload` 保存到本地 `uploads/`，并提交异步 RAG 索引任务。
- RAG 文件索引 Worker：使用 goroutine + channel worker pool 调用 `rag_flow.GetIndexingGraph()`，更新 `files.status`。
- SQL 安全接口：`/api/sql/generate` 只生成 SQL；`/api/sql/execute` 需要 `confirm=true`，且仅允许单条安全 `SELECT`。
- Admin API：`/api/admin/stats`、`/api/admin/users`、`/api/admin/conversations`、`/api/admin/files`。
- Next.js 前端：登录、注册、聊天、会话列表、流式输出、文件上传、管理员统计。

### 数据库初始化

先创建 MySQL 数据库，然后执行：

```bash
mysql -h 127.0.0.1 -P 3306 -u root -p qqqai < manifest/schema.sql
```

管理员账号由 `.env` 中的 `ADMIN_EMAIL` 和 `ADMIN_PASSWORD` 在后端启动时自动创建或更新。

### 后端运行

```bash
cp .env.example .env
go run .
```

后端默认监听 `BOT_PORT=8080`。OneBot 仍连接：

```text
ws://127.0.0.1:8080/ws
```

Web API 默认地址：

```text
http://127.0.0.1:8080/api
```

### 前端运行

```bash
cd web
npm install
npm run dev
```

前端默认读取：

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

然后打开：

```text
http://localhost:3000
```

### 并发设计

- `ChatTaskPool` 使用固定数量 goroutine 从 channel 消费聊天任务，QQ 端和 Web 同步聊天都复用 `ChatService`。
- `FileIndexWorker` 使用独立 worker pool 异步处理上传文件索引，单个文件失败只更新该文件状态，不阻塞上传接口和其他任务。
- SSE 接口在写入每个 chunk 前检查 request context；客户端断开后停止写入并结束 handler。
- [ ] 多群隔离知识库
- [ ] 支持插件化 MCP 工具注册
- [ ] 支持流式回复
- [ ] 支持 Docker Compose 一键部署

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request。

你可以从这些方向参与：

- 新增模型适配器
- 优化 RAG 检索效果
- 增加 MCP 工具
- 完善 Docker 部署
- 改进提示词和意图识别
- 优化 QQ 群文件处理

---

## 📄 License

本项目建议使用 MIT License。  
如果你需要其他 License，请根据实际使用场景自行调整。

---

## 🌟 一句话介绍

**QQQAI 是一个会聊天、会读文件、会查数据的 QQ 智能 Agent Bot。**

它把 QQ 机器人从“自动回复工具”升级成了一个真正可扩展的多能力智能助理。
