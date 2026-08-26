# LogMaster 后端与前端 API 文档

> **2026/08/26 更新**：补充采集端配置同步接口契约说明，前端无需代理或修改。

> **825 调整（2026-08-25）**：登录页新增“企业员工”和“外包账号”两个入口。企业员工继续跳转 `/api/auth/feishu-login`；外包账号调用 `POST /api/auth/external/login` 或 `POST /api/auth/external/register`，注册字段为 `name`、`email`、`company`、`password`、`confirm_password`。注册和登录成功后后端写入同一个 HttpOnly `session_token` Cookie，并跳转 `/upload`。后端需执行迁移 `045_external_password_accounts.sql` 与 `047_external_role_source.sql`；若注册收到 500，请先确认后端已使用包含该迁移的新版本重启。

> **825 AI 配置调整**：管理后台不得编辑或提交 `llm_api_base_url`、`llm_api_key`、`clear_llm_api_key`、`llm_model`。页面将前三项仅作为环境配置的只读展示；尝试修改时后端返回 `400`。可提交的 AI 设置仅为超时、命中数量、输入字节上限、单文件 token 上限和每日 token 配额。

日志搜索接口会缓存相同上传、文件版本、关键词和大小写条件的命中结果 15 分钟，因此分页或多个用户重复查询不会重复扫描原始文件。缓存是服务端内存缓存，重启后清空，不改变接口返回结构。

外包注册不做邮箱验证码；密码只要求非空并与确认密码一致。`GET /api/user/info` 新增只读字段 `identity_type`（`feishu` 或 `external`）和 `company`。外包用户固定为普通用户，前端不能根据职位字段赋予管理入口。

外包账号登录后可在“账号设置”中调用 `POST /api/auth/external/change-password` 修改密码，或调用 `POST /api/auth/external/change-email` 修改邮箱；两个接口都要求当前密码。飞书账号不显示该入口。

外包登录页可提供“忘记密码”入口，调用 `POST /api/auth/external/password-reset`，请求字段为 `name`、`email`、`password`、`confirm_password`。成功后跳转外包账号登录页；失败统一展示“姓名、邮箱或账号信息不匹配”。飞书登录不显示忘记密码入口。

网页服务端上传可在 multipart 表单提交 `ai_analysis_enabled=true|false` 控制该上传是否创建 AI 作业；未提交默认为开启。采集端上传会话不受前端开关影响，后端固定关闭 AI。

外包账号不具备飞书消息权限。前端不要承诺外包用户会收到飞书登录或 AI 完成通知；其任务状态应通过现有任务列表、详情或站内通知查询。

> **824 调整（2026-08-24）**：采集端上传人经飞书校验后，后端同步 `users.job_title`；管理员用户接口已有 `job_title` 字段，因此前端无需新增接口。

飞书自动角色规则调整为：职位含 `主任` 为 `super_admin`，含 `高级` 为 `admin`，含 `软件工程师` 或 `硬件工程师` 为 `developer`，其他为 `user`。手工分配角色保持优先；前端继续读取现有 `role` 控制权限。

超管配置支持 `FEISHU_SUPER_ADMIN_OPEN_IDS` 和逗号分隔的 `FEISHU_SUPER_ADMIN_NAMES`。登录时按配置名单强制授予超管；两者都为空且系统尚无超管时，首个成功飞书登录用户初始化为超管。前端无需新增接口。

> **823 调整（2026-08-23）**：通知中心增加全部已读、用户开关和 SSE 实时推送；任务、AI、负责人分配和备注事件均可产生持久化通知。日志下载扩展为单文件、解析批次、原始上传包和分析结果包四类。
AI 作业现已使用 PostgreSQL 持久化队列。任务列表和任务详情新增 `ai_status`、`ai_error_message`；AI 状态为 `disabled`、`queued`、`running`、`completed`、`partial_failed`、`failed`、`cancelled`。`GET /api/tasks` 可传 `ai_status` 进行服务端筛选。规则解析状态与 AI 状态互不覆盖。
AI 支持整任务重试 `POST /api/tasks/{task_id}/agent-retry`、单文件重试 `POST /api/tasks/{task_id}/agent-retry/{file_id}` 和取消 `POST /api/tasks/{task_id}/agent-cancel`。单文件重试保留其他文件结果并在完成后刷新任务总览；取消仅停止 AI，不影响规则结果和原始日志。
`GET /api/tasks/{task_id}/agent-results` 的失败记录新增 `error_code`，可选值为 `authentication`、`rate_limit`、`quota`、`timeout`、`invalid_response`、`upstream`、`cancelled`、`unknown`；界面展示使用中文 `error_message`，业务分支使用稳定的 `error_code`。
任务暂停和恢复分别调用 `POST /api/tasks/{task_id}/pause`、`POST /api/tasks/{task_id}/resume`；任务列表需要识别 `paused` 状态。
任务优先级使用 `PUT /api/tasks/{task_id}/priority`，项目优先级使用 `PUT /api/admin/projects/{id}/priority`。任务详情返回 `priority`，管理员项目列表返回 `scheduling_priority`。
批量任务操作调用 `POST /api/tasks/batch`，请求为 `{ "action": "retry|cancel|delete", "task_ids": [] }`；批量负责人调用 `PUT /api/results/batch/assignment`。
任务因文件数或总字节数超过限制时状态为 `failed`，前端直接展示中文 `error_message`；因用户或项目并发达到上限时保持 `queued`。
异常操作历史通过 `GET /api/results/{id}/history` 查询，`action` 为 `status_changed`、`assignment_changed` 或 `comment_added`。
任务导出调用 `GET /api/tasks/{task_id}/export`，`format` 可选 `csv`、`json`、`report`；JSON 数据包包含 AI 结果。

> **822 调整（2026-08-22）**：采集端上传的会话在邮箱通过飞书校验后会自动关联到对应用户；该用户登录后可通过现有日志和任务接口查看记录，前端无需新增接口或字段。

失败或缺失 AI 结果时，前端可调用 `POST /api/tasks/{task_id}/agent-retry`。该接口只重跑 AI，不会重新解析规则；成功返回 `202`，任务尚未完成规则解析返回 `409`。

本文档记录 `server/frontend` 使用的 Web API，更新日期为 2026-08-23。后端全部接口见 [`backend-api.md`](backend-api.md)。

> **最新：821 调整（2026-08-21）**：后端新增持久化解析任务队列和受控 Worker，上传接口仍返回 `202 + status=queued`，前端请求方式不变；同时为新完成的 AI 任务生成任务级总览，并兼容地作为 `GET /api/tasks/{task_id}/agent-results` 数组第一条返回。本次没有新增前端必须调用的接口。

> **最新：818 调整（2026-08-18）**：采集端连续上传会话新增飞书通讯录 `uploader_email` 校验；前端普通文件上传保持 `uploader_name` 契约不变。

> **818 补充**：修复 AI 分析用量记录的数据库权限问题，前端接口和返回结构不变。

## 1. 前端 API 清单

### 1.0 任务调度兼容说明

新增取消接口：`POST /api/tasks/{task_id}/cancel`。任务处于 `queued` 或解析中时可取消，成功返回 `202` 和 `status=cancelled`；已完成或失败任务返回 `409`，重复取消已取消任务幂等返回 `202`。取消不会删除原始日志或已有解析结果。
新增原始日志搜索：`GET /api/logs/{upload_id}/search`，前端可传 `file_id`、`keyword`、`case_sensitive`、`page`、`page_size`，后端返回分页命中行。
日志下载统一使用 `GET /api/logs/{upload_id}/download`：`type=file&file_id=...` 下载单文件，`type=batch` 下载解析日志 ZIP，`type=original` 下载最初上传文件 ZIP，`type=results` 下载分析结果 ZIP。单文件支持 `Range`；不传 `type` 兼容原调用方式。结果包包含 `results.csv`、`data.json`、`report.md`。
上传、解压或解析失败时，前端应展示响应中的中文 `message`；任务详情可读取中文 `error_message`。管理员密码维护接口为 `GET/POST /api/admin/archive-passwords` 和 `DELETE /api/admin/archive-passwords/{id}`。解压密码不在解析规则列表中：未加密压缩包直接解压；加密压缩包仅尝试管理员维护的密码。
异常结果状态通过 `PATCH /api/results/{id}/status` 更新，状态值为 `pending`、`confirmed`、`false_positive`、`fixed`、`closed`。
异常结果备注通过 `POST /api/results/{id}/comments` 添加，通过 `GET /api/results/{id}/comments` 查询；备注内容必填，缺陷单号可选。
异常结果负责人通过 `PUT /api/results/{id}/assignment` 设置，传入用户 `open_id`；传空字符串取消分配。
版本对比通过 `GET /api/analysis/compare` 调用，必须传 `baseline_task_id` 和 `current_task_id`；同一异常按文件路径、规则名称和命中文本判定。
任务列表可将状态、项目、版本、页码和排序条件直接传给 `GET /api/tasks`，后端返回 `total`、`page`、`page_size` 和当前页列表。

解析任务和 AI 作业现在都由服务端持久化队列执行。前端通过现有 `GET /api/tasks`、`GET /api/tasks/{task_id}` 分别读取规则解析 `status` 和独立的 `ai_status`。失败解析任务可调用 `POST /api/tasks/{task_id}/retry`；解析任务取消调用 `POST /api/tasks/{task_id}/cancel`，AI 单独取消调用 `POST /api/tasks/{task_id}/agent-cancel`。

通知列表使用 `GET /api/notifications?page=1&page_size=20&unread_only=false`；单条已读使用 `PATCH /api/notifications/{id}/read`，全部已读使用 `POST /api/notifications/read-all`。用户通知开关通过 `GET/PUT /api/notification-settings` 维护，PUT 必须提交 `task_completed`、`task_failed`、`task_cancelled`、`ai_completed`、`ai_failed`、`result_assigned`、`result_commented` 七个布尔字段。实时通知连接 `GET /api/notifications/stream`，事件名为 `notification`，断线重连可传 `Last-Event-ID`；通知对象可包含 `task_id`、`upload_id`、`result_id`。

> **8/16新增**：`GET` / `PUT` `/api/admin/ai-analysis-settings`
> **8/16之前**：其余全部接口

### 1.1 健康检查与认证

| 方法 | 路径 | 访问要求 | 前端用途 | 日期 |
| --- | --- | --- | --- | --- |
| `GET` | `/api/health` | 公开 | 检查后端连接状态 | 8/16之前 |
| `GET` | `/api/auth/feishu-url` | 公开 | 获取飞书登录 URL | 8/16之前 |
| `GET` | `/api/auth/feishu-login` | 公开 | 浏览器跳转飞书登录 | 8/16之前 |
| `GET` | `/api/auth/callback` | OAuth 回调 | 完成登录并返回首页 | 8/16之前 |
| `POST` | `/api/auth/logout` | 公开 | 退出登录 | 8/16之前 |
| `GET` | `/api/auth/me` | 飞书 Session | 当前用户兼容接口 | 8/16之前 |
| `GET` | `/api/user/info` | 飞书 Session | 获取当前用户和角色 | 8/16之前 |

### 1.2 日志、查询与任务

| 方法 | 路径 | 访问要求 | 前端用途 | 日期 |
| --- | --- | --- | --- | --- |
| `GET` | `/api/upload-config` | 飞书 Session | 获取上传容量 | 8/16之前 |
| `POST` | `/api/logs/inspect` | 飞书 Session | 上传前检查文件 | 8/16之前 |
| `POST` | `/api/logs/upload` | 飞书 Session | 网页上传日志 | 8/16之前 |
| `GET` | `/api/query/{query_code}` | 公开 | 查询采集端上传进度 | 8/16之前 |
| `POST` | `/api/query/{query_code}/collect` | 飞书 Session | 收藏采集会话 | 8/16之前 |
| `GET` | `/api/logs` | 飞书 Session | 上传记录列表 | 8/16之前 |
| `GET` | `/api/logs/{upload_id}` | 飞书 Session | 上传详情 | 8/16之前 |
| `GET` | `/api/logs/{upload_id}/preview` | 飞书 Session | 日志文本预览 | 8/16之前 |
| `GET` | `/api/logs/{upload_id}/download` | 飞书 Session | 下载单文件、解析批次、原始上传包或结果包 | 8/23调整 |
| `GET` | `/api/tasks` | 飞书 Session | 解析任务列表 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}` | 飞书 Session | 任务详情 | 8/16之前 |
| `DELETE` | `/api/tasks/{task_id}` | 飞书 Session | 删除任务 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}/results` | 飞书 Session | 规则解析结果 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}/agent-results` | 飞书 Session | 文件级分析结果；新任务还会在第一条返回任务级 AI 总览 | 8/21调整 |
| `POST` | `/api/tasks/{task_id}/agent-retry` | 飞书 Session | 重试整个任务的 AI 分析 | 8/22调整 |
| `POST` | `/api/tasks/{task_id}/agent-retry/{file_id}` | 飞书 Session | 只重试指定文件并刷新任务总览 | 8/23调整 |
| `POST` | `/api/tasks/{task_id}/agent-cancel` | 飞书 Session | 取消该任务尚未完成的 AI 分析 | 8/23调整 |
| `GET` | `/api/dashboard/stats` | 飞书 Session | 仪表板统计 | 8/16之前 |
| `GET` | `/api/projects` | 飞书 Session | 上传项目选项 | 8/16之前 |
| `GET` | `/api/notifications` | 飞书 Session | 分页查询通知和未读数 | 8/23调整 |
| `PATCH` | `/api/notifications/{id}/read` | 飞书 Session | 单条通知已读 | 8/23调整 |
| `POST` | `/api/notifications/read-all` | 飞书 Session | 全部通知已读 | 8/23调整 |
| `GET` | `/api/notifications/stream` | 飞书 Session | SSE 实时通知 | 8/23调整 |
| `GET/PUT` | `/api/notification-settings` | 飞书 Session | 查询或保存当前用户通知开关 | 8/23调整 |

### 1.3 规则与测试场景

| 方法 | 路径 | 访问要求 | 前端用途 | 日期 |
| --- | --- | --- | --- | --- |
| `GET` | `/api/rules` | 飞书 Session | 查询规则 | 8/16之前 |
| `POST` | `/api/rules` | 飞书 Session | 当前禁止直接创建 | 8/16之前 |
| `PUT` | `/api/rules/batch` | 飞书 Session | 批量启停规则 | 8/16之前 |
| `PUT` | `/api/rules/{id}` | 飞书 Session | 更新规则 | 8/16之前 |
| `DELETE` | `/api/rules/{id}` | 飞书 Session | 删除规则 | 8/16之前 |
| `GET` | `/api/scenarios` | 飞书 Session | 查询测试场景 | 8/16之前 |
| `POST` | `/api/scenarios` | 飞书 Session | 创建测试场景 | 8/16之前 |
| `GET` | `/api/scenarios/{id}` | 飞书 Session | 查询场景详情 | 8/16之前 |
| `PUT` | `/api/scenarios/{id}` | 飞书 Session | 更新测试场景 | 8/16之前 |
| `DELETE` | `/api/scenarios/{id}` | 飞书 Session | 删除测试场景 | 8/16之前 |
| `PATCH` | `/api/scenarios/{id}/enabled` | 飞书 Session | 启停测试场景 | 8/16之前 |

### 1.4 管理与申请

| 方法 | 路径 | 访问要求 | 前端用途 | 日期 |
| --- | --- | --- | --- | --- |
| `GET` | `/api/admin/users` | `super_admin` | 用户列表 | 8/16之前 |
| `PUT` | `/api/admin/users/{id}/role` | `super_admin` | 修改用户角色 | 8/16之前 |
| `PUT` | `/api/admin/users/{id}/restore-feishu-role` | `super_admin` | 恢复自动角色 | 8/16之前 |
| `GET` | `/api/admin/permission-requests` | 飞书 Session | 查询权限申请 | 8/16之前 |
| `POST` | `/api/admin/permission-requests` | 非超级管理员 | 创建权限申请 | 8/16之前 |
| `DELETE` | `/api/admin/permission-requests/{id}` | 申请人 | 撤回权限申请 | 8/16之前 |
| `PUT` | `/api/admin/permission-requests/{id}/decision` | `super_admin` | 审批权限申请 | 8/16之前 |
| `GET` | `/api/admin/runtime-logs` | 飞书 Session | 运行日志 | 8/16之前 |
| `GET` | `/api/admin/project-requests` | 飞书 Session | 查询项目申请 | 8/16之前 |
| `POST` | `/api/admin/project-requests` | `user` 或 `developer` | 创建项目申请 | 8/16之前 |
| `PUT` | `/api/admin/project-requests/{id}/decision` | `admin` 或 `super_admin` | 审批项目申请 | 8/16之前 |
| `GET` | `/api/admin/upload-capacity` | `super_admin` | 容量设置 | 8/16之前 |
| `PUT` | `/api/admin/upload-capacity` | `super_admin` | 修改容量设置 | 8/16之前 |
| `GET` | `/api/admin/ai-analysis-settings` | `super_admin` | AI 分析限额设置 | 8/16新增 |
| `PUT` | `/api/admin/ai-analysis-settings` | `super_admin` | 修改 AI 分析限额 | 8/16新增 |
| `GET` | `/api/admin/keyword-rules` | 关键词权限 | 标准关键词列表 | 8/16之前 |
| `POST` | `/api/admin/keyword-rules` | 关键词权限 | 新增公共解析规则 | 8/26新增 |
| `POST` | `/api/admin/keyword-rules/import` | 关键词权限 | 导入标准关键词 | 8/16之前 |
| `PUT` | `/api/admin/keyword-rules/{id}` | 关键词权限 | 修改公共解析规则 | 8/26新增 |
| `DELETE` | `/api/admin/keyword-rules/{id}` | 关键词权限 | 删除标准关键词 | 8/16之前 |
| `GET` | `/api/admin/projects` | `admin` 或 `super_admin` | 项目管理列表 | 8/16之前 |
| `POST` | `/api/admin/projects` | `admin` 或 `super_admin` | 创建项目 | 8/16之前 |
| `PUT` | `/api/admin/projects/{id}` | `admin` 或 `super_admin` | 更新项目 | 8/16之前 |
| `DELETE` | `/api/admin/projects/{id}` | `admin` 或 `super_admin` | 停用项目 | 8/16之前 |
| `GET` | `/api/admin/project-options` | 飞书 Session | 项目属性选项 | 8/16之前 |
| `POST` | `/api/admin/project-options` | `admin` 或 `super_admin` | 创建项目属性选项 | 8/16之前 |
| `PUT` | `/api/admin/project-options/{id}` | `admin` 或 `super_admin` | 更新项目属性选项 | 8/16之前 |
| `DELETE` | `/api/admin/project-options/{id}` | `admin` 或 `super_admin` | 停用项目属性选项 | 8/16之前 |

## 2. 前端调用约定

### 2.1 Axios 基础地址

前端 API 模块中的路径不带 `/api`，必须通过以下构建变量补全：

```powershell
$env:VITE_API_BASE_URL="/api"
$env:VITE_FEISHU_LOGIN_URL="/api/auth/feishu-login"
```

例如 `service.post('/auth/logout')` 最终必须请求 `/api/auth/logout`。如果 `VITE_API_BASE_URL` 为空，请求会落到 SPA 路径并可能返回 `405`。

### 2.2 登录和响应

飞书登录状态存储在 HttpOnly Cookie `session_token` 中，同源请求会自动携带。统一响应：

```json
{"code":0,"message":"success","data":{}}
```

Axios 响应拦截器返回 `data` 字段，因此页面通常直接获得业务对象。HTTP `401` 表示需要重新登录，`403` 表示已登录但角色权限不足。

### 2.3 分页

`page` 默认 1；`page_size` 默认 20、最大 200。日志和任务列表返回 `{total, list}`。

## 3. 健康检查与认证接口说明

### 3.1 `GET /api/health`

页面用于检查后端连接。成功数据为 `{"status":"ok"}`。

### 3.2 `GET /api/auth/feishu-url`

返回 `{"url":"飞书授权地址"}`，同时设置 OAuth state Cookie。适合前端自行获取 URL 后跳转。

### 3.3 `GET /api/auth/feishu-login`

直接以 HTTP `302` 跳转到飞书授权页。当前路由守卫和登录入口优先使用此地址。

### 3.4 `GET /api/auth/callback`

飞书回调地址，参数为 `code` 和 `state`。成功后写入 `session_token` 并跳转 `/`，不是普通 Axios JSON 接口。

### 3.5 `POST /api/auth/logout`

清除服务端 Session 和 Cookie。成功数据为 `null`。正确地址必须包含 `/api`。

### 3.6 `GET /api/auth/me`

当前用户兼容接口，与 `/api/user/info` 返回相同数据。

### 3.7 `GET /api/user/info`

返回用户的 `feishu_open_id`、姓名、邮箱、头像、`role`、`role_source` 和 `job_title`。前端依据 `role` 控制管理页面入口。

## 4. 日志、查询与任务接口说明

### 4.1 `GET /api/upload-config`

返回 `max_upload_bytes` 和 `max_files_per_upload`，用于前端上传前校验。

### 4.2 `POST /api/logs/inspect`

使用 `multipart/form-data`，字段 `file` 只能有一个。返回 `archive` 和压缩包内可解析日志 `entries`，不创建任务。

### 4.3 `POST /api/logs/upload`

使用 `multipart/form-data`。采集会话创建接口要求 `uploader_email`，后端通过飞书通讯录解析并保存用户身份；普通前端文件上传仍可提交 `uploader_name`，不受采集端邮箱契约影响。

请求头 `Idempotency-Key` 优先于表单幂等键。成功为 HTTP `202`，数据包含 `upload_id`、`task_id`、`query_code`、`status`、`file_count` 和 `client_request_id`。前端收到 `202` 后继续轮询任务接口。

### 4.4 `GET /api/query/{query_code}`

公开查询接口。返回采集会话汇总、处理状态和批次数组。查询页对输入转大写，并只保留字母、数字和连字符。

### 4.5 `POST /api/query/{query_code}/collect`

将内置采集端上传会话关联到当前飞书用户。成功数据包含 `query_code`、`batch_count` 和 `source_type=collector`。

### 4.6 `GET /api/logs`

参数为 `page`、`page_size` 和可选 `source_type`。`source_type` 只能是 `collector` 或 `uploaded`。返回 `{total, list}`。

### 4.7 `GET /api/logs/{upload_id}`

返回 `{upload, files}`。后端按当前用户和已收藏采集会话进行数据隔离。

### 4.8 `GET /api/logs/{upload_id}/preview`

可选参数 `file_id`。返回 `file_id`、`relative_path`、`lines` 和 `truncated`；最多预览 500 行、2 MiB。

### 4.9 `GET /api/tasks`

使用分页参数，返回 `{total, list}`。

### 4.10 `GET /api/tasks/{task_id}`

返回 `{task, files, agent_enabled}`，供任务详情和分析结果页面轮询。

### 4.11 `DELETE /api/tasks/{task_id}`

删除任务、关联上传记录和存储文件。成功数据为 `null`。

### 4.12 `GET /api/tasks/{task_id}/results`

使用分页参数，返回规则命中结果数组，包括级别、行号、内容、规则、上下文和关联原因。

### 4.13 `GET /api/tasks/{task_id}/agent-results`

返回 AI/Agent 分析记录数组。未启用 AI/Agent 时为空数组。每条记录含 `provider`（`llm` 为直连大模型、`http-agent` 为外部 Agent）、`status`、`summary`、`findings` 和 `error_message`。

821 调整后，新完成的直连大模型任务会在数组第一条返回任务级聚合记录。该记录的 `file_path` 为 `任务级 AI 总览`、`log_file_id` 为 `0`，`summary` 是整个任务的结论，`findings` 是跨文件聚合后的风险和建议。其余数组项仍保持逐文件结构。

本次没有修改前端代码。当前解析结果页本来就遍历整个返回数组，因此任务级总览会作为第一条 AI 结果显示。前端不需要调用新接口，也不需要提交新字段。

`findings` 每项包含：`category`、`severity`、`root_cause`、`evidence`、`impact`（影响）、`suggestion`、`confidence`、`line_number`（关联命中的行号）和 `file_path`（关联文件）。

前端解析结果页可用 `summary` + `findings` 渲染「AI 总结」面板；用 `line_number`/`file_path` 与规则结果行匹配，在「看上下文」旁渲染单条「AI 解读」。

AI 分析是异步任务：关键字规则结果先返回，AI 结果可能延迟出现，页面应在解析完成后再拉取一次本接口（或轮询到有结果）。

### 4.14 `GET /api/dashboard/stats`

参数 `days` 支持 7 或 30。返回日志行数、错误/警告数、任务统计、趋势、热门命中和最近任务。

### 4.15 `GET /api/projects`

返回启用项目名称数组，供上传和场景页面选择。

## 5. 规则与场景接口说明

### 5.1 `GET /api/rules`

返回系统规则和当前用户可见规则，包含启用状态、是否可编辑和被场景引用次数。

### 5.2 `POST /api/rules`

当前固定返回 `403`。前端不应展示普通新增规则入口；标准规则通过管理端关键词导入。

### 5.3 `PUT /api/rules/batch`

请求 `{"ids":[1,2],"enabled":true}`，ID 数量为 1 到 500，返回更新数量。

### 5.4 `PUT /api/rules/{id}`

使用完整规则对象更新当前用户可编辑规则，路径 ID 优先。

### 5.5 `DELETE /api/rules/{id}`

删除可编辑规则。被测试场景引用时返回 `409`。

### 5.6 `GET /api/scenarios`

返回测试场景数组。

### 5.7 `POST /api/scenarios`

创建场景，`id` 和 `name` 必填。后端校验引用规则并保存快照。

### 5.8 `GET /api/scenarios/{id}`

返回单个场景，不存在时返回 `404`。

### 5.9 `PUT /api/scenarios/{id}`

更新场景，路径 ID 覆盖请求体 ID。

### 5.10 `DELETE /api/scenarios/{id}`

删除指定场景。

### 5.11 `PATCH /api/scenarios/{id}/enabled`

请求 `{"enabled":true}`，返回更新后的场景。

## 6. 用户、权限和运行日志接口说明

### 6.1 `GET /api/admin/users`

仅超级管理员。返回用户、角色、角色来源、职务和时间字段。

### 6.2 `PUT /api/admin/users/{id}/role`

仅超级管理员。请求 `{"role":"developer"}`。不能降级系统中最后一个超级管理员。

### 6.3 `PUT /api/admin/users/{id}/restore-feishu-role`

仅超级管理员。恢复飞书自动角色并重新计算当前角色。

### 6.4 `GET /api/admin/permission-requests`

超级管理员查看全部申请，其他角色只查看本人申请。返回 `{requests, current_role, can_apply, can_review}`。

### 6.5 `POST /api/admin/permission-requests`

请求 `{"requested_role":"developer","reason":"..."}`。原因必填、最多 1000 字；同一用户只能有一个待处理申请。成功为 HTTP `201`。

### 6.6 `DELETE /api/admin/permission-requests/{id}`

申请人撤回自己的待处理申请，已处理时返回 `409`。

### 6.7 `PUT /api/admin/permission-requests/{id}/decision`

仅超级管理员。请求 `{"action":"approve","comment":"..."}`，不能审批自己的申请。

### 6.8 `GET /api/admin/runtime-logs`

最多返回 500 条。普通用户和开发者查看本人记录，管理员和超级管理员查看全部记录。

## 7. 项目相关接口说明

### 7.1 `GET /api/admin/project-requests`

普通用户和开发者查看本人申请，管理员查看全部申请。返回 `{requests, can_review}`。

### 7.2 `POST /api/admin/project-requests`

普通用户和开发者提交项目名、产品线、类型、阶段、说明和申请原因。成功为 HTTP `201`。

### 7.3 `PUT /api/admin/project-requests/{id}/decision`

管理员或超级管理员请求 `{"action":"approve","comment":"..."}`。拒绝时必须填写意见。

### 7.4 `GET /api/admin/projects`

管理员或超级管理员获取完整项目对象列表。

### 7.5 `POST /api/admin/projects`

管理员或超级管理员创建项目。字段为 `name`、`product_line`、`product_type`、`stage`、`description`，成功为 HTTP `201`。

### 7.6 `PUT /api/admin/projects/{id}`

使用与创建项目相同的结构更新项目。

### 7.7 `DELETE /api/admin/projects/{id}`

软删除项目，不删除历史日志。

### 7.8 `GET /api/admin/project-options`

所有登录用户可用，返回 `{lines, types, stages}`。

### 7.9 `POST /api/admin/project-options`

管理员或超级管理员请求 `{"kind":"line","name":"车载线"}`。`kind` 为 `line`、`type` 或 `stage`。

### 7.10 `PUT /api/admin/project-options/{id}`

使用与创建选项相同的请求体更新选项。

### 7.11 `DELETE /api/admin/project-options/{id}`

软删除自定义选项。系统选项和正在使用的选项不能删除。

## 8. 容量和关键词接口说明

### 8.1 `GET /api/admin/upload-capacity`

仅超级管理员。返回当前容量、文件数限制和更新时间。

### 8.2 `PUT /api/admin/upload-capacity`

请求 `{"max_upload_bytes":2147483648,"max_files_per_upload":100}`。容量为 1 MiB 到 100 GiB，文件数为 1 到 500。

### 8.3 `GET /api/admin/keyword-rules`

要求关键词管理权限，返回管理员导入的标准关键词数组。

### 8.4 `POST /api/admin/keyword-rules/import`

上传不超过 2 MiB 的 TXT/CSV 文件。可附带 `category`、`level`、`scope` 和 `description` 默认值；成功为 HTTP `201`。

### 8.5 `DELETE /api/admin/keyword-rules/{id}`

删除管理员导入规则。规则被场景引用时返回 `409`。

### 8.6 管理员解析规则维护（2026/08/26）

管理员界面应使用管理员专用接口，不要调用普通 `/api/rules` 来创建公共规则：

- `GET /api/admin/keyword-rules`：读取所有公共规则，包括 `system` 和关键字文档导入规则；响应包含 `enabled`、`source`、`editable=true`、`scenario_count`、`created_at`、`updated_at`。
- `POST /api/admin/keyword-rules`：具有关键词权限的 `developer`、`admin`、`super_admin` 可新增。
- `PUT /api/admin/keyword-rules/{id}`：具有关键词权限的 `developer`、`admin`、`super_admin` 可编辑公共规则。
- `DELETE /api/admin/keyword-rules/{id}`：具有关键词权限的 `developer`、`admin`、`super_admin` 可删除；`scenario_count > 0` 时应禁用删除入口或展示“已被测试场景引用”。

写入字段为 `name`、`category`、`keyword`、`scope`、`level`、`enabled`、`description`。普通用户不能进入或提交该模块，后端返回 `403`。

### 8.6 AI 分析限额设置

`GET /api/admin/ai-analysis-settings` 返回 `{max_tokens_per_file, daily_token_quota}`；`PUT` 使用相同结构更新。仅超级管理员可访问。

- `max_tokens_per_file`：单个文件允许的 AI 模型最大输出 token 数（1 到 1000000）。
- `daily_token_quota`：每个用户每天的 AI token 配额，`0` 表示不限制。

管理页面在「AI 分析」相关设置里提供这两个输入项，保存后调用 `PUT`；加载时调用 `GET`。

## 9. 前端联调检查

- 所有 Axios 请求必须通过 `/api` 基础路径。
- `401` 才跳转登录；管理接口的 `403` 应显示权限不足，不应循环跳转。
- 上传返回 `202` 只表示已接收，必须继续轮询任务。
- 查询码查看接口公开，收藏接口要求登录。
- 页面不能通过传入用户 ID 绕过后端的数据隔离。
## Collector synchronization contract

Collector synchronization is handled by backend upload-token endpoints: `/api/projects/sync`, `/api/scenarios/sync`, and `/api/collector/sync`. The frontend does not need to proxy these endpoints or alter their payloads. The combined response contains projects, published scenarios, standard keywords, and a UTC `synced_at` timestamp.
### Administrator role restoration (2026/08/26)

When the administrator uses `PUT /api/admin/users/{user_id}/restore-feishu-role`, the backend restores external accounts to ordinary `user` and Feishu accounts according to their current Feishu job title. The frontend should display the returned `identity_type` and `role_source` so the result is clear.
