# 接口对接说明

## 当前上传接口

客户端使用 `POST /api/logs/upload`，提交项目、版本、任务快照和日志文件。后端地址由 `backend.base_url` 与 `backend.upload_path` 组合。请求超时、上传周期和预检开关由配置控制。

请求使用 `multipart/form-data`，重复的文件字段名为 `file`。文本字段包括：

- 必填：`project_name`、`version`、`uploader_name`。
- 选填：`project_id`、`test_task_id`、`test_task_name`、`remark`、`collector_version`、`timezone`、`created_at`、`started_at`、`ended_at`、`scenario_ids`。
- `scenario_ids` 是 JSON 数组字符串，最多 20 个元素。
- `client_request_id` 可同时作为表单字段发送；请求头 `Idempotency-Key` 优先，长度最多 128 字符。
- `project_id` 当前只发送纯数字平台 ID；本地目录中的 `project-a` 等 ID 不得冒充平台项目 ID。
- `uploader_id` 不由客户端发送，由服务端根据当前登录身份保存。

若配置 `authorization_token_env`，客户端从该环境变量读取 Token，并发送 `Authorization: Bearer <token>`；Token 不写入 YAML。启用 `upload_gzip` 时整个 multipart 请求体使用 GZIP，并发送 `Content-Encoding: gzip`。

成功响应必须同时满足 HTTP `202`、JSON `code=0`、非空 `data.upload_id`、非空 `data.task_id`，并且 `data.file_count` 与本批文件数相同。若响应包含 `data.client_request_id`，它必须与本地请求 ID一致。当前契约没有返回 `query_code`，客户端不得自行生成查询码。

后端必须明确区分已接收和失败。客户端只有在收到契约要求的成功响应后才能标记 `uploaded`；请求体尚未发出时可自动重试；请求可能已发出但响应未知时必须标记 `uncertain`。同一个 `Idempotency-Key` 在同一登录用户下必须返回首次创建的上传任务，不得重复保存文件。

## 文件元数据

每个封口文件应保留设备编号、序列范围、大小、SHA-256、项目、版本和本地路径。后端核对 `uncertain` 时至少使用 SHA-256、大小、设备编号和时间范围，不能只依据文件名。

## 本地桌面绑定

Wails 桌面层面向 UI 提供端口扫描、设备连接/断开、配置更新、任务开始/停止、指令发送、日志事件、设备状态和上传队列状态。绑定方法是本地进程边界，不应暴露为未认证的网络管理接口。UI 关闭或卡顿不得阻塞采集、落盘或上传 Worker。

## 平台扩展

心跳、远程设备状态、标签、异常同步和服务端幂等键通过平台适配层扩展，不应改变 V1 文件优先落盘和 SQLite 队列语义。启用自动重试 `uncertain` 前，后端必须先提供经过验证的幂等机制。

## 联调检查表

- 后端 URL、HTTPS 证书、代理超时和上传大小限制正确。
- multipart 字段、项目名、版本、文件名和响应状态与 `API接口文档.md` 一致。
- 断网时本地继续落盘，恢复后 `pending` 自动上传。
- 丢失响应时进入 `uncertain`，不会无条件自动重传。
- 四路文件、队列项和设备元数据互不混淆。
