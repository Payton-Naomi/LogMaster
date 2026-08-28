# LogMaster 后端与采集端 API 文档

> **2026/08/26 更新**：新增采集端项目、已发布测试场景及统一配置快照同步接口。

> **824 调整（2026-08-24）**：创建上传会话成功响应新增只读 `data.uploader_job_title`。后端通过飞书详情自动解析并同步职位，采集端无需提交、保存或依赖该字段；上传、鉴权和查询协议不变。

采集端上传时，`test_task_id` 会严格匹配服务端测试场景 ID，`test_task_name` 在没有 ID 时按唯一名称匹配。采集端已同步的测试任务不会因缺少场景而静默使用其他规则；匹配失败返回 HTTP 400。完全未选择测试任务时，后端使用全部已启用的平台关键字，但排除 `FATAL/ERROR/WARNING/WARN` 通用规则。

采集端上传会话默认关闭 AI 分析。采集端无需新增字段；服务端在创建会话时固定保存 `ai_analysis_enabled=false`，会话下的文件解析只执行关键字规则，不创建 AI 文件分析或任务总览作业。

解析任务的并发和容量由后端环境变量控制：`MAX_PARSE_WORKERS`、`MAX_PARSE_PER_USER`、`MAX_PARSE_PER_PROJECT`、`MAX_FILES_PER_PARSE_TASK`、`MAX_BYTES_PER_PARSE_TASK`。采集端无需调整请求。外包或非飞书账号不会收到飞书通知。

上传人同步时按职位自动分配角色：`主任` 为 `super_admin`，`高级` 为 `admin`，`软件工程师` 或 `硬件工程师` 为 `developer`，其他为 `user`。采集端请求字段不变，人工角色不会被同步覆盖。
AI 分析和任务总览已改为 PostgreSQL 持久化队列，服务重启后可恢复；采集端不需要提交新的字段或调用新接口。通过原查询接口看到的上传/解析状态不受 `ai_status` 影响。
服务端新增 AI 单文件重试、AI 取消和失败分类，均由 Web 任务管理使用；采集端无需新增字段或调用接口，上传、查询码和状态查询协议不变。
任务暂停和恢复接口仅供 Web 使用；采集端通过原查询接口可能看到 `paused` 状态，但上传协议不变。
任务和项目优先级仅影响服务端 Worker 领取顺序，采集端上传协议不变。
批量任务和异常负责人接口仅供 Web 使用，采集端上传协议不变。
采集端上传协议不变；任务容量超限时可通过原查询接口读取 `failed` 和中文错误信息，并发达到上限时状态保持 `queued`。
异常操作审计仅供 Web 使用，采集端上传协议不变。
解析结果和任务报告下载仅供 Web 使用，采集端上传协议不变。

> **822 调整（2026-08-22）**：采集端发送 `uploader_email` 后，后端使用飞书 `tenant_access_token` 校验企业成员；校验成功会自动注册或更新用户，并将本次采集会话授权给该用户。采集端请求字段、认证方式和接口路径不变。

AI 单独重试接口仅供 Web 任务管理使用，采集端上传协议不变，也不需要调用 `agent-retry`。

本文档记录 `collector/collector` 对接的后端 API，更新日期为 2026-08-23。后端全部接口见 [`backend-api.md`](backend-api.md)。

> **最新：821 调整（2026-08-21）**：后端新增持久化解析任务队列和受控 Worker，采集端上传请求、配置、字段和响应处理均不变；文件级 AI 分析后仍会在服务端自动生成任务级总览。

> **最新：818 调整（2026-08-18）**：连续上传会话创建必须提交 `uploader_email`；后端先通过飞书通讯录校验企业归属和账号状态，再同步姓名、用户 ID 与邮箱，`uploader_name` 可选。

> **818 补充**：AI 用量记录权限已通过 `027_ai_usage_permissions.sql` 修复，采集端 API 契约不变。

## 1. 采集端 API 清单

### 1.0 上传后任务调度说明

采集端不需要调用任务取消接口。取消由 Web 后端调用 `POST /api/tasks/{task_id}/cancel` 完成，采集端原有上传字段、认证方式和轮询接口保持不变。
原始日志搜索接口仅供 Web 日志查看使用，采集端不需要调用 `GET /api/logs/{upload_id}/search`，上传协议保持不变。
日志下载接口仅供 Web 使用，采集端不需要调用 `GET /api/logs/{upload_id}/download` 的 `file`、`batch`、`original` 或 `results` 类型，上传协议保持不变。
采集端无需传递解压密码。后端会自动尝试管理员维护的密码，不使用内置默认密码；失败时上传接口或任务查询接口返回中文错误信息。ZIP 解压后的 70mai `logfile` 系列设备日志，以及采集端直接上传的设备编码日志，均由后端自动检测、解码后再解析，采集端上传协议无需改变。
异常结果状态接口仅供 Web 端使用，采集端上传协议不变。
异常结果备注接口仅供 Web 端使用，采集端上传协议不变。
异常结果负责人分配接口仅供 Web 端使用，采集端上传协议不变。
分析对比接口仅供 Web 端使用，采集端上传协议不变。
任务服务端分页筛选仅供 Web 端使用，采集端上传协议不变。

采集端上传成功后，后端仍返回 `202` 和 `status=queued`。后端会将准备/解压及规则解析任务写入 PostgreSQL 队列，由受控 Worker 执行，并通过现有 `GET /api/query/{query_code}` 返回任务进度。采集端不需要新增任务队列、重试或取消字段；任务重试接口仅供 Web 用户使用，采集端上传契约不变。

> **8/16新增**：无
> **8/16之前**：全部接口

| 方法 | 路径 | 访问要求 | 采集端用途 | 日期 |
| --- | --- | --- | --- | --- |
| `GET` | `/api/health` | 公开 | 检查后端连接 | 8/16之前 |
| `POST` | `/api/upload-requests` | 上传 Token | 创建上传会话的兼容路径 | 8/16之前 |
| `POST` | `/api/upload-sessions` | 上传 Token | 创建连续上传会话和查询码 | 8/16之前 |
| `POST` | `/api/upload-sessions/{id}/complete` | 上传 Token | 关闭连续上传会话 | 8/16之前 |
| `GET` | `/api/keywords/sync` | 上传 Token | 同步平台标准关键词 | 8/16之前 |
| `POST` | `/api/logs/inspect` | 当前不支持上传 Token | 上传前检查文件 | 8/16之前 |
| `POST` | `/api/logs/upload` | 上传 Token | 上传日志批次 | 8/16之前 |
| `GET` | `/api/query/{query_code}` | 公开 | 查询会话和批次状态 | 8/16之前 |

## 2. 采集端调用约定

### 2.1 后端地址

`backend.base_url` 必须是绝对 HTTP(S) 地址并以 `/api` 结尾：

```yaml
backend:
  base_url: "http://127.0.0.1:8080/api"
  health_path: "/health"
  inspect_path: "/logs/inspect"
  upload_path: "/logs/upload"
  inspect_before_upload: false
```

上传最终请求地址为 `http://127.0.0.1:8080/api/logs/upload`。

### 2.2 Bearer Token

除健康检查和查询码查询外，采集接口发送：

```http
Authorization: Bearer <token>
```

后端支持：

- 内置 Token：`logmaster-internal-collector-v1`，数据归属内置采集用户。
- 自定义 `LOGMASTER_UPLOAD_TOKEN`，必须同时配置 `LOGMASTER_UPLOAD_OWNER_OPEN_ID`。

生产环境通过采集端 `authorization_token_env` 读取 Token，不要把 Token 写入 YAML 或提交到仓库。

### 2.3 统一响应

```json
{"code":0,"message":"success","data":{}}
```

采集端必须同时校验 HTTP 状态码、`code` 和业务确认字段，不能只判断请求没有报错。

## 3. 接口逐项说明

### 3.1 `GET /api/health`

不要求 Token。成功必须同时满足 HTTP `200`、`code=0` 和 `data.status="ok"`：

```json
{"code":0,"message":"success","data":{"status":"ok"}}
```

### 3.2 `POST /api/upload-requests`

`POST /api/upload-sessions` 的兼容路径，请求、鉴权和响应完全相同。新采集端统一使用 `/api/upload-sessions`。

### 3.3 `POST /api/upload-sessions`

创建连续上传会话。请求头：

```http
Content-Type: application/json
Authorization: Bearer <token>
```

必填字段：

| 字段 | 限制 |
| --- | --- |
| `client_request_id` | 非空，最多 128 字符，同一上传用户下用于幂等 |
| `project_name` | 非空，最多 128 字符，必须是启用项目 |
| `version` | 非空，最多 64 字符 |
| `uploader_email` | 必填，必须匹配飞书当前企业通讯录中的有效邮箱，最多 320 字符 |
| `uploader_name` | 可选；提供时必须与邮箱对应用户姓名一致，最多 128 字符 |

完整请求可包含：

```json
{
  "client_request_id": "session-001",
  "device_id": "device-01",
  "name": "COM24",
  "port_name": "COM24",
  "baud_rate": 115200,
  "data_bits": 8,
  "stop_bits": 1,
  "parity": "none",
  "handshake": "none",
  "dtr": false,
  "rts": false,
  "project_id": "1",
  "project_name": "DR2860",
  "version": "1.0.0",
  "test_task_id": "task-001",
  "test_task_name": "稳定性测试",
  "uploader_email": "wushouchao@70mai.com",
  "uploader_name": "张三",
  "remark": "连续采集",
  "scenario_ids": ["scenario-01"],
  "keyword_profile_id": "default",
  "keyword_rule_ids": ["1", "2"],
  "keyword_matching_enabled": true,
  "save_enabled": true,
  "upload_enabled": true,
  "no_log_timeout_seconds": 60,
  "vid": "1234",
  "pid": "5678",
  "usb_serial": "ABC123",
  "location": "USB-1",
  "collector_version": "1.0.0",
  "timezone": "Asia/Shanghai"
}
```

`project_id` 只接受纯数字；非数字值会被清空并按 `project_name` 查找项目。未知 JSON 字段返回 `400`。

成功为 HTTP `201`：

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

相同上传用户和 `client_request_id` 重复请求时返回第一次创建的会话，不生成新查询码。

### 3.4 `POST /api/upload-sessions/{id}/complete`

关闭属于当前 Token 上传用户的会话。成功：

```json
{"code":0,"message":"upload session closed","data":{"upload_session_id":"uuid","status":"closed"}}
```

不存在或不属于当前上传用户时返回 `404`。关闭后不影响已经接收的批次继续解析。

### 3.5 `GET /api/keywords/sync`

要求上传 Token。返回已启用的管理员标准关键词数组：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "name": "设备断连",
      "category": "connection",
      "keyword": "disconnect",
      "scope": "line",
      "level": "warning",
      "description": "",
      "updated_at": "2026-08-15T10:00:00Z"
    }
  ]
}
```

当前上传 Client 尚未自动调用该接口，需要由配置同步功能显式接入。

### 3.6 `POST /api/logs/inspect`

采集 Client 已定义此调用，但当前后端只接受飞书 Cookie Session，不接受上传 Bearer Token。采集端开启预检会收到 `401`。

当前配置必须保持：

```yaml
backend:
  inspect_before_upload: false
```

后端未来支持上传 Token 后，请求应为 `multipart/form-data`，且只能包含一个 `file`。

### 3.7 `POST /api/logs/upload`

上传一个日志批次，使用 `multipart/form-data` 和 Bearer Token。

请求头：

```http
Authorization: Bearer <token>
Idempotency-Key: <client_request_id>
Content-Type: multipart/form-data; boundary=...
```

启用整体请求压缩时增加：

```http
Content-Encoding: gzip
```

表单字段：

| 字段 | 必填 | 限制或说明 |
| --- | --- | --- |
| `file` | 是 | 一个或多个文件，重复使用相同字段名 |
| `project_name` | 是 | 最多 128 字符，项目必须存在 |
| `version` | 是 | 最多 64 字符 |
| `uploader_name` | 是 | 最多 128 字符 |
| `upload_session_id` | 会话模式必填 | 创建会话返回的 ID |
| `query_code` | 会话模式必填 | 必须与会话匹配 |
| `config_snapshot` | 会话模式必填 | JSON 内容必须与创建会话时一致 |
| `project_id` | 否 | 只发送平台数字项目 ID |
| `test_task_id` | 否 | 最多 128 字符 |
| `test_task_name` | 否 | 最多 256 字符 |
| `remark` | 否 | 最多 4000 字符 |
| `client_request_id` | 建议 | 最多 128 字符，与幂等请求头一致 |
| `collector_version`、`timezone` | 否 | 客户端信息 |
| `created_at`、`started_at`、`ended_at` | 否 | RFC 3339 时间 |
| `scenario_ids` | 否 | JSON 数组字符串，最多 20 个 |

会话模式下，项目、版本、测试任务、上传人和 `config_snapshot` 必须与创建会话时一致。没有 `upload_session_id` 时不能自行发送 `query_code`。

支持 `.log`、`.txt`、`.out`、`.csv`、`.zip`、`.gz`、`.tgz` 和 `.tar.gz`。

成功为 HTTP `202`：

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
    "scenario_ids": ["scenario-01"],
    "client_request_id": "batch-001"
  }
}
```

采集端只有在以下条件全部满足时才能标记批次已上传：

- HTTP 状态为 `202`。
- `code=0`。
- `upload_id` 和 `task_id` 非空。
- `file_count` 等于本批文件数。
- 返回的 `client_request_id` 为空或与请求一致。
- 会话模式下返回的 `query_code` 与本地一致。

相同上传用户和 `Idempotency-Key` 会返回第一次已接收任务的信息。

### 3.8 `GET /api/query/{query_code}`

公开接口，不要求 Token。可传完整查询码或 10 位随机后缀，返回会话汇总和所有批次：

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

## 4. 错误处理和重试

| HTTP 状态 | 采集端处理 |
| --- | --- |
| `400` | 请求被拒绝，修正字段或文件，不自动重试 |
| `401` / `403` | 暂停上传，检查 Token 和归属用户 |
| `413` | 拆分批次；单文件仍超限时标记失败 |
| `429` | 按 `Retry-After` 延迟重试 |
| `500` / `502` / `503` / `504` | 使用退避策略重试 |

请求体尚未发出时失败可以安全重试。请求体已经发出但未获得有效确认时属于 `uncertain`，必须保留原 `Idempotency-Key` 核对，不能创建新请求 ID 盲目重传。

## 5. 采集端联调检查

- `backend.base_url` 以 `/api` 结尾。
- Token 从环境变量读取，后端已配置对应上传用户。
- 会话和批次分别使用稳定且不同的 `client_request_id`。
- 会话模式下查询码、元数据和配置快照保持一致。
- 只有完整校验 `202` 响应后才标记批次上传成功。
- 当前保持 `inspect_before_upload=false`。
- 断网时继续本地落盘，恢复后队列继续上传。
- `uncertain` 批次不更换幂等键重传。
## 6. Collector configuration synchronization

The collector can use `GET /api/projects/sync` for active projects, `GET /api/scenarios/sync` for published test scenarios, or `GET /api/collector/sync` for one response snapshot containing `projects`, `scenarios`, `keywords`, and UTC `synced_at`. All endpoints use the same upload-token authentication as upload sessions and return `401` for invalid credentials. Projects include `id` and `name`; scenarios include `id`, `name`, `enabled`, applicable projects, and keywords. The server validates the selected project and scenario again when creating an upload session.
