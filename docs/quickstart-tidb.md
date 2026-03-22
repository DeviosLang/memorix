# 快速上手：TiDB Serverless 最简部署方案

本文档覆盖从零到完整工作的全流程：部署 memorix-server、接入 TiDB Serverless 向量数据库、开通租户，最终让 Claude Code 获得持久化云端记忆。

**预计完成时间：15 分钟。**

---

## 目录

1. [方案概览](#1-方案概览)
2. [前置条件](#2-前置条件)
3. [第一步：准备数据库](#3-第一步准备数据库)
   - [方式 A：TiDB Zero 全自动（推荐）](#方式-a-tidb-zero-全自动推荐)
   - [方式 B：已有 TiDB Serverless 手动配置](#方式-b-已有-tidb-serverless-手动配置)
4. [第二步：部署 memorix-server](#4-第二步部署-memorix-server)
5. [第三步：开通租户](#5-第三步开通租户)
6. [第四步：接入 Claude Code](#6-第四步接入-claude-code)
7. [验证端到端](#7-验证端到端)
8. [当前具备的能力](#8-当前具备的能力)
9. [重要注意事项](#9-重要注意事项)
10. [常见问题](#10-常见问题)
11. [进阶配置](#11-进阶配置)

---

## 1. 方案概览

```
Claude Code
    ↕ hooks（session-start / stop / user-prompt-submit）
memorix-server（Go）
    ↕ REST API
TiDB Serverless
    ├── 向量搜索（EMBED_TEXT 服务端自动生成）
    ├── 关键词搜索
    └── 用户画像、会话摘要、GC 等所有数据
```

**为什么选 TiDB Serverless？**

- **免费额度**：25 GiB 存储 + 250M Request Units/月，个人和小团队够用
- **不需要 Embedding 服务**：向量由 TiDB 数据库侧的 `EMBED_TEXT()` 函数自动生成，不需要调用 OpenAI Embedding API，也不需要本地模型
- **零运维**：无服务器，自动扩缩，自动备份
- **MySQL 兼容**：随时可迁移到自托管 TiDB 或 MySQL

**最小配置清单（只需 4 个环境变量）：**

```bash
MNEMO_DSN="..."                    # 数据库连接（TiDB Zero 自动填 / 手动填）
MNEMO_LLM_API_KEY="sk-..."         # LLM API Key（智能写入必须）
MNEMO_LLM_BASE_URL="https://..."   # LLM 接口地址
MNEMO_EMBED_AUTO_MODEL="tidbcloud_free/amazon/titan-embed-text-v2"  # TiDB 自动向量化
```

---

## 2. 前置条件

| 项目 | 说明 |
|---|---|
| **Go 1.22+** | 或使用 Docker，跳过 Go 安装 |
| **LLM 服务** | OpenAI / DeepSeek / 本地 Ollama 均可，用于智能写入的事实提取和对账 |
| **TiDB Cloud 账号** | [tidbcloud.com](https://tidbcloud.com) 免费注册，或使用 TiDB Zero 全自动开通（无需注册） |

---

## 3. 第一步：准备数据库

### 方式 A：TiDB Zero 全自动（推荐）

**不需要提前建数据库**。memorix-server 启动后，调用一个 API 即可自动分配一个独立的 TiDB Serverless 实例，DSN 由系统自动管理。

服务端需要一个**控制面数据库**（用于记录租户信息），可以用任意 MySQL 兼容数据库，包括另一个 TiDB Serverless 实例。

```bash
# 1. 在 TiDB Cloud 上手动创建一个 Serverless 实例作为控制面
#    获取连接信息后，拼成 DSN：
MNEMO_DSN="user:pass@tcp(gateway01.us-east-1.prod.aws.tidbcloud.com:4000)/memorix?parseTime=true&tls=true"

# 2. 设置好后，TiDB Zero 会在 POST /v1alpha1/memorix 时
#    自动为每个租户创建独立的 TiDB Serverless 实例
MNEMO_TIDB_ZERO_ENABLED=true   # 默认已开启，无需显式设置
```

> **注意**：TiDB Zero 自动开通的实例有 30 天免费试用期。

---

### 方式 B：已有 TiDB Serverless 手动配置

如果你已经在 TiDB Cloud 上有 Serverless 实例，直接拿连接信息配置 DSN：

**1. 在 TiDB Cloud 获取连接信息**

进入 TiDB Cloud → 你的集群 → Connect → 选择 **General** → 复制连接字符串。

格式如下：
```
mysql://user:password@gateway01.us-east-1.prod.aws.tidbcloud.com:4000/test?sslmode=require
```

**2. 转换为 memorix 使用的 DSN 格式**

```bash
# TiDB Cloud 给的格式（mysql://...）需要转换为 Go mysql driver 格式
MNEMO_DSN="user:password@tcp(gateway01.us-east-1.prod.aws.tidbcloud.com:4000)/memorix?parseTime=true&tls=true"
```

转换规则：
- `mysql://user:password@host:port/db` → `user:password@tcp(host:port)/db?parseTime=true&tls=true`
- 数据库名建议用 `memorix`（需要在 TiDB Cloud 上提前建好）
- **务必加 `tls=true`**，TiDB Serverless 要求 TLS
- **DSN 在 shell 中必须加引号**，避免 `tcp(...)` 括号被解析

**3. 在 TiDB Cloud 建库**

```sql
CREATE DATABASE memorix;
```

然后执行 schema：
```bash
mysql -h gateway01.us-east-1.prod.aws.tidbcloud.com -P 4000 \
  -u user -p --ssl-mode=REQUIRE memorix < server/schema.sql
```

---

## 4. 第二步：部署 memorix-server

### 方式一：源码运行

```bash
git clone https://github.com/devioslang/memorix.git
cd memorix/server

# 配置环境变量
export MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true&tls=true"
export MNEMO_LLM_API_KEY="sk-..."
export MNEMO_LLM_BASE_URL="https://api.openai.com/v1"
export MNEMO_EMBED_AUTO_MODEL="tidbcloud_free/amazon/titan-embed-text-v2"

# 启动
go run ./cmd/memorix-server
# → 2026/03/22 INFO server started port=8080
```

### 方式二：Docker

```bash
docker build -t memorix-server ./server

docker run -d --name memorix-server -p 8080:8080 \
  -e MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true&tls=true" \
  -e MNEMO_LLM_API_KEY="sk-..." \
  -e MNEMO_LLM_BASE_URL="https://api.openai.com/v1" \
  -e MNEMO_EMBED_AUTO_MODEL="tidbcloud_free/amazon/titan-embed-text-v2" \
  memorix-server
```

### LLM Provider 配置参考

三选一，填入对应的 `MNEMO_LLM_*` 变量：

```bash
# OpenAI
MNEMO_LLM_BASE_URL=https://api.openai.com/v1
MNEMO_LLM_MODEL=gpt-4o-mini          # 默认值，可不填

# DeepSeek（性价比高）
MNEMO_LLM_BASE_URL=https://api.deepseek.com/v1
MNEMO_LLM_MODEL=deepseek-chat

# Ollama（完全本地，无需联网）
MNEMO_LLM_BASE_URL=http://localhost:11434/v1
MNEMO_LLM_MODEL=llama3.2
MNEMO_LLM_API_KEY=ollama              # Ollama 需要一个占位 key
```

### 验证服务启动

```bash
curl http://localhost:8080/healthz
# → {"status":"ok"}
```

---

## 5. 第三步：开通租户

**每个用户/团队对应一个租户（Tenant）**，所有记忆数据按租户隔离。

```bash
curl -s -X POST http://localhost:8080/v1alpha1/memorix | jq .
```

返回：

```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "claim_url": "https://tidbcloud.com/..."
}
```

**保存 `id`**，这是你的 Tenant ID，后续所有操作都需要它。

> **TiDB Zero 用户**：此时系统已自动为你分配了一个独立的 TiDB Serverless 实例，并建好了所有表（memories、user_profile_facts 等）和向量索引。`claim_url` 是认领该实例的链接（可选）。
>
> **手动 DSN 用户**：数据表在首次调用此接口时自动创建，无需手动建表。

### ⚠️ 关键：表结构只建一次

`MNEMO_EMBED_AUTO_MODEL` 决定了 `embedding` 列是普通向量列还是自动计算列，而**建表 DDL 只在开通租户时执行一次**。

- **正确顺序**：先配好 `MNEMO_EMBED_AUTO_MODEL`，再调用 `POST /v1alpha1/memorix`
- **错误顺序**：先开通租户（建了普通 `embedding` 列），再设置 `MNEMO_EMBED_AUTO_MODEL`——向量搜索不会工作

如果顺序搞反了，需要删除 `memories` 表后重新开通租户。

---

## 6. 第四步：接入 Claude Code

### 方式 A：Marketplace 安装（推荐）

在 Claude Code 中运行：

```
/plugin marketplace add devioslang/memorix
/plugin install memorix-memory@memorix
```

出现权限确认时，接受 hook 权限。

然后编辑 `~/.claude/settings.json`，在 `env` 下添加：

```json
{
  "env": {
    "MNEMO_API_URL": "http://localhost:8080",
    "MNEMO_TENANT_ID": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "MNEMO_AGENT_ID": "claude-code"
  }
}
```

重启 Claude Code 生效。

### 方式 B：手动安装

```bash
cd memorix
chmod +x claude-plugin/hooks/*.sh

# 复制 skills
mkdir -p ~/.claude/skills
cp -r claude-plugin/skills/memory-recall ~/.claude/skills/
cp -r claude-plugin/skills/memory-store ~/.claude/skills/

# 获取绝对路径
PLUGIN_DIR="$(pwd)/claude-plugin"
echo $PLUGIN_DIR   # 记下这个路径
```

编辑 `~/.claude/settings.json`（合并进现有配置）：

```json
{
  "env": {
    "MNEMO_API_URL": "http://localhost:8080",
    "MNEMO_TENANT_ID": "你的-tenant-id",
    "MNEMO_AGENT_ID": "claude-code"
  },
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "/绝对路径/claude-plugin/hooks/session-start.sh"}]}],
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "/绝对路径/claude-plugin/hooks/user-prompt-submit.sh"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "/绝对路径/claude-plugin/hooks/stop.sh", "timeout": 120}]}]
  }
}
```

---

## 7. 验证端到端

### 7.1 手动写入一条记忆

```bash
curl -s -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/memories \
  -H "Content-Type: application/json" \
  -H "X-Memorix-Agent-Id: test" \
  -d '{"content": "用户偏好 TypeScript，不喜欢 any 类型", "tags": ["preference"]}' | jq .
```

返回示例：

```json
{
  "id": "a1b2c3d4-...",
  "content": "用户偏好 TypeScript，不喜欢 any 类型",
  "state": "active",
  "version": 1
}
```

### 7.2 语义搜索验证向量化

```bash
# 用语义相关但不完全相同的词来搜索
curl -s "http://localhost:8080/v1alpha1/memorix/<tenantID>/memories?q=前端类型系统&limit=5" | jq '.memories[].content'
```

如果能搜到上面那条记忆，说明 TiDB 自动向量化和语义搜索正常工作。

### 7.3 验证智能写入（需要 LLM）

```bash
curl -s -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/memories \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {"role": "user", "content": "我叫张三，是一名后端工程师，主要用 Go 和 Python"},
      {"role": "assistant", "content": "你好张三！很高兴认识你。"}
    ],
    "session_id": "test-session"
  }' | jq .
```

返回的 `memories_changed > 0` 且 `status: "complete"` 说明 LLM 提取事实成功。

### 7.4 验证 Claude Code 集成

重启 Claude Code 后，开始一个新会话，问：

```
你知道我之前对你说过什么吗？
```

如果 Claude 能提到刚才写入的内容，说明 session-start hook 正常从 memorix 加载了记忆。

---

## 8. 当前具备的能力

成功部署后，你获得了以下全部功能：

### 🧠 记忆存储与搜索

| 功能 | 说明 |
|---|---|
| **语义搜索** | 基于向量相似度，搜"前端类型系统"能找到"TypeScript 偏好" |
| **关键词搜索** | 精确词命中，两路合并 RRF 排序 |
| **智能写入** | LLM 自动从对话提取原子事实，自动去重/更新/删除矛盾信息 |
| **记忆保护** | `type=pinned` 的记忆不会被自动覆盖或删除 |
| **乐观锁** | `If-Match` 头实现并发安全更新，版本冲突返回 409 |

### 👤 用户画像

自动积累用户的结构化长期事实（姓名、偏好、技能、目标），支持按分类精确查询，有完整的变更 audit log。

### 💬 会话摘要

每个会话结束后可生成 LLM 摘要，下次对话时作为上下文注入，避免丢失历史对话脉络。

### 🤖 Claude Code 自动记忆

| 时机 | 行为 |
|---|---|
| **Session 开始** | 自动加载最近记忆到上下文 `<relevant-memories>` 标签 |
| **每次 Prompt** | 系统提示可用 `/memory-store` 和 `/memory-recall` 技能 |
| **Session 结束** | 自动保存最后一次 assistant 回复为新记忆 |
| **手动存储** | "帮我记住：..." → 触发 `/memory-store` |
| **手动搜索** | "我们之前讨论过什么..." → 触发 `/memory-recall` |

### 🗑️ 自动 GC（内存垃圾回收）

默认每 24 小时自动清理，**无需任何配置即开箱工作**：
- 90 天未访问且低置信度的记忆 → 标记为 stale 删除
- 重要性分数最低的记忆（综合来源、置信度、访问频率计算）→ 低重要性删除
- 每租户超过 10,000 条时 → 按重要性从低到高删除超出部分

---

## 9. 重要注意事项

### ⚠️ MNEMO_EMBED_AUTO_MODEL 必须在建表前配置

这是最常见的坑。`embedding` 列的类型由首次建表时的配置决定：

```
配置了 MNEMO_EMBED_AUTO_MODEL → embedding 列 = GENERATED ALWAYS AS (EMBED_TEXT(...)) STORED
没配置                         → embedding 列 = VECTOR(1536) NULL（普通列，需客户端传值）
```

一旦表建好，切换方式需要删表重建。

### ⚠️ DSN 必须加引号

```bash
# 错误：shell 会把 tcp(...) 括号当子命令处理
MNEMO_DSN=user:pass@tcp(host:4000)/db

# 正确：用双引号包裹
MNEMO_DSN="user:pass@tcp(host:4000)/db?parseTime=true&tls=true"
```

### ⚠️ TiDB Serverless 必须开 TLS

连接串里必须带 `&tls=true`，否则连接被拒绝。

### ⚠️ 没有 LLM 会静默降级

`MNEMO_INGEST_MODE=smart`（默认）但不配 LLM 时，系统**不报错**，直接把整段对话原文存储（raw 模式）。如果你发现写入的内容是整段对话而不是结构化事实，检查 LLM 环境变量是否配置正确。

### ⚠️ 多 Agent 共用同一个 Tenant

多个 Agent（多台机器、多个工具）用同一个 Tenant ID 时，记忆是完全共享的。用 `MNEMO_AGENT_ID`（或 `X-Memorix-Agent-Id` 请求头）区分来源，可按 `?agent_id=` 过滤。

---

## 10. 常见问题

**Q：`POST /v1alpha1/memorix` 返回 "provisioning disabled"**

TiDB Zero 未配置。设置 `MNEMO_TIDB_ZERO_ENABLED=true`（默认已是 true），或改用方式 B 手动配置 DSN 后直接调用 CRUD 接口。

---

**Q：搜索结果没有语义相关性，只有关键词匹配**

`MNEMO_EMBED_AUTO_MODEL` 没有设置，或在租户开通后才设置（表结构已是普通 `embedding` 列）。删除 `memories` 表并重新开通租户，确保 `MNEMO_EMBED_AUTO_MODEL` 在开通前就配置好。

---

**Q：写入后 `memories_changed: 0`，`status: "complete"`**

LLM 从对话中没有提取到有价值的事实（例如只是打招呼）。这是正常行为，LLM 会过滤闲聊和一次性调试信息。

---

**Q：写入后 `status: "partial"`，`warnings: 1`**

LLM 提取了事实，但对账（Reconcile）阶段 LLM 调用失败（网络超时、API 额度不足等）。事实被提取但没有写入，避免重复写入。检查 LLM 服务状态。

---

**Q：Claude Code session 启动很慢或卡住**

Hook 脚本路径不对或没有执行权限。检查：
```bash
# 确认脚本有执行权限
ls -la ~/memorix/claude-plugin/hooks/*.sh

# 手动测试
MNEMO_API_URL=http://localhost:8080 MNEMO_TENANT_ID=xxx \
  bash ~/memorix/claude-plugin/hooks/session-start.sh
```

---

**Q：记忆搜索返回空，但我确实写了很多内容**

检查请求是否带了正确的 Tenant ID。所有数据按 Tenant ID 隔离，用错 ID 查不到任何数据。

---

## 11. 进阶配置

基础部署跑通后，可以按需开启更多功能：

### 启用全文搜索（FTS）

TiDB Serverless 支持 `FTS_MATCH_WORD` 时，可开启 FTS 提升中英文混合搜索精度：

```bash
MNEMO_FTS_ENABLED=true
```

> 注意：TiDB Zero 自动开通的实例默认不支持 FTS，需确认集群版本。

### 调整 GC 参数

```bash
MNEMO_GC_STALE_THRESHOLD=720h          # 缩短到 30 天（默认 2160h/90天）
MNEMO_GC_MAX_MEMORIES_PER_TENANT=5000  # 降低容量上限（默认 10000）
MNEMO_GC_INTERVAL=6h                   # 更频繁运行（默认 24h）
```

### 调整 Token 预算

```bash
MNEMO_MAX_CONTEXT_TOKENS=16384         # 上下文窗口大小（默认 8192）
MNEMO_USER_MEMORY_BUDGET_MAX=3000      # 记忆注入最大 token 数（默认 1500）
```

### 使用客户端 Embedding（替代 TiDB Auto-Embed）

如果不使用 TiDB 自动向量化，改用 OpenAI / Ollama 客户端 Embedding：

```bash
# 不设置 MNEMO_EMBED_AUTO_MODEL
MNEMO_EMBED_API_KEY="sk-..."
MNEMO_EMBED_BASE_URL="https://api.openai.com/v1"
MNEMO_EMBED_MODEL="text-embedding-3-small"
MNEMO_EMBED_DIMS=1536

# 使用 Ollama 本地 Embedding（完全离线）
MNEMO_EMBED_BASE_URL="http://localhost:11434/v1"
MNEMO_EMBED_MODEL="nomic-embed-text"
MNEMO_EMBED_API_KEY="ollama"
MNEMO_EMBED_DIMS=768
```

### 多个 Agent 共享记忆

同一个 Tenant ID 可以被 Claude Code、OpenCode、OpenClaw 同时使用，所有 Agent 读写同一个记忆池：

```bash
# Claude Code
MNEMO_AGENT_ID=claude-code

# 另一个 Claude Code 实例（工作用）
MNEMO_AGENT_ID=claude-work
```

按 agent_id 过滤：`GET /memories?agent_id=claude-code`

---

## 配置速查表

```bash
# ===== 必填 =====
MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true&tls=true"
MNEMO_LLM_API_KEY="sk-..."
MNEMO_LLM_BASE_URL="https://api.openai.com/v1"
MNEMO_EMBED_AUTO_MODEL="tidbcloud_free/amazon/titan-embed-text-v2"

# ===== 可选，有合理默认值 =====
MNEMO_LLM_MODEL=gpt-4o-mini            # LLM 模型，默认 gpt-4o-mini
MNEMO_EMBED_AUTO_DIMS=1024             # 向量维度，默认 1024
MNEMO_PORT=8080                        # 监听端口，默认 8080
MNEMO_INGEST_MODE=smart                # 写入模式，默认 smart
MNEMO_GC_ENABLED=true                  # 自动 GC，默认开启
MNEMO_MAX_CONTEXT_TOKENS=8192          # 上下文 token 上限，默认 8192

# ===== Claude Code 插件 =====
MNEMO_API_URL="http://localhost:8080"
MNEMO_TENANT_ID="你的-tenant-id"
MNEMO_AGENT_ID="claude-code"
```
