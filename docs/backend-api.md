# LogMaster 后端 API 文档

> **2026/08/26 更新**：新增采集端项目、已发布测试场景及统一配置快照同步接口。

> **825 调整（2026-08-25）**：新增外包账号密码注册与登录。外包账号固定为 `role=user`、`role_source=external`、`identity_type=external`，不使用飞书职位自动授权规则。迁移 `045_external_password_accounts.sql` 新增凭证表，`047_external_role_source.sql` 将用户角色来源约束扩展为 `external`，两者均需在后端重启时自动执行；密码只以 Argon2id 哈希保存；本期不包含邮箱验证、邀请码、账号禁用或有效期。

`POST /api/auth/external/register` 公开注册，JSON 为 `{ "name": "...", "email": "...", "company": "...", "password": "非空字符串", "confirm_password": "..." }`。邮箱全局不可重复，成功返回 `201` 并写入登录 Cookie。

`POST /api/auth/external/login` 公开登录，JSON 为 `{ "email": "...", "password": "..." }`。成功返回当前用户并写入登录 Cookie；错误邮箱或密码返回 `401`。`GET /api/auth/me` 和 `GET /api/user/info` 同时支持飞书和外包 Session。

`POST /api/auth/external/change-password` 要求外包用户登录，JSON 为 `{ "current_password": "...", "password": "非空字符串", "confirm_password": "..." }`；当前密码正确且新密码非空、两次输入一致时修改成功。

`POST /api/auth/external/change-email` 要求外包用户登录，JSON 为 `{ "current_password": "...", "email": "..." }`；邮箱全局唯一，修改成功后刷新当前登录 Cookie。飞书账号不能通过这两个接口修改凭证或邮箱。

`POST /api/auth/external/password-reset` 为外包账号忘记密码接口，不要求登录，JSON 为 `{ "name": "...", "email": "...", "password": "非空字符串", "confirm_password": "..." }`。后端仅在外包账号的姓名和邮箱同时精确匹配时更新密码；不匹配统一返回 `401`，不透露邮箱是否已注册。成功后作废该账号现有登录会话，需使用新密码重新登录；飞书账号不支持该接口。

采集端上传规则：采集端同步的测试任务必须对应服务端 `test_scenarios`。上传带 `test_task_id` 时按场景 ID 精确匹配；仅带 `test_task_name` 时按唯一场景名称匹配；带有测试任务但无法匹配时直接拒绝上传，不回退到全量规则。未带测试任务时使用全部已启用的服务端关键字规则，但不使用 `FATAL/ERROR/WARNING/WARN` 通用兜底规则。

任务调度环境变量：`MAX_PARSE_WORKERS` 控制规则解析并行 Worker 数；`MAX_AI_WORKERS` 控制 AI 并行 Worker 数；`MAX_PARSE_ATTEMPTS` 控制解析任务自动恢复次数；`MAX_PARSE_PER_USER` 和 `MAX_PARSE_PER_PROJECT` 控制单用户、单项目并发上限；`MAX_FILES_PER_PARSE_TASK` 和 `MAX_BYTES_PER_PARSE_TASK` 限制单个任务解压后的文件数和总字节数。超过并发上限的任务排队，超过文件或字节容量的任务失败。

飞书通知只发送给 `identity_type=feishu` 的企业员工。外包账号不会收到飞书登录成功或 AI 分析完成/失败消息，也不会被加入飞书消息通知收件人。

AI 开关：采集端 `POST /api/upload-sessions` 创建的会话固定关闭 AI，后续该会话上传的文件不会创建 AI 作业。网页服务端上传接口 `POST /api/uploads` 可通过表单字段 `ai_analysis_enabled=true|false` 控制，未提交时默认为 `true`。迁移 `049_upload_ai_analysis_switch.sql` 增加上传会话和上传记录的持久化开关。

> **最新：824 调整（2026-08-24）**：`POST /api/upload-sessions` 成功响应新增 `data.uploader_job_title`。后端使用飞书 `tenant_access_token` 查询用户详情并同步 `users.job_title`；该字段为后端解析的只读展示字段，采集端请求不需要新增字段。

飞书自动角色固定按职位匹配：`主任` → `super_admin`，`高级` → `admin`，`软件工程师` / `硬件工程师` → `developer`，其他 → `user`。人工设置的角色不被自动同步覆盖。

超级管理员初始化规则：`FEISHU_SUPER_ADMIN_OPEN_IDS` 支持用英文逗号分隔多个飞书 Open ID，匹配后每次飞书登录都会强制授予 `super_admin`；`FEISHU_SUPER_ADMIN_NAMES` 支持用英文逗号分隔多个姓名，兼容但存在同名误匹配风险，优先使用 Open ID。两个配置都为空时，数据库还没有超级管理员的情况下，首次成功飞书登录的用户会成为超级管理员。权限检查只读查询角色，不再执行角色更新。

HTTP 服务默认配置为请求头读取 10 秒、请求读取 1800 秒、响应写入 600 秒、空闲连接 120 秒、优雅停机等待 30 秒，可通过同名 `HTTP_*_SECONDS` 环境变量调整。服务统一捕获 HTTP handler panic 并返回 500；收到 SIGINT/SIGTERM 后停止接收新请求并等待现有请求结束。

迁移 `044_fixed_feishu_job_title_roles.sql` 会重算现有飞书自动角色用户；重新启动后端后自动执行。

> **最新：823 调整（2026-08-23）**：完善持久化通知中心和下载接口。通知覆盖任务、AI、负责人分配和备注事件，支持列表、单条/全部已读、用户开关和 SSE；下载支持单文件、解析批次、原始上传包和分析结果包。
新增迁移 `042_persistent_ai_queue.sql`，AI 文件分析和任务总览使用 PostgreSQL `ai_jobs` 持久化队列、租约、心跳和最多 3 次异常恢复。`MAX_AI_WORKERS` 控制单实例 AI Worker 数，默认 1。任务返回独立的 `ai_status`、`ai_error_message`，`GET /api/tasks` 支持 `ai_status` 筛选。
新增 `POST /api/tasks/{task_id}/agent-retry/{file_id}` 单独重试一个文件的 AI 分析，新增 `POST /api/tasks/{task_id}/agent-cancel` 取消未完成的 AI 作业。AI 失败结果新增结构化 `error_code`，迁移为 `041_ai_job_controls.sql`。
`POST /api/tasks/{task_id}/pause` 暂停排队或运行中的任务；`POST /api/tasks/{task_id}/resume` 将暂停任务恢复为 `queued`。暂停状态为 `paused`，暂停任务仍可取消。
`PUT /api/tasks/{task_id}/priority` 设置排队或暂停任务优先级；`PUT /api/admin/projects/{id}/priority` 由管理员设置项目调度优先级。Worker 按两者之和调度。
`POST /api/tasks/batch` 批量执行 `retry`、`cancel`、`delete`；`PUT /api/results/batch/assignment` 批量设置负责人。接口返回逐项处理结果。
解析调度支持单用户、单项目并发限制以及单任务文件数、总字节数限制。并发超限保持排队，任务容量超限进入失败并返回中文原因。
`GET /api/results/{id}/history` 查询异常状态、负责人和备注操作历史，返回操作人、旧值、新值和时间。
`GET /api/tasks/{task_id}/export?format=csv|json|report` 下载规则结果、包含 AI 结论的 JSON 数据包或 Markdown 任务分析报告。

> **最新：822 调整（2026-08-22）**：采集端上传会话继续使用飞书 `tenant_access_token` 校验 `uploader_email`；校验通过后自动注册或更新本地用户，并将采集会话幂等授权给该用户。HTTP 接口和采集端字段不变。

`POST /api/tasks/{task_id}/agent-retry` 只重新执行 AI 分析，不重新解析日志。仅已完成规则解析的任务可调用，成功返回 `202`；解析未完成返回 `409`，AI 未配置返回 `503`。
`POST /api/tasks/{task_id}/cancel` 取消解析任务。`queued` 或 `running` 可取消并返回 `202`、`status=cancelled`；已完成或失败返回 `409`，重复取消已取消任务幂等返回 `202`。取消保留原始文件、解析结果和运行日志，不修改采集端上传协议。
`GET /api/logs/{upload_id}/search` 按关键字搜索原始日志，支持 `file_id`、`case_sensitive`、`page`、`page_size`，返回命中总数和行号、路径、内容。
`GET /api/logs/{upload_id}/download` 的 `type` 可选 `file`、`batch`、`original`、`results`；单文件需传 `file_id` 并支持 HTTP Range，结果包包含 CSV、JSON、Markdown。不传 `type` 时兼容旧行为。
上传、解压和解析失败统一返回中文 `message`，后台任务的 `error_message` 和运行日志也使用中文。管理员可通过 `GET/POST /api/admin/archive-passwords` 管理解压密码，通过 `DELETE /api/admin/archive-passwords/{id}` 删除；普通管理员使用现有关键字管理权限即可操作。解压密码不属于解析规则，后端不再内置默认密码：未加密 ZIP 直接解压；加密 ZIP 仅依次尝试管理员维护的密码，支持标准 ZipCrypto 与 AES。任务开始会记录加载到的密码数量但不记录密码内容；读取密码表失败时任务以“读取解压密码配置失败”终止，不再误报为密码错误。
`PATCH /api/results/{id}/status` 更新异常结果状态，允许值为 `pending`、`confirmed`、`false_positive`、`fixed`、`closed`，只允许结果有权访问者操作。
`POST /api/results/{id}/comments` 添加异常备注，JSON 为 `{ "comment": "...", "defect_id": "BUG-123" }`；`GET /api/results/{id}/comments` 查询备注历史。
`PUT /api/results/{id}/assignment` 分配或取消负责人，JSON 为 `{ "assigned_to": "用户 open_id" }`，空字符串表示取消分配。
`GET /api/analysis/compare?baseline_task_id=...&current_task_id=...` 对比两个任务的异常聚合结果，返回新增、解决、持续、增加和减少五类数据。
`GET /api/tasks` 支持服务端 `page`、`page_size`、`status`、`ai_status`、`project`、`version`、`sort` 筛选和排序，`sort` 可选 `updated_at`、`errors`、`oldest`。

通知接口为：`GET /api/notifications`、`PATCH /api/notifications/{id}/read`、`POST /api/notifications/read-all`、`GET /api/notifications/stream`、`GET/PUT /api/notification-settings`。SSE 事件名为 `notification`，支持 `Last-Event-ID` 或 `after_id`；设置 PUT 是完整替换，包含七个布尔开关。通知类型为 `task_completed`、`task_failed`、`task_cancelled`、`ai_completed`、`ai_failed`、`result_assigned`、`result_commented`。

本文档是 `server/backend` 当前全部 HTTP API 的统一说明，更新日期为 2026-08-23。前端专用说明见 [`api-to-frontend.md`](api-to-frontend.md)，采集端专用说明见 [`api-to-collector.md`](api-to-collector.md)。

> **最新：821 调整（2026-08-21）**：解析任务改为 PostgreSQL 持久化队列，由受控 Worker 领取；上传接口仍返回 `status=queued`，没有新增 HTTP API。直连大模型模式同时新增任务级 AI 总览，现有 `GET /api/tasks/{task_id}/agent-results` 将总览作为数组第一条返回。

> **最新：818 调整（2026-08-18）**：`POST /api/upload-sessions` 必须提交 `uploader_email`；后端通过飞书通讯录校验企业成员身份并同步用户资料，`uploader_name` 可选。

> **818 补充**：AI 分析完成后会记录 `ai_usage` token 用量；迁移 `027_ai_usage_permissions.sql` 补齐表和序列权限，后端重启后自动执行。该修复不改变 HTTP API 返回结构。

> **825 调整（2026-08-25）**：`LLM_API_BASE_URL`、`LLM_API_KEY` 和 `LLM_MODEL` 只能由服务端环境变量或 Docker `.env` 配置，管理后台仅可修改超时、命中采样、输入字节上限和 token 配额。迁移 `046_lock_ai_provider_settings.sql` 会清除历史数据库中的模型地址、密钥和模型名，数据库不再覆盖环境变量。

日志全文搜索会在后端进程内缓存 `upload_id`、文件版本、关键词和大小写模式相同的完整命中结果，默认缓存 15 分钟。翻页和其他用户的相同搜索直接复用缓存，不再重复读取原始日志；缓存仅用于加速搜索，服务重启后会清空，超出单条 4 MiB 或总计 32 MiB 时自动不缓存。

AI 分析命中日志采用按规则/关键词分组轮询采样，避免重复关键词占满输入；采样上限和输入字节保护由后端固定执行。

### 821 任务队列说明

`parse_tasks` 是服务端解析任务的持久化队列。上传完成后任务先处于 `queued`，Worker 领取后内部任务进入 `running`，完成后进入 `completed`，不可恢复的错误进入 `failed`。准备阶段对外上传状态仍为 `queued`，规则解析阶段为 `parsing`。任务准备/解压和规则解析由同一任务的不同阶段执行；数据库保存 Worker 租约和执行代次，租约过期后可自动接管未完成任务，AI 结果也只接受当前执行代次写入。

`POST /api/tasks/{task_id}/retry` 允许有权限的用户重新解析失败任务，返回 `202` 和 `status=queued`。同一次人工重试仍在排队时重复调用也返回 `202`，不会重复执行；初始排队、解析中或已完成状态返回 `409`。人工重试保留执行代次历史，并不受自动租约接管次数上限限制。`MAX_PARSE_WORKERS`（默认 `2`）控制单个后端实例的解析并发数，`MAX_PARSE_ATTEMPTS`（默认 `3`）控制租约过期后的最大自动接管次数。

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
| `GET` | `/api/logs/{upload_id}/download` | 飞书 Session | 下载单文件、解析批次、原始上传包或结果包 | 8/23调整 |
| `GET` | `/api/tasks` | 飞书 Session | 查询解析任务 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}` | 飞书 Session | 查询任务详情 | 8/16之前 |
| `DELETE` | `/api/tasks/{task_id}` | 飞书 Session | 删除任务和存储文件 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}/results` | 飞书 Session | 查询规则解析结果 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}/agent-results` | 飞书 Session | 查询任务级总览和逐文件 AI/Agent 分析结果 | 8/21调整 |
| `POST` | `/api/tasks/{task_id}/agent-retry` | 飞书 Session | 重试整个任务的 AI 分析 | 8/22调整 |
| `POST` | `/api/tasks/{task_id}/agent-retry/{file_id}` | 飞书 Session | 只重试指定文件并刷新任务总览 | 8/23调整 |
| `POST` | `/api/tasks/{task_id}/agent-cancel` | 飞书 Session | 取消该任务尚未完成的 AI 作业 | 8/23调整 |
| `GET` | `/api/dashboard/stats` | 飞书 Session | 查询仪表板统计 | 8/16之前 |
| `GET` | `/api/projects` | 飞书 Session | 查询可上传项目名称 | 8/16之前 |
| `GET` | `/api/notifications` | 飞书 Session | 分页查询通知和未读数 | 8/23调整 |
| `PATCH` | `/api/notifications/{id}/read` | 飞书 Session | 标记当前用户单条通知已读 | 8/23调整 |
| `POST` | `/api/notifications/read-all` | 飞书 Session | 标记当前用户全部通知已读 | 8/23调整 |
| `GET` | `/api/notifications/stream` | 飞书 Session | SSE 实时通知流 | 8/23调整 |
| `GET/PUT` | `/api/notification-settings` | 飞书 Session | 当前用户通知开关 | 8/23调整 |

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
| `POST` | `/api/admin/keyword-rules` | `developer`、`admin` 或 `super_admin` | 新增公共解析规则 | 8/26新增 |
| `POST` | `/api/admin/keyword-rules/import` | `developer`、`admin` 或 `super_admin` | 导入标准关键词规则 | 8/16之前 |
| `PUT` | `/api/admin/keyword-rules/{id}` | `developer`、`admin` 或 `super_admin` | 修改公共解析规则 | 8/26新增 |
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

使用通用分页参数，返回 `{total, page, page_size, list}`。支持 `status`、`ai_status`、`project`、`version`、`sort`；其中 `ai_status` 可选 `disabled`、`queued`、`running`、`completed`、`partial_failed`、`failed`、`cancelled`。

### 5.5 `GET /api/tasks/{task_id}`

返回任务、文件列表以及是否启用了外部 Agent。`task` 中的规则解析 `status` 与 `ai_status` 相互独立，AI 失败不会修改解析状态：

```json
{"code":0,"message":"success","data":{"task":{"status":"completed","ai_status":"partial_failed","ai_error_message":"部分文件 AI 分析失败"},"files":[],"agent_enabled":true}}
```

### 5.6 `DELETE /api/tasks/{task_id}`

删除当前用户有权访问的任务、数据库关联记录及其存储目录。成功时 `data` 为 `null`。

### 5.7 `GET /api/tasks/{task_id}/results`

使用通用分页参数，返回规则命中数组。结果包含级别、命中文本、行号、文件路径、规则、事件时间、上下文行和关联原因。

### 5.8 `GET /api/tasks/{task_id}/agent-results`

返回 AI/外部 Agent 对各日志文件的分析记录数组。每条记录包含 `id`、`task_id`、`log_file_id`、`file_path`、`provider`、`status`、`summary`、`findings`、`error_message`、`error_code`、`created_at`、`updated_at`。`error_code` 仅失败时返回，可选值为 `authentication`、`rate_limit`、`quota`、`timeout`、`invalid_response`、`upstream`、`cancelled`、`unknown`。

直连大模型模式下，新任务完成全部文件级分析后，后端会再生成一条任务级总览，并将它放在返回数组第一条。任务级记录的兼容字段如下：

| 字段 | 任务级总览值 |
| --- | --- |
| `log_file_id` | `0`，不对应单个文件 |
| `file_path` | `任务级 AI 总览` |
| `provider` | `llm` |
| `summary` | 跨文件总体结论 |
| `findings` | 去重后的风险、证据、影响、建议和汇总操作 |

任务级总览只输入已完成的文件级 `summary/findings`，不会重新读取原始日志。输入最多包含 50 个文件和 100 条诊断，受 `llm_max_input_bytes` 保护，输出上限为 4000 token，并写入 `ai_usage`。结果保存在 `task_ai_overviews` 表，由迁移 `029_task_ai_overviews.sql` 创建。

`provider` 取值：直连大模型为 `llm`，转调外部 Agent 为 `http-agent`；`status` 为 `completed` 或 `failed`。

#### 5.8.1 `POST /api/tasks/{task_id}/agent-retry`

重新执行整个任务的 AI 分析，不重复规则解析。接口清理当前 AI 结果、递增 AI 结果代次并将全部文件重新排队。成功返回 `202`；规则解析未完成返回 `409`；AI 未配置返回 `503`。

#### 5.8.2 `POST /api/tasks/{task_id}/agent-retry/{file_id}`

仅清理并重新分析指定文件，保留同一任务其他文件的 AI 结果。文件分析结束后自动重新生成任务级总览。成功返回 `202` 和 `{task_id,file_id,status:"queued"}`；文件不属于任务时返回 `404`；已有 AI 重试在执行时幂等返回 `202`。

#### 5.8.3 `POST /api/tasks/{task_id}/agent-cancel`

设置任务级 AI 取消标记。尚未执行的文件和总览作业会跳过，正在进行的模型请求由 Worker 最迟约 1 秒发现并取消。接口保留规则结果、原始日志及已经完成的 AI 结果，成功或重复取消均返回 `202`。再次发起整任务或单文件 AI 重试会清除取消标记。

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
| `llm_api_base_url` | 只读 | 服务端环境变量中的大模型 OpenAI 兼容接口地址 |
| `llm_api_key_configured` | 只读 | 环境变量中是否已配置 API Key |
| `llm_model` | 只读 | 服务端环境变量中的模型名称 |
| `llm_timeout_seconds` | 5 到 600 | 单次请求超时时间 |
| `llm_max_matches` | 1 到 5000 | 按规则/关键词轮询采样的命中条数上限 |
| `llm_max_input_bytes` | 1024 到 10485760 | 单次模型输入大小上限 |

`PUT` 尝试修改 `llm_api_base_url`、`llm_api_key`、`clear_llm_api_key` 或 `llm_model` 时后端返回 `400`，这些字段不会写入数据库。

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

### 9.7 管理员公共解析规则 CRUD（2026/08/26）

管理员规则界面使用 `/api/admin/keyword-rules` 管理全部公共解析规则，包括原有 `system` 规则和从关键字文档导入的 `admin_keyword_upload` 规则。普通 `/api/rules` 保持用户自定义规则设置入口，`POST /api/rules` 仍固定返回 `403`，不能用于维护公共规则。

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/admin/keyword-rules` | `developer`、`admin` 或 `super_admin` | 查询全部公共规则；所有返回项均为 `editable=true`。 |
| `POST` | `/api/admin/keyword-rules` | `developer`、`admin` 或 `super_admin` | 新增公共规则，来源固定为 `admin_keyword_upload`。 |
| `PUT` | `/api/admin/keyword-rules/{id}` | `developer`、`admin` 或 `super_admin` | 修改公共规则的名称、分类、关键字、范围、等级、启用状态和说明。 |
| `DELETE` | `/api/admin/keyword-rules/{id}` | `developer`、`admin` 或 `super_admin` | 删除未被测试场景引用的公共规则。 |

新增和修改请求体：

```json
{
  "name": "录像写盘失败",
  "category": "recording",
  "keyword": "XA_WRITE_FAIL",
  "scope": "全局",
  "level": "critical",
  "enabled": true,
  "description": "检测录像写盘失败"
}
```

`category` 仅允许 `power`、`storage`、`recording`、`system`、`connectivity`、`feature`、`tool`；`level` 仅允许 `critical`、`warning`、`info`。同一公共规则不能存在相同的 `keyword + category`，冲突返回 `409`。规则被测试场景引用时删除返回 `409`，必须先从场景中移除引用；修改不会删除场景引用，但会使场景后续解析使用修改后的规则内容。管理员修改公共规则后，会清除该规则的个人启用/停用覆盖，确保新公共配置立即生效。

兼容历史测试场景：后端仅展开 JSON 数组类型的 `checks`；空值、`null` 或旧对象格式按“未引用规则”处理，不会使规则列表或删除前引用检查返回 `500`。引用检查以字符串规则 ID 绑定到 SQL 文本参数，兼容 pgx 驱动；若数据库访问本身失败，接口仅返回中文通用提示，具体 PostgreSQL 错误会记录在后端控制台日志 `check keyword rule usage for rule <id>` 中，不向浏览器暴露数据库细节。

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
## Collector configuration synchronization

`GET /api/projects/sync`, `GET /api/scenarios/sync`, and `GET /api/collector/sync` are upload-identity endpoints for collector configuration synchronization. The combined endpoint returns active projects, published scenarios, enabled standard keywords, and `synced_at`. The upload-session API remains the final validation point.
### Role restoration behavior (2026/08/26)

`PUT /api/admin/users/{user_id}/restore-feishu-role` now checks the account identity. External accounts are always restored to `role=user` with `role_source=external`; they are never assigned a Feishu job-title role. Feishu accounts are restored with `role_source=feishu` and their role is recalculated from the current Feishu job title. The operation is audited with the actual previous and new roles.
