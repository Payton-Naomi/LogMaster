# LogMaster 后端 API 文档

本文档是 `server/backend` 当前全部 HTTP API 的统一说明，更新日期为 2026-08-15。前端专用说明见 [`api-to-frontend.md`](api-to-frontend.md)，采集端专用说明见 [`api-to-collector.md`](api-to-collector.md)。

> **最新：818 调整（2026-08-18）**：`POST /api/upload-sessions` 必须提交 `uploader_email`；后端通过飞书通讯录校验企业成员身份并同步用户资料，`uploader_name` 可选。

> **818 补充**：AI 分析完成后会记录 `ai_usage` token 用量；迁移 `027_ai_usage_permissions.sql` 补齐表和序列权限，后端重启后自动执行。该修复不改变 HTTP API 返回结构。

> **819 调整**：超级管理员可通过 `/api/admin/ai-analysis-settings` 修改大模型地址、模型、超时、命中采样上限、输入字节上限和输出 token/配额。API Key 只接收写入，不返回明文，并使用 `LOGMASTER_CONFIG_ENCRYPTION_KEY` 加密保存；数据库配置优先于环境变量。

AI 分析命中日志采用按规则/关键词分组轮询采样，避免重复关键词占满输入；采样上限和输入字节保护由后端固定执行。

## 1. 全部 API 清单

> **8/16新增**：`GET` / `PUT` `/api/admin/ai-analysis-settings`
> **8/16之前**：其余全部接口

### 1.1 健康检查与认证

| 方法 | 路径 | 访问要求 | 说明 | 日期 |
| --- | --- | --- | --- | --- |
| `GET` | `/health` | 公开 | 健康检查兼容路径 | 8/16之前 |
| `GET` | `/api/health` | 公开 | API 健康检查 | 8/16之前 |
| `GET` | `/api/auth/feishu-url` | 公开 | 获取飞书 OAuth 地址 | 8/16之前 |
| `GET` | `/api/auth/feishu-login` | 公开 | 跳转飞书 OAuth | 8/16之前 |
| `GET` | `/api/auth/callback` | OAuth 回调 | 校验回调并创建登录会话 | 8/16之前 |
| `POST` | `/api/auth/logout` | 公开 | 删除当前登录会话 Cookie | 8/16之前 |
| `GET` | `/api/auth/me` | 飞书 Session | 获取当前用户，兼容路径 | 8/16之前 |
| `GET` | `/api/user/info` | 飞书 Session | 获取当前用户 | 8/16之前 |

### 1.2 上传、查询与分析

| 方法 | 路径 | 访问要求 | 说明 | 日期 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/upload-requests` | 上传身份 | 创建上传会话，兼容路径 | 8/16之前 |
| `POST` | `/api/upload-sessions` | 上传身份 | 创建连续上传会话和查询码 | 8/16之前 |
| `POST` | `/api/upload-sessions/{id}/complete` | 上传身份 | 关闭上传会话 | 8/16之前 |
| `GET` | `/api/upload-config` | 飞书 Session | 获取上传容量 | 8/16之前 |
| `GET` | `/api/keywords/sync` | 上传身份 | 同步标准关键词 | 8/16之前 |
| `POST` | `/api/logs/inspect` | 飞书 Session | 检查文件或压缩包内容 | 8/16之前 |
| `POST` | `/api/logs/upload` | 上传身份 | 上传日志并创建解析任务 | 8/16之前 |
| `GET` | `/api/query/{query_code}` | 公开 | 按查询码查看上传会话状态 | 8/16之前 |
| `POST` | `/api/query/{query_code}/collect` | 飞书 Session | 将采集会话关联到当前用户 | 8/16之前 |
| `GET` | `/api/logs` | 飞书 Session | 查询上传记录 | 8/16之前 |
| `GET` | `/api/logs/{upload_id}` | 飞书 Session | 查询上传详情 | 8/16之前 |
| `GET` | `/api/logs/{upload_id}/preview` | 飞书 Session | 预览上传日志文本 | 8/16之前 |
| `GET` | `/api/tasks` | 飞书 Session | 查询解析任务 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}` | 飞书 Session | 查询任务详情 | 8/16之前 |
| `DELETE` | `/api/tasks/{task_id}` | 飞书 Session | 删除任务和存储文件 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}/results` | 飞书 Session | 查询规则解析结果 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}/agent-results` | 飞书 Session | 查询 AI/Agent 分析结果 | 8/16之前 |
| `GET` | `/api/dashboard/stats` | 飞书 Session | 查询仪表板统计 | 8/16之前 |
| `GET` | `/api/projects` | 飞书 Session | 查询可上传项目名称 | 8/16之前 |

### 1.3 规则与测试场景

| 方法 | 路径 | 访问要求 | 说明 | 日期 |
| --- | --- | --- | --- | --- |
| `GET` | `/api/rules` | 飞书 Session | 查询当前用户可见规则 | 8/16之前 |
| `POST` | `/api/rules` | 飞书 Session | 当前禁止直接创建，固定返回 `403` | 8/16之前 |
| `PUT` | `/api/rules/batch` | 飞书 Session | 批量设置规则启用状态 | 8/16之前 |
| `PUT` | `/api/rules/{id}` | 飞书 Session | 更新可编辑规则 | 8/16之前 |
| `DELETE` | `/api/rules/{id}` | 飞书 Session | 删除可编辑规则 | 8/16之前 |
| `GET` | `/api/scenarios` | 飞书 Session | 查询测试场景 | 8/16之前 |
| `POST` | `/api/scenarios` | 飞书 Session | 创建测试场景 | 8/16之前 |
| `GET` | `/api/scenarios/{id}` | 飞书 Session | 查询场景详情 | 8/16之前 |
| `PUT` | `/api/scenarios/{id}` | 飞书 Session | 更新测试场景 | 8/16之前 |
| `DELETE` | `/api/scenarios/{id}` | 飞书 Session | 删除测试场景 | 8/16之前 |
| `PATCH` | `/api/scenarios/{id}/enabled` | 飞书 Session | 启用或停用场景 | 8/16之前 |

### 1.4 管理与申请

| 方法 | 路径 | 访问要求 | 说明 | 日期 |
| --- | --- | --- | --- | --- |
| `GET` | `/api/admin/users` | `super_admin` | 查询用户 | 8/16之前 |
| `PUT` | `/api/admin/users/{id}/role` | `super_admin` | 修改用户角色 | 8/16之前 |
| `PUT` | `/api/admin/users/{id}/restore-feishu-role` | `super_admin` | 恢复飞书自动角色 | 8/16之前 |
| `GET` | `/api/admin/permission-requests` | 飞书 Session | 查询本人或全部权限申请 | 8/16之前 |
| `POST` | `/api/admin/permission-requests` | 非超级管理员 | 创建权限申请 | 8/16之前 |
| `DELETE` | `/api/admin/permission-requests/{id}` | 申请人 | 撤回待处理权限申请 | 8/16之前 |
| `PUT` | `/api/admin/permission-requests/{id}/decision` | `super_admin` | 审批权限申请 | 8/16之前 |
| `GET` | `/api/admin/runtime-logs` | 飞书 Session | 查询运行日志 | 8/16之前 |
| `GET` | `/api/admin/project-requests` | 飞书 Session | 查询项目申请 | 8/16之前 |
| `POST` | `/api/admin/project-requests` | `user` 或 `developer` | 创建项目申请 | 8/16之前 |
| `PUT` | `/api/admin/project-requests/{id}/decision` | `admin` 或 `super_admin` | 审批项目申请 | 8/16之前 |
| `GET` | `/api/admin/upload-capacity` | `super_admin` | 查询上传容量设置 | 8/16之前 |
| `PUT` | `/api/admin/upload-capacity` | `super_admin` | 修改上传容量设置 | 8/16之前 |
| `GET` | `/api/admin/ai-analysis-settings` | `super_admin` | 查询 AI 分析限额设置 | 8/16新增 |
| `PUT` | `/api/admin/ai-analysis-settings` | `super_admin` | 修改 AI 分析限额设置 | 8/16新增 |
| `GET` | `/api/admin/keyword-rules` | `developer`、`admin` 或 `super_admin` | 查询标准关键词规则 | 8/16之前 |
| `POST` | `/api/admin/keyword-rules/import` | `developer`、`admin` 或 `super_admin` | 导入标准关键词规则 | 8/16之前 |
| `DELETE` | `/api/admin/keyword-rules/{id}` | `developer`、`admin` 或 `super_admin` | 删除标准关键词规则 | 8/16之前 |
| `GET` | `/api/admin/projects` | `admin` 或 `super_admin` | 查询项目管理列表 | 8/16之前 |
| `POST` | `/api/admin/projects` | `admin` 或 `super_admin` | 创建项目 | 8/16之前 |
| `PUT` | `/api/admin/projects/{id}` | `admin` 或 `super_admin` | 更新项目 | 8/16之前 |
| `DELETE` | `/api/admin/projects/{id}` | `admin` 或 `super_admin` | 停用项目 | 8/16之前 |
| `GET` | `/api/admin/project-options` | 飞书 Session | 查询项目属性选项 | 8/16之前 |
| `POST` | `/api/admin/project-options` | `admin` 或 `super_admin` | 创建项目属性选项 | 8/16之前 |
| `PUT` | `/api/admin/project-options/{id}` | `admin` 或 `super_admin` | 更新项目属性选项 | 8/16之前 |
| `DELETE` | `/api/admin/project-options/{id}` | `admin` 或 `super_admin` | 停用项目属性选项 | 8/16之前 |

## 2. 通用约定

### 2.1 地址和响应

本地服务地址为 `http://localhost:8080`，业务 API 统一使用 `/api` 前缀。JSON 接口使用统一响应结构：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

失败响应通常使用 HTTP 状态码作为 `code`：

```json
{
  "code": 400,
  "message": "错误说明",
  "data": null
}
```

`201` 表示创建成功，`202` 表示上传已接收但仍在后台处理。常见错误为 `400` 参数错误、`401` 未认证、`403` 权限不足、`404` 不存在、`409` 冲突、`405` 方法错误和 `500` 服务端错误。

### 2.2 认证方式

飞书登录成功后，后端写入 HttpOnly Cookie：

```text
session_token=<随机令牌>
```

“上传身份”表示满足以下任意一种认证方式：

- 有效的飞书 `session_token`。
- `Authorization: Bearer logmaster-internal-collector-v1`。
- `Authorization: Bearer <LOGMASTER_UPLOAD_TOKEN>`，并同时配置 `LOGMASTER_UPLOAD_OWNER_OPEN_ID`。

`/api/logs/inspect` 目前只接受飞书 Session，不接受采集端 Bearer Token。

### 2.3 角色权限

| 角色 | 管理权限 |
| --- | --- |
| `user` | 无管理操作权限，可提交申请 |
| `developer` | 标准关键词相关权限 |
| `admin` | 项目和标准关键词相关权限 |
| `super_admin` | 用户、项目、容量、审批和标准关键词权限 |

权限申请的最终审批只允许 `super_admin`；项目申请审批允许 `admin` 和 `super_admin`。

### 2.4 分页

日志、任务和任务结果使用：

| 参数 | 默认 | 限制 |
| --- | --- | --- |
| `page` | `1` | 最小为 1 |
| `page_size` | `20` | 最大为 200 |

## 3. 健康检查与认证接口

### 3.1 `GET /health`

公开健康检查兼容路径，与 `/api/health` 行为相同。

### 3.2 `GET /api/health`

公开健康检查。成功响应：

```json
{"code":0,"message":"success","data":{"status":"ok"}}
```

### 3.3 `GET /api/auth/feishu-url`

创建有效期 10 分钟的 `feishu_oauth_state` Cookie，返回飞书授权地址：

```json
{"code":0,"message":"success","data":{"url":"https://accounts.feishu.cn/open-apis/authen/v1/authorize?..."}}
```

未配置飞书 App ID 或 Secret 时返回 `500`。

### 3.4 `GET /api/auth/feishu-login`

创建 OAuth state 后，以 HTTP `302` 跳转到飞书授权地址。适合浏览器直接访问。

### 3.5 `GET /api/auth/callback`

查询参数：`code`、`state`。后端校验 state、向飞书换取令牌、读取用户资料、保存角色并创建 `session_token`，成功后 `302` 跳转到 `/`。

state 无效返回 `400`；飞书换取令牌或用户信息失败返回 `502`。

### 3.6 `POST /api/auth/logout`

删除内存中的当前 Session，并清除 `session_token` Cookie。即使当前没有登录也返回成功：

```json
{"code":0,"message":"success","data":null}
```

### 3.7 `GET /api/auth/me`

`GET /api/user/info` 的兼容路径，要求飞书 Session。

### 3.8 `GET /api/user/info`

返回当前飞书用户和实时角色：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "",
    "feishu_open_id": "ou_xxx",
    "name": "张三",
    "email": "user@example.com",
    "avatar": "https://...",
    "role": "user",
    "role_source": "feishu",
    "job_title": "研发工程师"
  }
}
```

## 4. 上传与查询接口

### 4.1 `POST /api/upload-requests`

`POST /api/upload-sessions` 的兼容路径，请求和响应完全相同。新调用统一使用 `/api/upload-sessions`。

### 4.2 `POST /api/upload-sessions`

创建连续上传会话。请求为 JSON，未知字段会导致 `400`。

必填字段：`client_request_id`、`project_name`、`version`、`uploader_email`。后端通过飞书通讯录校验邮箱归属并同步姓名、用户 ID 和邮箱；`uploader_name` 可选，若提供则必须与飞书姓名一致。

```json
{
  "client_request_id": "session-001",
  "device_id": "device-01",
  "port_name": "COM24",
  "baud_rate": 115200,
  "project_id": "1",
  "project_name": "DR2860",
  "version": "1.0.0",
  "test_task_id": "task-001",
  "test_task_name": "稳定性测试",
  "uploader_email": "wushouchao@70mai.com",
  "uploader_name": "张三",
  "scenario_ids": ["scenario-01"],
  "collector_version": "1.0.0",
  "timezone": "Asia/Shanghai"
}
```

成功返回 HTTP `201`：

```json
{
  "code": 0,
  "message": "upload session created",
  "data": {
    "upload_session_id": "uuid",
    "query_code": "DR2860-A1B2C3D4E5",
    "client_request_id": "session-001",
    "project_id": "1",
    "project_name": "DR2860",
    "version": "1.0.0",
    "test_task_id": "task-001",
    "test_task_name": "稳定性测试",
    "uploader_name": "张三",
    "status": "active",
    "created_at": "2026-08-15T10:00:00Z"
  }
}
```

同一上传用户和 `client_request_id` 重复请求时返回原会话。项目不存在返回 `400`。

### 4.3 `POST /api/upload-sessions/{id}/complete`

关闭属于当前上传身份的会话。成功返回：

```json
{"code":0,"message":"upload session closed","data":{"upload_session_id":"uuid","status":"closed"}}
```

会话不存在或不属于当前上传身份时返回 `404`。

### 4.4 `GET /api/upload-config`

要求飞书 Session，返回当前生效的上传限制：

```json
{"code":0,"message":"success","data":{"max_upload_bytes":2147483648,"max_files_per_upload":100}}
```

### 4.5 `GET /api/keywords/sync`

要求上传身份。返回已启用、来源为 `admin_keyword_upload` 的平台标准关键词数组，每项包含 `id`、`name`、`category`、`keyword`、`scope`、`level`、`description` 和 `updated_at`。

### 4.6 `POST /api/logs/inspect`

要求飞书 Session。使用 `multipart/form-data`，必须且只能提交一个 `file`。接口不会创建上传任务。

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "archive": true,
    "entries": [
      {"path":"device/system.log","size_bytes":1024,"encrypted":false}
    ]
  }
}
```

### 4.7 `POST /api/logs/upload`

要求上传身份，使用 `multipart/form-data`。支持 `.log`、`.txt`、`.out`、`.csv`、`.zip`、`.gz`、`.tgz` 和 `.tar.gz`。

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` 或 `files` | 是 | 可重复提交，至少一个文件 |
| `project_name` | 是 | 最多 128 字符，项目必须存在 |
| `version` | 是 | 最多 64 字符 |
| `uploader_name` | 是 | 最多 128 字符 |
| `project_id` | 否 | 平台数字项目 ID |
| `test_task_id` | 否 | 最多 128 字符 |
| `test_task_name` | 否 | 最多 256 字符 |
| `remark` | 否 | 最多 4000 字符 |
| `scenario_ids` | 否 | JSON 字符串数组，最多 20 个 |
| `client_request_id` | 建议 | 幂等键，最多 128 字符 |
| `collector_version`、`timezone` | 否 | 客户端信息 |
| `created_at`、`started_at`、`ended_at` | 否 | RFC 3339 时间 |
| `disable_parsing_rules` | 否 | 布尔值，默认 `true` |
| `upload_session_id`、`query_code` | 会话上传必填 | 必须属于同一上传身份并相互匹配 |
| `config_snapshot` | 会话上传必填 | 必须与创建会话时的配置快照一致 |

请求头 `Idempotency-Key` 优先于表单中的 `client_request_id`。整个 multipart 请求体可使用 gzip，并设置 `Content-Encoding: gzip`。

成功返回 HTTP `202`：

```json
{
  "code": 0,
  "message": "upload accepted",
  "data": {
    "upload_id": "uuid",
    "task_id": "uuid",
    "query_code": "DR2860-A1B2C3D4E5",
    "status": "queued",
    "file_count": 2,
    "scenario_ids": [],
    "client_request_id": "batch-001"
  }
}
```

上传接收后，解压和解析在后台执行。相同用户和幂等键会返回第一次已接收的任务。

### 4.8 `GET /api/query/{query_code}`

公开接口。查询码转为大写后，可使用完整的 `项目-随机码` 或仅使用 10 位随机码。返回会话汇总和批次数组：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "upload_session_id": "uuid",
    "query_code": "DR2860-A1B2C3D4E5",
    "project_name": "DR2860",
    "version": "1.0.0",
    "status": "parsing",
    "batch_count": 2,
    "total_files": 4,
    "processed_files": 2,
    "total_lines": 10000,
    "error_count": 12,
    "warning_count": 35,
    "batches": []
  }
}
```

状态可能为 `uploading`、`queued`、`parsing`、`completed` 或 `failed`。

### 4.9 `POST /api/query/{query_code}/collect`

要求飞书 Session。仅能关联内置采集用户创建的上传会话。成功后，当前用户可通过日志和任务接口访问该会话的数据。

```json
{"code":0,"message":"collected upload session linked","data":{"query_code":"DR2860-A1B2C3D4E5","batch_count":2,"source_type":"collector"}}
```

## 5. 日志、任务和统计接口

### 5.1 `GET /api/logs`

查询参数：`page`、`page_size`、`source_type`。`source_type` 可为空、`collector` 或 `uploaded`。

```json
{"code":0,"message":"success","data":{"total":1,"list":[]}}
```

### 5.2 `GET /api/logs/{upload_id}`

返回当前用户有权访问的上传记录和文件列表：

```json
{"code":0,"message":"success","data":{"upload":{},"files":[]}}
```

### 5.3 `GET /api/logs/{upload_id}/preview`

可选查询参数 `file_id`；未提供时预览第一个文件。最多返回 500 行、2 MiB 内容。

```json
{"code":0,"message":"success","data":{"file_id":1,"relative_path":"items/1/original/system.log","lines":[],"truncated":false}}
```

### 5.4 `GET /api/tasks`

使用通用分页参数，返回 `{total, list}`。

### 5.5 `GET /api/tasks/{task_id}`

返回任务、文件列表以及是否启用了外部 Agent：

```json
{"code":0,"message":"success","data":{"task":{},"files":[],"agent_enabled":false}}
```

### 5.6 `DELETE /api/tasks/{task_id}`

删除当前用户有权访问的任务、数据库关联记录及其存储目录。成功时 `data` 为 `null`。

### 5.7 `GET /api/tasks/{task_id}/results`

使用通用分页参数，返回规则命中数组。结果包含级别、命中文本、行号、文件路径、规则、事件时间、上下文行和关联原因。

### 5.8 `GET /api/tasks/{task_id}/agent-results`

返回 AI/外部 Agent 对各日志文件的分析记录数组。每条记录包含 `id`、`task_id`、`log_file_id`、`file_path`、`provider`、`status`、`summary`、`findings`、`error_message`、`created_at`、`updated_at`。

`provider` 取值：直连大模型为 `llm`，转调外部 Agent 为 `http-agent`；`status` 为 `completed` 或 `failed`。

`findings` 每项字段：

| 字段 | 说明 |
| --- | --- |
| `category` | 分类：system / camera / gps / storage / sensor / network / recording / unknown |
| `severity` | 级别：warning / error / critical |
| `root_cause` | 可能原因 |
| `evidence` | 证据（引用的命中行或上下文） |
| `impact` | 影响（新增） |
| `suggestion` | 建议 |
| `confidence` | 置信度 0~1 |
| `line_number` | 关联的原命中行号（新增，用于单条“AI 解读”定位） |
| `file_path` | 关联的文件路径（新增） |

示例：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "task_id": "uuid",
      "log_file_id": 3,
      "file_path": "items/1/extracted/system.log",
      "provider": "llm",
      "status": "completed",
      "summary": "本次日志整体运行正常，仅在存储链路出现间歇性 I/O 异常",
      "findings": [
        {
          "category": "storage",
          "severity": "error",
          "root_cause": "存储介质读写超时",
          "evidence": "第 120 行附近连续出现 input/output error",
          "impact": "可能导致录像写入失败并丢失片段",
          "suggestion": "检查 SD 卡健康状态、剩余空间与接触可靠性",
          "confidence": 0.9,
          "line_number": 120,
          "file_path": "items/1/extracted/system.log"
        }
      ],
      "error_message": "",
      "created_at": "2026-08-16T10:00:00Z",
      "updated_at": "2026-08-16T10:00:05Z"
    }
  ]
}
```

AI 分析为异步任务：关键字规则解析先同步完成（兜底），AI 结果在后台写入；未完成时本接口只返回已完成的部分（通常为空数组）。

### 5.9 `GET /api/dashboard/stats`

查询参数 `days` 仅支持 `7` 或 `30`，其他值按 7 天处理。返回总行数、错误数、警告数、任务统计、趋势、热门命中和最近任务。

### 5.10 `GET /api/projects`

返回所有启用项目的名称数组，供上传页面选择：

```json
{"code":0,"message":"success","data":["DR1210","DR2860"]}
```

## 6. 规则与测试场景接口

### 6.1 `GET /api/rules`

返回系统规则和当前用户可见的自定义规则。规则包含 `id`、`name`、`category`、`keyword`、`scope`、`level`、`enabled`、`description`、`priority`、`source`、`editable` 和 `scenario_count`。

### 6.2 `POST /api/rules`

当前禁止通过普通规则接口创建规则，固定返回 `403`。标准规则应通过 `/api/admin/keyword-rules/import` 导入。

### 6.3 `PUT /api/rules/batch`

批量设置当前用户的规则启用状态，规则 ID 数量为 1 到 500：

```json
{"ids":[1,2,3],"enabled":true}
```

返回 `{"updated":3}`。

### 6.4 `PUT /api/rules/{id}`

更新当前用户可编辑的规则。请求体使用完整规则对象；规则 ID 以路径参数为准。

### 6.5 `DELETE /api/rules/{id}`

删除当前用户可编辑的规则。规则被测试场景引用时返回 `409`。

### 6.6 `GET /api/scenarios`

返回测试场景数组。

### 6.7 `POST /api/scenarios`

创建测试场景，`id` 和 `name` 必填。后端校验场景引用规则的可见性并保存规则快照。

### 6.8 `GET /api/scenarios/{id}`

返回指定测试场景，不存在时返回 `404`。

### 6.9 `PUT /api/scenarios/{id}`

更新测试场景，路径中的 ID 覆盖请求体 ID，并重新校验规则快照。

### 6.10 `DELETE /api/scenarios/{id}`

删除指定测试场景。

### 6.11 `PATCH /api/scenarios/{id}/enabled`

请求体：

```json
{"enabled":true}
```

返回更新后的场景。

场景主要字段包括 `id`、`name`、`description`、`remark`、`enabled`、`all_projects`、`projects`、`keywords`、`color`、`judgement`、`metadata` 和 `checks`。

## 7. 用户、权限申请与运行日志

### 7.1 `GET /api/admin/users`

仅 `super_admin`。返回用户数组，包含飞书信息、角色、角色来源、职务、是否当前用户和时间字段。

### 7.2 `PUT /api/admin/users/{id}/role`

仅 `super_admin`。请求体：

```json
{"role":"developer"}
```

角色必须是 `user`、`developer`、`admin` 或 `super_admin`。不能降级系统中最后一个超级管理员。

### 7.3 `PUT /api/admin/users/{id}/restore-feishu-role`

仅 `super_admin`。将用户角色来源恢复为 `feishu`，再根据 Open ID 和职务规则重新计算角色。

### 7.4 `GET /api/admin/permission-requests`

普通用户、开发者和管理员只看到自己的申请；超级管理员看到全部申请。返回：

```json
{"code":0,"message":"success","data":{"requests":[],"current_role":"user","can_apply":true,"can_review":false}}
```

### 7.5 `POST /api/admin/permission-requests`

超级管理员不能申请。其他用户提交：

```json
{"requested_role":"developer","reason":"需要维护解析规则"}
```

原因必填且最多 1000 字；同一用户只能存在一个待处理申请。成功返回 HTTP `201`。

### 7.6 `DELETE /api/admin/permission-requests/{id}`

申请人撤回自己的待处理申请。申请已处理或不属于当前用户时返回 `409`。

### 7.7 `PUT /api/admin/permission-requests/{id}/decision`

仅 `super_admin`，不能审批自己的申请：

```json
{"action":"approve","comment":"同意"}
```

`action` 为 `approve` 或 `reject`，意见最多 1000 字。

### 7.8 `GET /api/admin/runtime-logs`

返回最多 500 条最新运行日志。`user` 和 `developer` 只看到自己的记录；`admin` 和 `super_admin` 查看全部记录。

## 8. 项目申请、项目和属性选项

### 8.1 `GET /api/admin/project-requests`

`user` 和 `developer` 只看到自己的申请；`admin` 和 `super_admin` 查看全部申请。返回 `{requests, can_review}`。

### 8.2 `POST /api/admin/project-requests`

仅 `user` 和 `developer` 可提交；管理员应直接创建项目。

```json
{
  "name": "DR2860",
  "product_line": "vehicle",
  "product_type": "dashcam",
  "stage": "production",
  "description": "项目说明",
  "reason": "联调需要"
}
```

项目名转为大写，仅支持字母、数字和连字符；属性代码必须是已启用选项；说明和原因最多 1000 字。成功返回 HTTP `201`。

### 8.3 `PUT /api/admin/project-requests/{id}/decision`

仅 `admin` 和 `super_admin`：

```json
{"action":"approve","comment":"同意"}
```

拒绝时 `comment` 必填。批准后在同一事务中创建项目。

### 8.4 `GET /api/admin/projects`

仅 `admin` 和 `super_admin`，返回启用项目的完整对象数组。

### 8.5 `POST /api/admin/projects`

仅 `admin` 和 `super_admin`。请求体字段为 `name`、`product_line`、`product_type`、`stage` 和 `description`。属性代码必须存在，成功返回 HTTP `201`。

### 8.6 `PUT /api/admin/projects/{id}`

仅 `admin` 和 `super_admin`，使用与创建项目相同的请求体更新项目。

### 8.7 `DELETE /api/admin/projects/{id}`

仅 `admin` 和 `super_admin`。执行软删除，将 `is_active` 设为 `false`，不删除历史日志。

### 8.8 `GET /api/admin/project-options`

任意飞书登录用户可访问。返回：

```json
{"code":0,"message":"success","data":{"lines":[],"types":[],"stages":[]}}
```

每个选项包含 `id`、`kind`、`code`、`name` 和 `is_system`。

### 8.9 `POST /api/admin/project-options`

仅 `admin` 和 `super_admin`：

```json
{"kind":"line","name":"车载线"}
```

`kind` 只能是 `line`、`type` 或 `stage`，名称最多 64 字。成功返回 HTTP `201`。

### 8.10 `PUT /api/admin/project-options/{id}`

仅 `admin` 和 `super_admin`，请求体与创建选项相同。

### 8.11 `DELETE /api/admin/project-options/{id}`

仅 `admin` 和 `super_admin`。系统预置选项及正在被启用项目使用的选项不能删除；删除为软删除。

## 9. 容量与标准关键词管理

### 9.1 `GET /api/admin/upload-capacity`

仅 `super_admin`。返回 `max_upload_bytes`、`max_files_per_upload` 和可选的 `updated_at`。

### 9.2 `PUT /api/admin/upload-capacity`

仅 `super_admin`：

```json
{"max_upload_bytes":2147483648,"max_files_per_upload":100}
```

容量范围为 1 MiB 到 100 GiB，文件数范围为 1 到 500。

### 9.3 `GET /api/admin/ai-analysis-settings` 与 `PUT /api/admin/ai-analysis-settings`

仅 `super_admin`。用于查看和修改 AI 模型连接、输入采样和分析限额。

`GET` 返回：

```json
{"code":0,"message":"success","data":{"max_tokens_per_file":20000,"daily_token_quota":1000000}}
```

`PUT` 请求：

```json
{"max_tokens_per_file":20000,"daily_token_quota":1000000}
```

字段说明：

| 字段 | 范围 | 含义 |
| --- | --- | --- |
| `max_tokens_per_file` | 1 到 1000000 | 单个日志文件的 AI 模型最大输出 token 数 |
| `daily_token_quota` | `>= 0` | 每个用户每天的 AI token 配额，`0` 表示不限制 |
| `llm_api_base_url` | HTTP(S) URL | 大模型 OpenAI 兼容接口地址 |
| `llm_api_key` | PUT 时可选 | 新 API Key；GET 不返回明文 |
| `llm_api_key_configured` | 只读 | 是否已配置 API Key |
| `llm_model` | 1 到 128 字符 | 模型名称 |
| `llm_timeout_seconds` | 5 到 600 | 单次请求超时时间 |
| `llm_max_matches` | 1 到 5000 | 按规则/关键词轮询采样的命中条数上限 |
| `llm_max_input_bytes` | 1024 到 10485760 | 单次模型输入大小上限 |

超过每日配额时，对应文件跳过 AI 分析（关键字规则仍正常执行），AI 结果记录为 `failed` 并提示 quota exceeded。

### 9.4 `GET /api/admin/keyword-rules`

要求关键词管理权限。返回管理员上传的标准关键词规则数组。

### 9.5 `POST /api/admin/keyword-rules/import`

要求关键词管理权限。使用 `multipart/form-data`：

| 字段 | 必填 | 默认值或限制 |
| --- | --- | --- |
| `file` | 是 | `.txt` 或 `.csv`，最大 2 MiB |
| `category` | 否 | `system` |
| `level` | 否 | `critical` |
| `scope` | 否 | `全局` |
| `description` | 否 | 最多 1000 字 |

单次最多导入 1000 条。CSV 必须包含 `keyword` 或 `关键词` 列，可选 `name`、`category`、`level`、`scope` 和 `description`。成功返回 HTTP `201`，`data` 包含 `created`、`updated`、`skipped` 和 `rules`。

### 9.6 `DELETE /api/admin/keyword-rules/{id}`

要求关键词管理权限。规则被测试场景引用时返回 `409`；不存在或不是管理员上传规则时返回 `404`。

## 10. AI 分析与外部 Agent 接口

后端在解析完成后以**异步任务**方式调用 AI/Agent 分析；关键字规则解析作为兜底始终先行同步完成。两种启用方式二选一，都未配置时 AI 分析不启用。

### 10.1 直连大模型（推荐）

配置 `LLM_API_BASE_URL` 后启用。后端直接调用 OpenAI 兼容的 `/chat/completions` 接口，通义千问、DeepSeek、OpenAI、Moonshot 以及本地 Ollama 的 OpenAI 兼容端点均适用。

配置项：

| 环境变量 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- |
| `LLM_API_BASE_URL` | 是 | 空 | OpenAI 兼容端点，末尾不带 `/chat/completions` |
| `LLM_API_KEY` | 视服务 | 空 | API Key；Ollama 可留空 |
| `LLM_MODEL` | 否 | `qwen-plus` | 模型名 |
| `LLM_TIMEOUT_SECONDS` | 否 | `120` | 单次分析超时 |
| `LLM_MAX_MATCHES` | 否 | `50` | 单次最多送入模型的命中条数，按规则/关键词分组轮询采样 |
| `LLM_MAX_INPUT_BYTES` | 否 | `200000` | 单次送入模型的最大输入字节数 |
| `AI_MAX_TOKENS_PER_FILE` | 否 | `20000` | 单文件 AI 模型最大输出 token 数（回退默认值，管理员可在后台改） |
| `AI_DAILY_TOKEN_QUOTA` | 否 | `1000000` | 每用户每日 token 配额（回退默认值，`0` 不限） |

结果存入 `agent_analyses` 表，`provider` 记为 `llm`，通过 `/api/tasks/{task_id}/agent-results` 读取。

### 10.2 转调外部 Agent（旧方式）

配置 `AGENT_ANALYSIS_URL` 后启用。后端在每个日志文件完成本地解析后异步发送 HTTP `POST`：

```json
{
  "task_id": "uuid",
  "upload_id": "uuid",
  "file": {
    "id": 1,
    "relative_path": "items/1/extracted/system.log",
    "size_bytes": 1024,
    "sha256": "...",
    "line_count": 200
  },
  "total_lines": 200,
  "matches": []
}
```

期望响应：

```json
{
  "summary": "分析摘要",
  "findings": [
    {
      "category": "recording",
      "severity": "error",
      "root_cause": "原因",
      "suggestion": "建议",
      "evidence": "证据",
      "impact": "影响",
      "confidence": 0.92,
      "line_number": 120,
      "file_path": "items/1/extracted/system.log"
    }
  ]
}
```

配置项为 `AGENT_ANALYSIS_URL`、`AGENT_ANALYSIS_TOKEN` 和 `AGENT_ANALYSIS_TIMEOUT_SECONDS`，结果 `provider` 记为 `http-agent`。

两种方式的失败都会单独持久化为 `failed` 记录，不会使本地解析任务失败。
