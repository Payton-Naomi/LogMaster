# LogMaster 后端与采集端 API 文档

本文档记录 `collector/collector` 对接的后端 API，更新日期为 2026-08-15。后端全部接口见 [`backend-api.md`](backend-api.md)。

## 1. 采集端 API 清单

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
| `uploader_name` | 非空，最多 128 字符 |

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
