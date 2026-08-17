# LogMaster 后端与前端 API 文档

本文档记录 `server/frontend` 使用的 Web API，更新日期为 2026-08-15。后端全部接口见 [`backend-api.md`](backend-api.md)。

## 1. 前端 API 清单

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
| `GET` | `/api/tasks` | 飞书 Session | 解析任务列表 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}` | 飞书 Session | 任务详情 | 8/16之前 |
| `DELETE` | `/api/tasks/{task_id}` | 飞书 Session | 删除任务 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}/results` | 飞书 Session | 规则解析结果 | 8/16之前 |
| `GET` | `/api/tasks/{task_id}/agent-results` | 飞书 Session | Agent 分析结果 | 8/16之前 |
| `GET` | `/api/dashboard/stats` | 飞书 Session | 仪表板统计 | 8/16之前 |
| `GET` | `/api/projects` | 飞书 Session | 上传项目选项 | 8/16之前 |

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
| `POST` | `/api/admin/keyword-rules/import` | 关键词权限 | 导入标准关键词 | 8/16之前 |
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

使用 `multipart/form-data`。必填字段为一个或多个 `file`、`project_name`、`version`、`uploader_name`；常用可选字段为 `project_id`、任务信息、`remark`、`scenario_ids` 和 `client_request_id`。

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

### 8.6 AI 分析限额设置

`GET /api/admin/ai-analysis-settings` 返回 `{max_files_per_task, daily_token_quota}`；`PUT` 使用相同结构更新。仅超级管理员可访问。

- `max_files_per_task`：每个上传任务最多对多少个文件做 AI 分析（1 到 500）。
- `daily_token_quota`：每个用户每天的 AI token 配额，`0` 表示不限制。

管理页面在「AI 分析」相关设置里提供这两个输入项，保存后调用 `PUT`；加载时调用 `GET`。

## 9. 前端联调检查

- 所有 Axios 请求必须通过 `/api` 基础路径。
- `401` 才跳转登录；管理接口的 `403` 应显示权限不足，不应循环跳转。
- 上传返回 `202` 只表示已接收，必须继续轮询任务。
- 查询码查看接口公开，收藏接口要求登录。
- 页面不能通过传入用户 ID 绕过后端的数据隔离。
