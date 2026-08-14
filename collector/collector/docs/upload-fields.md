# 云端上传字段规范

## 1. 文档目的

本文档定义 LogMaster 采集端与平台端之间的云端上传字段、字段来源、必填规则和生命周期。

本文档是连续上传会话、上传字段和查询码生命周期的统一依据。当前已实现 `POST /api/upload-sessions` 创建会话，后续多个 `POST /api/logs/upload` 批次复用同一查询码。

> 查询码属于单个串口的连续上传会话。程序重启、串口短暂断开和暂停恢复不更换查询码；关闭上传或修改会话配置快照后创建新会话。

目标场景：

- 一台 Windows 电脑可同时连接多台记录仪。
- 不追踪同一记录仪跨电脑使用，不上传设备码。
- 用户通过平台返回的查询码查看上传和分析结果。
- 关键字方案 ID、已选规则 ID 和匹配开关随通道快照上传；具体匹配表达式仍由采集端维护。
- 平台接收关键字命中时间、命中序号、命中行以及前后各 10 行证据。

## 2. 核心约定

### 2.1 上传任务是查询单位

一次上传任务对应一个查询码，可以包含多个采集通道、多个日志分段文件和多条关键字命中证据。

```text
上传任务（一个查询码）
  通道 1
    日志文件 1
    日志文件 2
    命中证据
  通道 2
    日志文件 1
    命中证据
```

查询码属于上传任务，不属于单个文件、单个通道或单条命中事件。

### 2.2 上传通道配置快照

平台保存通道 ID、串口名、串口参数和 USB 识别信息作为会话配置快照。这些字段用于还原采集上下文，不作为跨电脑资产身份。

### 2.3 关键字配置边界

上传关键字方案 ID、规则 ID 列表和匹配开关。关键字文本、正则表达式及具体匹配条件不随上传会话发送。

平台只接收采集端生成的命中证据。

## 3. 用户配置字段

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `project_id` | string | 是 | 平台项目稳定 ID，界面显示项目名称 |
| `project_name` | string | 是 | 项目名称快照；V2 平台应以 `project_id` 为准 |
| `version` | string | 是 | 本次日志对应的业务、固件或产品版本 |
| `test_task_id` | string | 否 | 平台测试任务 ID |
| `test_task_name` | string | 否 | 测试任务名称快照 |
| `uploader_name` | string | 是 | 本次执行上传的人员姓名 |
| `uploader_id` | string | 否 | 有登录体系时由平台身份自动提供，不要求用户填写 |
| `remark` | string | 否 | 本次测试备注 |

规则：

- 本地采集不要求填写上述字段。
- 打开“上传云端”时，必须校验项目、版本和上传人。
- `uploader_name` 去除首尾空白后不能为空。
- 上传人不参与查询权限和幂等判断。
- 采集端可以记住上次填写的上传人姓名，但每次创建任务时保存独立快照。

## 4. 采集端自动生成字段

### 4.1 上传任务字段

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `local_job_id` | string | 是 | 采集端本地上传任务 ID，不要求平台使用 |
| `client_request_id` | string | 是 | 创建平台任务的幂等 ID；重试时保持不变 |
| `collector_version` | string | 是 | 采集端程序版本 |
| `timezone` | string | 是 | IANA 时区，例如 `Asia/Shanghai` |
| `created_at` | datetime | 是 | 本地任务创建时间，RFC 3339 |
| `started_at` | datetime | 否 | 开始采集时间，RFC 3339 |
| `ended_at` | datetime | 否 | 停止采集时间，RFC 3339 |

所有时间上传时必须包含时区；推荐统一发送 UTC，并保留 `timezone` 用于平台展示。

### 4.2 通道来源字段

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `source_id` | string | 是 | 任务内唯一，例如 `channel-1` |
| `source_name` | string | 是 | 展示名称，例如“通道 1” |
| `session_id` | string | 是 | 该通道本次采集会话 ID |
| `started_at` | datetime | 是 | 通道开始采集时间 |
| `ended_at` | datetime | 否 | 通道停止采集时间 |

`source_id` 只要求在同一个上传任务内唯一，不承担跨任务识别设备的职责。

## 5. 日志文件字段

每个封口后的日志文件提供以下字段：

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `file_id` | string | 是 | 采集端生成的文件 ID |
| `source_id` | string | 是 | 所属通道来源 |
| `session_id` | string | 是 | 所属采集会话 |
| `file_name` | string | 是 | 文件名，不含本地目录 |
| `size_bytes` | integer | 是 | 原始文件字节数 |
| `sha256` | string | 是 | 原始文件 SHA-256，小写十六进制 |
| `first_sequence` | integer | 是 | 文件第一条逻辑日志序号 |
| `last_sequence` | integer | 是 | 文件最后一条逻辑日志序号 |
| `line_count` | integer | 是 | 文件逻辑日志行数 |
| `captured_from` | datetime | 是 | 文件第一条日志采集时间 |
| `captured_to` | datetime | 是 | 文件最后一条日志采集时间 |

禁止上传本地绝对路径，例如 `D:\logs\...`。平台无法使用该路径，而且路径可能包含用户或电脑信息。

文件唯一性建议使用 `task_id + sha256`，不能只依赖文件名。

## 6. 关键字命中证据字段

每次命中生成一条证据事件。同一日志行同时命中多个本地规则时，只生成一条证据，避免重复上传相同上下文。

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `event_id` | string | 是 | 采集端生成的事件 ID |
| `source_id` | string | 是 | 所属通道来源 |
| `session_id` | string | 是 | 所属采集会话 |
| `file_id` | string | 是 | 命中行所在日志文件 |
| `file_sha256` | string | 是 | 所属文件摘要，用于平台核对 |
| `matched_sequence` | integer | 是 | 命中行逻辑序号 |
| `matched_at` | datetime | 是 | 命中行采集时间 |
| `matched_text` | string | 是 | 命中行完整文本；超长时按约定截断 |
| `context_before` | array | 是 | 命中前最多 10 行 |
| `context_after` | array | 是 | 命中后最多 10 行 |
| `before_count` | integer | 是 | 实际前置行数，范围 0-10 |
| `after_count` | integer | 是 | 实际后置行数，范围 0-10 |
| `context_complete` | boolean | 是 | 是否成功取得完整前后文 |
| `truncated` | boolean | 是 | 命中行或上下文是否发生长度截断 |

上下文中的每一行使用统一结构：

```json
{
  "sequence": 12883,
  "captured_at": "2026-07-31T07:20:31.120Z",
  "text": "previous log line"
}
```

定位一条日志时使用 `session_id + sequence`，不使用会因文件分段而重置的普通文件行号。

停止采集、断开串口或退出程序时，如果命中事件尚未收满后 10 行，仍需保存和上传，并设置：

```json
{
  "after_count": 3,
  "context_complete": false
}
```

## 7. 分批上传字段

一个上传任务可能包含多个文件并持续较长时间，因此允许分批上传。

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `task_id` | string | 是 | 平台上传任务 ID |
| `part_id` | string | 是 | 当前分片幂等 ID；同一分片重试时保持不变 |
| `part_index` | integer | 是 | 分片顺序，从 1 开始 |
| `file_count` | integer | 是 | 当前分片文件数 |
| `event_count` | integer | 是 | 当前分片命中事件数 |

平台必须对 `task_id + part_id` 建立唯一约束。重复请求应返回第一次成功接收的结果，不得重复保存文件或事件。

## 8. 平台返回字段

### 8.1 创建任务响应

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `task_id` | string | 是 | 平台上传和分析任务 ID |
| `query_code` | string | 是 | 用户查询码 |
| `status` | string | 是 | 初始状态，通常为 `uploading` |
| `created_at` | datetime | 是 | 平台任务创建时间 |

### 8.2 分片接收响应

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `upload_id` | string | 是 | 当前分片接收记录 ID |
| `part_id` | string | 是 | 对应客户端分片 ID |
| `accepted_file_count` | integer | 是 | 平台确认接收的文件数 |
| `accepted_event_count` | integer | 是 | 平台确认接收的事件数 |
| `received_at` | datetime | 是 | 平台接收时间 |

客户端只有在数量一致、响应完整时，才能将当前分片标记为已上传。

### 8.3 完成任务响应

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `task_id` | string | 是 | 平台任务 ID |
| `query_code` | string | 是 | 原查询码，不得重新生成 |
| `status` | string | 是 | `analyzing`、`completed` 或失败状态 |
| `completed_at` | datetime | 否 | 平台完成时间 |

## 9. 推荐接口

### 9.1 云端连通性检查

```http
GET /api/health
Authorization: Bearer <token>
```

连通性检查只验证网络、鉴权和接口可用性，不创建任务，不生成查询码。

### 9.2 创建上传任务

```http
POST /api/upload-tasks
Authorization: Bearer <token>
Idempotency-Key: <client_request_id>
Content-Type: application/json
```

示例请求：

```json
{
  "project_id": "project-a",
  "project_name": "Project A",
  "version": "V1.2.3",
  "test_task_id": "",
  "test_task_name": "",
  "uploader_id": "",
  "uploader_name": "张三",
  "remark": "高温重启测试",
  "collector_version": "0.0.3",
  "timezone": "Asia/Shanghai",
  "created_at": "2026-07-31T07:00:00Z",
  "sources": [
    {
      "source_id": "channel-1",
      "source_name": "通道 1",
      "session_id": "session-001",
      "started_at": "2026-07-31T07:00:02Z"
    },
    {
      "source_id": "channel-2",
      "source_name": "通道 2",
      "session_id": "session-002",
      "started_at": "2026-07-31T07:00:03Z"
    }
  ]
}
```

示例响应：

```json
{
  "code": 0,
  "message": "accepted",
  "data": {
    "task_id": "task_xxx",
    "query_code": "7K4M9P2QAF",
    "status": "uploading",
    "created_at": "2026-07-31T07:00:04Z"
  }
}
```

### 9.3 上传分片

```http
POST /api/upload-tasks/{task_id}/parts
Authorization: Bearer <token>
Idempotency-Key: <part_id>
Content-Type: multipart/form-data
```

Multipart 字段：

| 字段 | 说明 |
| --- | --- |
| `manifest` | JSON，包含分片字段、文件元数据和命中证据 |
| `file` | 可重复的日志文件字段 |

### 9.4 完成上传任务

```http
POST /api/upload-tasks/{task_id}/finalize
Authorization: Bearer <token>
```

平台完成文件数量、SHA-256 和事件引用校验后，才进入分析流程。

### 9.5 使用查询码查询

```http
GET /api/query/{query_code}
```

平台查询结果按 `source_id` 分组展示日志文件、命中证据和分析结果。

## 10. 查询码规则

- 查询码由平台生成，采集端不得自行生成展示码。
- 一个上传任务在整个生命周期中只使用一个查询码。
- 暂停和继续上传沿用原任务和原查询码。
- 只有明确取消原任务并创建新任务时，才生成新查询码。
- 查询码建议至少包含 64 位随机熵，使用 10-12 位以上不易混淆的字母数字组合。
- 平台必须设置查询频率限制；若日志敏感，还应增加登录、验证码或有效期控制。
- 采集端必须持久化查询码，并在任务详情和上传历史中提供复制按钮。

## 11. 上传状态

推荐任务状态：

| 状态 | 说明 |
| --- | --- |
| `local` | 仅本地采集，未开启云端上传 |
| `checking` | 正在检查平台连通性 |
| `ready` | 连通性通过，等待创建任务 |
| `uploading` | 正在采集或上传 |
| `paused` | 上传暂停，待传数据仍保留在本地 |
| `finalizing` | 正在封口文件并完成平台任务 |
| `analyzing` | 平台正在分析 |
| `completed` | 平台任务完成 |
| `uncertain` | 请求可能已送达，但确认响应丢失，需要核对 |
| `failed` | 上传失败，可按错误类型处理 |
| `cancelled` | 用户明确取消任务 |

关闭配置弹窗或收起界面不得改变任务状态。断开串口只停止产生新日志，默认继续上传已保存的数据。暂停后继续上传不得重复发送平台已确认接收的文件。

## 12. 明确不提供的字段

| 字段 | 原因 |
| --- | --- |
| `device_id` / `device_sn` | 当前业务不需要跨电脑追踪同一记录仪 |
| `port_name` | 仅用于本地连接和诊断，平台不依赖串口号 |
| 本地文件绝对路径 | 平台不可使用，且可能泄露本地信息 |
| 关键字方案和规则 | 仅用于采集端本地匹配 |
| 查询码输入框 | 查询码只能由平台返回，用户不可编辑 |
| 明文鉴权 Token | Token 只通过请求头传递，不进入任务字段和日志 |

## 13. 当前 V1 接口兼容说明

当前采集端使用 `POST /api/logs/upload`，Multipart 只提交：

- `project_name`
- `version`
- 一个或多个 `file`

当前成功判定要求 HTTP `202`、业务 `code=0`、非空 `upload_id`、非空 `task_id`，且返回 `file_count` 与实际文件数一致。

本文档中的任务、来源、命中证据、上传人、幂等 ID 和查询码生命周期属于目标 V2 协议。平台端完成 V2 接口前，采集端不得假定这些字段已经被平台接收。

## 14. 最小实现范围

首期建议只实现：

1. 用户填写项目、版本和上传人，测试任务和备注选填。
2. 一个上传任务支持多个通道并共用一个查询码。
3. 本地保存关键字命中行及前后各 10 行。
4. 上传任务创建、分片幂等上传、任务完成和查询接口。
5. 暂停后使用原任务和原查询码续传。
6. 本地保存查询码并提供复制入口。

设备身份、关键字配置同步、跨电脑任务管理和远程串口控制不在首期范围内。
