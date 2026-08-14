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
| GET | `/api/query/{query_code}` | 无需登录，按查询码聚合查询上传会话进度 |
| POST | `/api/query/{query_code}/collect` | 登录后收藏采集端上传会话到个人空间 |
| GET | `/api/keywords/sync` | 标准关键字库云端同步（采集端用上传 Token 认证） |
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
| GET/POST/DELETE | `/api/admin/session` | 查询、解锁或锁定管理员会话 |
| GET/POST | `/api/admin/projects` | 查询或创建管理员项目 |
| PUT/DELETE | `/api/admin/projects/{id}` | 更新或停用管理员项目 |
| GET/POST | `/api/admin/project-options` | 查询或创建项目产线、类型、阶段选项 |
| PUT/DELETE | `/api/admin/project-options/{id}` | 更新或停用项目属性选项 |
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

### POST `/api/upload-sessions`

采集端保存上传配置时创建连续上传会话。请求为 JSON，必须包含 `client_request_id`、`project_name`、`version` 和 `uploader_name`，并携带完整通道配置快照；同一上传用户重复提交相同 `client_request_id` 时返回原会话和原查询码。

成功返回 `201 Created`，数据包含 `upload_session_id`、`query_code`、`client_request_id` 和 `status`。程序重启、串口短暂断开、暂停及恢复继续使用该会话。

查询码格式：`<项目名大写字母数字前缀>-<10 位随机大写十六进制>`，例如 `DR2860-A1B2C3D4E5`，项目名前缀便于直接识别所属项目；纯数字字母外的字符会被过滤，前缀最长 16 个字符。

### POST `/api/upload-sessions/{upload_session_id}/complete`

结束会话的新采集活动。结束前形成的积压批次仍可携带原会话 ID 和查询码上传。

### GET `/api/keywords/sync`

面向采集端的标准关键字库同步接口。认证方式与日志上传一致：网页端可用登录 Cookie，采集端使用 `Authorization: Bearer <token>`。返回管理员维护的已启用标准关键字（`source = admin_keyword_upload` 且启用）：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 12,
      "name": "MP4 写入失败",
      "category": "recording",
      "keyword": "XA_MP4_Write failed",
      "scope": "自研通用",
      "level": "critical",
      "description": "视频文件写入失败",
      "updated_at": "2026-08-12T10:00:00+08:00"
    }
  ]
}
```

### POST `/api/logs/upload`

请求类型：`multipart/form-data`

网页端可使用登录 Cookie。采集端使用 `Authorization: Bearer <token>`；服务端必须同时配置 `LOGMASTER_UPLOAD_TOKEN` 和对应已有用户的 `LOGMASTER_UPLOAD_OWNER_OPEN_ID`。

请求头可携带 `Idempotency-Key: <client_request_id>`。同一上传用户重复提交相同键时，服务端返回首次创建的上传任务和查询码，不重复保存文件。采集端可使用 `Content-Encoding: gzip` 压缩整个 multipart 请求体。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file` | File[] | 是 | 字段可重复，支持多个文件 |
| `upload_session_id` | UUID | 否 | 连续上传会话 ID；携带查询码时必填 |
| `query_code` | String | 否 | 会话查询码；携带会话 ID 时必填 |
| `project_id` | String | 否 | 平台项目稳定 ID；当前对应项目的数字 ID，提供时必须与项目名称一致 |
| `project_name` | String | 是 | 项目名称快照，最大 128 字符 |
| `version` | String | 是 | 固件、软件或产品版本，最大 64 字符 |
| `test_task_id` | String | 否 | 测试任务 ID，最大 128 字符 |
| `test_task_name` | String | 否 | 测试任务名称快照，最大 256 字符 |
| `uploader_name` | String | 是 | 上传人姓名快照，最大 128 字符 |
| `remark` | String | 否 | 本次测试备注，最大 4000 字符 |
| `client_request_id` | String | 否 | 客户端幂等 ID；请求头优先，最大 128 字符 |
| `collector_version` | String | 否 | 客户端版本，最大 64 字符 |
| `timezone` | String | 否 | IANA 时区，例如 `Asia/Shanghai` |
| `created_at` | RFC 3339 | 否 | 客户端任务创建时间，必须包含时区 |
| `started_at` | RFC 3339 | 否 | 客户端开始采集时间，必须包含时区 |
| `ended_at` | RFC 3339 | 否 | 客户端停止采集时间，必须包含时区 |
| `scenario_ids` | JSON String[] | 否 | 测试场景 ID 列表，最多 20 个 |

`uploader_id` 不接受客户端指定，由服务端根据当前登录身份自动保存。

成功响应：`202 Accepted`

```json
{
  "code": 0,
  "message": "upload accepted",
  "data": {
    "upload_id": "eb1527fc-58bb-42a1-bd56-dff40d374afa",
    "task_id": "d3a67a1e-34c9-40f7-9224-42b19f53d143",
    "query_code": "DCD827E7A8",
    "status": "queued",
    "file_count": 6,
    "client_request_id": "71c8492d-0d7b-4ad4-88f7-dde542584898"
  }
}
```

处理流程：

1. 保存原始文件到 `LOG_STORAGE_DIR/<项目>/<登录用户名称或用户ID>/<YYYY-MM-DD>/<upload_id>`。
2. 安全解压压缩包。
3. 写入上传记录、文件记录和解析任务。
4. 后台逐文件解析。
5. 保存本地匹配结果。
6. 如果配置 Agent，则逐文件调用 Agent。

### GET `/api/query/{query_code}`

无需登录，按查询码聚合返回该连续上传会话下的全部批次、文件、项目、版本、上传人、任务状态和解析统计，不返回原始日志内容。查询码不区分大小写；支持输入完整带前缀查询码（如 `DR2860-A1B2C3D4E5`）或仅输入 10 位随机部分（如 `A1B2C3D4E5`），两种写法均可命中。

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "upload_id": "eb1527fc-58bb-42a1-bd56-dff40d374afa",
    "task_id": "d3a67a1e-34c9-40f7-9224-42b19f53d143",
    "query_code": "DR2860-A1B2C3D4E5",
    "project_name": "DR2860",
    "version": "V1.0.0",
    "status": "completed",
    "total_files": 6,
    "processed_files": 6,
    "total_lines": 50000,
    "error_count": 3,
    "warning_count": 8
  }
}
```

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
        "uploader_name": "张三",
        "uploader_email": "zhangsan@company.com",
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

当前本地解析器读取数据库中已启用的 `parse_rules`，按照优先级匹配文档规则；通用 `FATAL/ERROR` 规则仅作为未命中专用规则时的兜底。

解析规则驱动后，结果会在不删除原字段的情况下增加：

```json
{
  "rule_id": 12,
  "rule_name": "MP4 写入失败",
  "category": "recording",
  "event_time": "2026-07-22T10:16:13+08:00",
  "context_start_time": "2026-07-22T10:16:03+08:00",
  "context_end_time": "2026-07-22T10:16:23+08:00",
  "context_lines": [
    {"line_number": 13257, "timestamp": "2026-07-22T10:16:12+08:00", "content": "...", "is_hit": false}
  ],
  "related_causes": [
    {"kind": "block_io", "label": "块设备 I/O 异常", "reason": "底层块设备写入失败", "line_number": 13257, "confidence": 0.98}
  ]
}
```

`context_lines` 是关键字命中行前后各 50 行日志（文件边界处不足 50 行时按实际行数返回），`related_causes` 是这些上下文行内识别出的可能上游原因。原有 `level`、`matched_text`、`line_number`、`content` 和 `file_path` 字段保持不变。

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

返回所有用户共享且已启用的项目目录，包括系统预置项目以及管理员后续创建的项目。上传历史不会自动产生项目。

```json
{
  "code": 0,
  "message": "success",
  "data": ["DR1210", "DR1400", "DR1500", "DR1800", "DR1810"]
}
```

### 管理员项目接口

管理员接口需要先完成飞书登录，再通过 `POST /api/admin/session` 使用管理员账号和密码解锁。解锁状态保存在独立的 HttpOnly Cookie 中，有效期为 8 小时，后端服务重启后需重新解锁。

项目数据包含 `name`、`product_line`、`product_type`、`stage` 和 `description`。产线预置车载线、宠物线和安防线；项目类型与项目阶段可由管理员动态维护。删除项目会将其从全部用户的可选列表中停用，不会破坏已有日志记录；再次创建同名项目会恢复该项目。

项目属性选项分为 `line`、`type`、`stage` 三类。系统预置项不能删除；正在被启用项目使用的自定义项也不能删除。

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
| `ADMIN_USERNAME` | `Qsm123456` | 管理员子页面账号，生产环境应覆盖默认值 |
| `ADMIN_PASSWORD` | `Qsm123456` | 管理员子页面密码，生产环境应覆盖默认值 |
