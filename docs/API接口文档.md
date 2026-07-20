# LogMaster API 接口文档

版本：当前工作区实现  
基础地址：`http://localhost:8080/api`  
数据格式：除文件上传外均使用 `application/json`

## 1. 通用约定

### 1.1 响应结构

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

失败响应：

```json
{
  "code": 400,
  "message": "错误说明",
  "data": null
}
```

HTTP 状态码与 `code` 通常保持一致。上传成功使用 `202 Accepted`。

### 1.2 认证

Web 登录使用飞书 OAuth，登录成功后服务端写入 HttpOnly Cookie：

```text
session_token=<随机会话令牌>
```

当前只有用户信息接口强制检查会话；日志、任务、规则和场景接口尚未统一挂载认证中间件。生产部署前应补齐接口级鉴权。

### 1.3 分页参数

```text
page=1
page_size=20
```

- `page` 最小为 `1`
- `page_size` 默认 `20`，最大 `200`

### 1.4 任务状态

上传状态：

```text
uploading | queued | parsing | completed | failed
```

解析任务状态：

```text
queued | running | completed | failed
```

## 2. 接口总览

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/health` | 服务健康检查 |
| GET | `/api/auth/feishu-login` | 跳转飞书登录 |
| GET | `/api/auth/feishu-url` | 获取飞书登录 URL |
| GET | `/api/auth/callback` | 飞书 OAuth 回调 |
| POST | `/api/auth/logout` | 注销会话 |
| GET | `/api/user/info` | 当前用户信息 |
| POST | `/api/logs/inspect` | 上传前识别文件或压缩包内容 |
| POST | `/api/logs/upload` | 上传并创建解析任务 |
| GET | `/api/logs` | 上传记录列表 |
| GET | `/api/logs/{upload_id}` | 上传记录详情 |
| GET | `/api/tasks` | 解析任务列表 |
| GET | `/api/tasks/{task_id}` | 任务详情 |
| DELETE | `/api/tasks/{task_id}` | 删除任务及本地文件 |
| GET | `/api/tasks/{task_id}/results` | 本地解析结果 |
| GET | `/api/tasks/{task_id}/agent-results` | Agent 诊断结果 |
| GET | `/api/dashboard/stats` | 仪表板统计 |
| GET | `/api/projects` | 项目名称列表 |
| GET | `/api/system/com-ports` | 本机真实串口列表 |
| GET/POST | `/api/rules` | 查询或创建解析规则 |
| PUT/DELETE | `/api/rules/{id}` | 更新或删除解析规则 |
| GET/POST | `/api/scenarios` | 查询或创建测试场景 |
| PUT/DELETE | `/api/scenarios/{id}` | 更新或删除测试场景 |

## 3. 健康检查

### GET `/api/health`

响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "ok"
  }
}
```

## 4. 飞书认证

### GET `/api/auth/feishu-login`

创建 OAuth `state` Cookie，并以 `302` 跳转飞书授权页。

### GET `/api/auth/feishu-url`

响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "url": "https://accounts.feishu.cn/open-apis/authen/v1/authorize?..."
  }
}
```

### GET `/api/auth/callback?code=...&state=...`

校验 `state`，向飞书交换用户令牌，获取用户信息，写入 `session_token` Cookie，最后跳转 `/`。

### GET `/api/user/info`

已登录响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "用户 ID",
    "name": "用户名称",
    "email": "user@example.com",
    "avatar": "https://..."
  }
}
```

未登录返回 `401`。

## 5. 文件预检

### POST `/api/logs/inspect`

请求类型：`multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file` | File | 是 | 只能提交一个文件 |

支持：

- 无扩展名日志，例如 `logfile`、`logfile_0`
- `.log`、`.txt`、`.out`、`.csv`
- `.zip`、`.gz`、`.tgz`、`.tar.gz`
- ZipCrypto 和 AES 加密 ZIP

加密 ZIP 默认密码：

```text
70M_dashcam_^
```

普通无扩展名日志响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "archive": false,
    "entries": [
      {
        "path": "logfile",
        "size_bytes": 59452,
        "encrypted": false
      }
    ]
  }
}
```

压缩包响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "archive": true,
    "entries": [
      {
        "path": "logfile_0",
        "size_bytes": 3145728,
        "encrypted": true
      }
    ]
  }
}
```

设备生成的 `/logfile_0` 会安全规范化为 `logfile_0`；`../`、盘符路径等目录穿越路径仍会被拒绝。

## 6. 日志上传

### POST `/api/logs/upload`

请求类型：`multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file` | File[] | 是 | 字段可重复，支持多个文件 |
| `project_name` | String | 否 | 默认 `default`，最大 128 字符 |
| `version` | String | 否 | 固件或软件版本，最大 64 字符 |

成功响应：`202 Accepted`

```json
{
  "code": 0,
  "message": "upload accepted",
  "data": {
    "upload_id": "eb1527fc-58bb-42a1-bd56-dff40d374afa",
    "task_id": "d3a67a1e-34c9-40f7-9224-42b19f53d143",
    "status": "queued",
    "file_count": 6
  }
}
```

处理流程：

1. 保存原始文件到 `LOG_STORAGE_DIR`。
2. 安全解压压缩包。
3. 写入上传记录、文件记录和解析任务。
4. 后台逐文件解析。
5. 保存本地匹配结果。
6. 如果配置 Agent，则逐文件调用 Agent。

## 7. 上传记录

### GET `/api/logs?page=1&page_size=20`

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 1,
    "list": [
      {
        "id": "upload UUID",
        "task_id": "task UUID",
        "project_name": "DR2860",
        "version": "V1.0.0",
        "status": "completed",
        "original_name": "logs.zip",
        "original_size": 102400,
        "file_count": 6,
        "total_lines": 50000,
        "error_count": 3,
        "warning_count": 8,
        "created_at": "2026-07-20T15:00:00+08:00",
        "updated_at": "2026-07-20T15:00:10+08:00"
      }
    ]
  }
}
```

### GET `/api/logs/{upload_id}`

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "upload": {},
    "files": [
      {
        "id": 1,
        "relative_path": "items/1/extracted/logfile_0",
        "size_bytes": 3145728,
        "sha256": "...",
        "line_count": 12000
      }
    ]
  }
}
```

`relative_path` 是相对于该上传任务存储目录的后端内部路径，不是用户电脑上的原始路径。

## 8. 解析任务

### GET `/api/tasks`

响应结构与 `/api/logs` 列表相同。

### GET `/api/tasks/{task_id}`

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task": {},
    "files": [],
    "agent_enabled": true
  }
}
```

### DELETE `/api/tasks/{task_id}`

删除：

- 上传记录
- 文件记录
- 本地解析结果
- Agent 结果
- 本地存储目录
- 没有其他上传记录引用的空项目

### GET `/api/tasks/{task_id}/results?page=1&page_size=20`

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "level": "error",
      "matched_text": "ERROR",
      "line_number": 42,
      "content": "ERROR camera initialization failed",
      "file_path": "items/1/extracted/logfile_0"
    }
  ]
}
```

当前本地解析器只识别 `FATAL`、`ERROR`、`WARNING` 和 `WARN`。

### GET `/api/tasks/{task_id}/agent-results`

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "task_id": "task UUID",
      "log_file_id": 1,
      "file_path": "items/1/extracted/logfile_0",
      "provider": "http-agent",
      "status": "completed",
      "summary": "录像初始化异常",
      "findings": [],
      "created_at": "2026-07-20T15:00:00+08:00",
      "updated_at": "2026-07-20T15:00:00+08:00"
    }
  ]
}
```

## 9. 仪表板和基础信息

### GET `/api/dashboard/stats?days=7`

`days` 支持 `7` 或 `30`。

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_lines": 0,
    "error_count": 0,
    "warning_count": 0,
    "task_count": 0,
    "completed_count": 0,
    "failed_count": 0,
    "trend": [],
    "top_matches": [],
    "recent_tasks": []
  }
}
```

### GET `/api/projects`

```json
{
  "code": 0,
  "message": "success",
  "data": ["DR2860", "DR5800"]
}
```

### GET `/api/system/com-ports`

返回后端机器实际枚举到的串口：

```json
{
  "code": 0,
  "message": "success",
  "data": ["COM1"]
}
```

该接口只枚举串口；实时串口采集会话接口尚未实现。

## 10. 解析规则

规则结构：

```json
{
  "id": 1,
  "name": "系统错误",
  "category": "system",
  "keyword": "ERROR|FATAL",
  "scope": "通用",
  "level": "critical",
  "enabled": true,
  "description": "识别系统错误"
}
```

接口：

- `GET /api/rules`
- `POST /api/rules`
- `PUT /api/rules/{id}`
- `DELETE /api/rules/{id}`

注意：规则 CRUD 已完成，但当前本地日志解析器尚未读取 `parse_rules` 表执行动态规则。

## 11. 测试场景

场景结构：

```json
{
  "id": "power-cycle",
  "name": "开关机测试",
  "description": "检查启动和关机异常",
  "color": "blue",
  "judgement": "any-error",
  "checks": [
    {
      "id": "unexpected-reboot",
      "name": "异常重启",
      "severity": "critical",
      "enabled": true,
      "keywords": ["POWER_ID_SWRT", "backtrace"]
    }
  ]
}
```

接口：

- `GET /api/scenarios`
- `POST /api/scenarios`
- `PUT /api/scenarios/{id}`
- `DELETE /api/scenarios/{id}`

注意：场景配置已持久化，但尚未参与解析任务编排。

## 12. Agent 接入协议

### 12.1 启用配置

```powershell
$env:AGENT_ANALYSIS_URL="http://127.0.0.1:9000/analyze"
$env:AGENT_ANALYSIS_TOKEN="your-agent-token"
$env:AGENT_ANALYSIS_TIMEOUT_SECONDS="60"
```

`AGENT_ANALYSIS_URL` 必须是 Agent 接收请求的完整 URL。

### 12.2 调用时机

每个日志文件完成本地解析并写入数据库后，后端同步调用一次 Agent：

```text
上传任务
  -> 解压文件
  -> 本地 ERROR/WARN 匹配
  -> 保存本地结果
  -> POST Agent
  -> 保存 Agent 响应
```

Agent 调用失败不会让主解析任务失败；失败信息写入 `agent_analyses.error_message`。

### 12.3 后端发送给 Agent 的请求

请求头：

```http
Content-Type: application/json
Authorization: Bearer <AGENT_ANALYSIS_TOKEN>
```

未配置 Token 时不发送 `Authorization`。

请求体：

```json
{
  "task_id": "d3a67a1e-34c9-40f7-9224-42b19f53d143",
  "upload_id": "eb1527fc-58bb-42a1-bd56-dff40d374afa",
  "file": {
    "id": 1,
    "relative_path": "items/1/extracted/logfile_0",
    "size_bytes": 3145728,
    "sha256": "...",
    "line_count": 12000
  },
  "total_lines": 12000,
  "matches": [
    {
      "level": "error",
      "matched_text": "ERROR",
      "line_number": 42,
      "content": "ERROR recorder failed",
      "file_path": ""
    }
  ]
}
```

约束：

- 每个文件最多向 Agent 发送前 `2000` 条本地命中记录。
- 单行内容最多保留 `4000` 字节。
- 当前不发送完整日志正文。
- `relative_path` 是后端内部相对路径，远程 Agent 无法直接读取该路径。

### 12.4 Agent 必须返回的响应

HTTP 状态必须为 `2xx`，响应体：

```json
{
  "summary": "录像服务初始化失败",
  "findings": [
    {
      "category": "recording",
      "severity": "error",
      "root_cause": "摄像头初始化超时",
      "suggestion": "检查摄像头连接和初始化顺序",
      "evidence": "ERROR recorder failed",
      "confidence": 0.92
    }
  ]
}
```

Agent 响应最大读取 `4 MiB`。

### 12.5 Go 进程内接入点

```go
type AgentAnalyzer interface {
    Provider() string
    Analyze(context.Context, AgentAnalysisRequest) (AgentAnalysisResponse, error)
}
```

可以通过 `NewServiceWithAgent` 注入进程内实现，也可以使用默认 `HTTPAgentAnalyzer`。

## 13. Agent 接入成熟度

### 已完成

- Agent Go 接口抽象。
- 可配置 HTTP Agent 地址、Bearer Token 和超时。
- 每个日志文件自动触发 Agent。
- Agent 请求和响应结构固定。
- Agent 成功/失败结果持久化。
- 前端和 API 可查询 Agent 结果。
- Agent 失败不影响基础解析结果。

### 尚未完成

- 动态规则表尚未接入本地解析器。
- 测试场景尚未参与任务编排。
- Agent 收不到完整日志文件或下载地址。
- 没有独立 Agent 队列、并发控制和重试策略。
- 没有 Agent 异步回调接口。
- 没有请求幂等键、协议版本和能力协商。
- 没有 Agent 任务取消、进度上报和超时后的补偿任务。
- 数据接口尚未统一强制认证。

结论：当前已经为 Agent 铺好了可开发、可联调的基础通道，但距离生产级 Agent 解析平台仍需要补齐任务队列、完整日志访问、动态规则执行、回调/重试和鉴权。

## 14. 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DATABASE_URL` | 无 | PostgreSQL 连接地址，必填 |
| `LOG_STORAGE_DIR` | `data/logs` | 文件存储根目录 |
| `MAX_UPLOAD_BYTES` | `2147483648` | 最大上传字节数 |
| `MAX_EXTRACT_BYTES` | `8589934592` | 单任务最大解压字节数 |
| `FRONTEND_DIST_DIR` | `frontend/dist` | Vue 构建产物目录 |
| `FEISHU_APP_ID` | 测试默认值 | 飞书应用 ID |
| `FEISHU_APP_SECRET` | 无 | 飞书应用密钥 |
| `FEISHU_REDIRECT_URI` | `http://localhost:8080/api/auth/callback` | OAuth 回调地址 |
| `AGENT_ANALYSIS_URL` | 空 | Agent 完整 HTTP 地址 |
| `AGENT_ANALYSIS_TOKEN` | 空 | Agent Bearer Token |
| `AGENT_ANALYSIS_TIMEOUT_SECONDS` | `60` | Agent 请求超时秒数 |
