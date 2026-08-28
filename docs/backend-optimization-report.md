# LogMaster 后端代码优化分析报告

> 分析范围：`server/backend/`（Go 后端，module `logmaster-agent`）
> 分析日期：2026-08-25
> 总体评价：**数据库层与队列设计质量较高**（参数化查询、行锁、SKIP LOCKED 任务认领、租约恢复机制、迁移事务化），但**HTTP 服务层、认证会话层和若干热点路径存在需要优先处理的问题**。

---

## 一、严重问题（建议立即修复）

### 🔴 P0-1 飞书 access_token 通过错误信息泄漏
- **位置**：`internal/auth/feishu.go:31,35,48` → `internal/auth/handlers.go:82-83`
- **问题**：错误信息中包含飞书接口原始响应体（`raw: %s`），该错误被 `http.Error` 直接返回给浏览器，同时写入 `runtime_logs` 表持久化。攻击面：令牌泄漏到前端 + 落库长期暴露。
- **修复**：剥离 raw 响应体，仅记录 HTTP 状态码与 `Msg`；对客户端只返回通用错误信息。

### 🔴 P0-2 日志搜索接口全文件扫描
- **位置**：`internal/logservice/handlers.go:869-985`（`logSearchHandler`）
- **问题**：每次搜索请求都从头到尾逐行读取所有日志文件（即使只要第 1 页也要数完所有匹配才能返回 `total`）。2GB 日志文件意味着每次搜索都是 O(文件大小) 的磁盘 IO + 字符串处理，多用户并发下会拖垮服务。
- **修复**（按成本递增）：
  1. 限制最大扫描行数/字节数，返回"结果可能不完整"标记；
  2. 搜索结果按 (upload, file, keyword) 做短 TTL 缓存；
  3. 解析时顺手建立行偏移索引，支持 grep 式快速定位。

---

## 二、高优先级问题

### 🟠 H-1 HTTP 服务无超时、无 recover、无优雅停机
- **位置**：`main.go:69-70`
- **问题**：`http.ListenAndServe(":8080", mux)` 裸奔——无 `ReadTimeout/WriteTimeout/IdleTimeout`（Slowloris 慢连接可耗尽连接），handler panic 会直接杀死进程（只有 worker 里有 recover），没有优雅停机（重启时正在解析的任务只能靠租约超时恢复）。
- **修复**：
  ```go
  server := &http.Server{
      Addr: ":8080", Handler: mux,
      ReadTimeout: 15*time.Second, WriteTimeout: 0 /* SSE需要长连接，单独处理或用更大值 */,
      IdleTimeout: 60*time.Second, MaxHeaderBytes: 1 << 20,
  }
  // + signal.NotifyContext + server.Shutdown + worker WaitGroup 等待
  ```

### 🟠 H-2 会话管理三连：无过期、无持久化、内存无限增长
- **位置**：`internal/auth/service.go:23,147-161`
- **问题**：`sessions map` 纯内存、无 TTL（cookie 7 天过期但服务端条目永不删除）、重启全员掉线、无并发上限。长期运行内存持续增长。
- **修复**：会话加 `expiresAt` 字段 + 惰性清理 + 定期清扫 goroutine；或迁到数据库表。

### 🟠 H-3 会话令牌可预测的降级路径 + Cookie 缺 Secure
- **位置**：`internal/auth/service.go:174-181`（`rand.Read` 失败时用纳秒时间戳当令牌）；`auth/service.go:153` 等所有 `SetCookie` 缺 `Secure: true`。
- **修复**：`rand.Read` 失败应直接让登录失败；Cookie 按 `r.TLS != nil` 或配置开关设置 `Secure`。

### 🟠 H-4 权限检查路径：每次 admin 请求触发一次 UPDATE 写库
- **位置**：`internal/admin/service.go:473-485`（`roleForUser` fallback），且 `main.go:42-47` **从未调用 `SetUserRoleResolver`**，导致该 fallback 分支必然执行。
- **问题**：`GET /api/admin/users` 这类只读请求也会执行 `UPDATE users SET role=...`，既是写放大又是竞态隐患；硬编码姓名 `'刘欣彤'` 匹配（`config.go:83` 默认值）意味着同名员工会被提权。
- **修复**：main.go 注入 roleResolver；删除姓名匹配 fallback；权限判定改为只读查询。

### 🟠 H-5 ClaimAgentJob 加载 10 万条结果后内存过滤
- **位置**：`internal/logservice/ai_queue.go:165-177`
- **问题**：认领 AI 任务后调用 `r.Results(ctx, taskID, ownerOpenID, 100000, 0)` 把整个任务的解析结果全部载入内存，再用 Go 循环按文件路径过滤。大任务（几十万条命中）会瞬间占用大量内存。
- **修复**：加 `WHERE` 条件（按 `log_file_id` 或 `file_path`）直接在 SQL 层过滤。

### 🟠 H-6 AI 设置在每个文件解析循环中重复查询
- **位置**：`internal/logservice/service.go:685`（`processUpload` 的 `for _, file := range files` 循环内调用 `s.analysisEnabled(ctx)`，内部每次查 `AIAnalysisSettings`）。
- **修复**：循环前查一次缓存到局部变量。1 万文件的任务可减少近 1 万次 DB 查询。

---

## 三、中优先级问题

### 🟡 M-1 内置上传令牌硬编码
- **位置**：`internal/logservice/service.go:41-42`，`builtinUploadToken = "logmaster-internal-collector-v1"` 写死在源码中。
- **问题**：源码一旦泄露，任何人都能以内部采集器身份上传。
- **修复**：改为部署时生成的随机密钥（环境变量注入），内置常量仅作为未配置时的降级并打告警日志。

### 🟡 M-2 关键词导入 N+1 查询
- **位置**：`internal/admin/settings.go:490-513`（`saveKeywordRules`）：最多 1000 条规则逐条 SELECT + INSERT/UPDATE，约 2000 次往返。
- **修复**：事务内一次 `SELECT ... WHERE keyword = ANY($1)` 批量取回，再用 `INSERT ... ON CONFLICT` 批量 UPSERT。

### 🟡 M-3 JSONB 规则占用检查无索引
- **位置**：`internal/admin/settings.go:277-281`：对 `test_scenarios.checks` 做全表 `jsonb_array_elements` 展开查询。
- **修复**：加 GIN 索引或维护 `scenario_rule` 关联表。

### 🟡 M-4 SSE 通知流轮询压力
- **位置**：`internal/logservice/handlers.go:790-853`：每个连接每 2 秒查一次 DB（还叠加 15 秒心跳）。
- **问题**：100 个在线用户 = 每秒 50 次 `NotificationsAfter` 查询。
- **修复**：改用 Postgres `LISTEN/NOTIFY` 或进程内事件总线广播；至少将轮询间隔加长并按 recipients 分组查询。

### 🟡 M-5 登录无防暴力破解、注册无密码强度
- **位置**：`internal/auth/external.go:74-97, 257-280`
- **修复**：登录接口加 IP/账号级限流（简单的内存令牌桶即可）；注册密码要求最小 8 位混合字符。

### 🟡 M-6 解压密码明文存储
- **位置**：`internal/admin/archive_passwords.go:52`（`archive_passwords.password` 为明文 TEXT 列）。
- **修复**：项目里已有现成的 `internal/securevalue`（AES-GCM）包但从未被使用——用它加密存储，读取时解密。顺带清理死代码问题。

### 🟡 M-7 任务导出无上限加载
- **位置**：`internal/logservice/handlers.go:1618`：`Results(ctx, taskID, ownerOpenID, 1000000, 0)` 一次性加载最多 100 万行进内存。
- **修复**：流式写出（`rows.Next()` 边读边写 CSV/JSON），或分批拉取。

### 🟡 M-8 CSV 公式注入
- **位置**：`internal/logservice/handlers.go:1631-1634`：`MatchedText`、`Content` 等用户可控内容直接写 CSV。以 `=`、`+`、`-`、`@` 开头的单元格在 Excel 打开时会被当公式执行。
- **修复**：对以这四个字符开头的字段前置 `'` 或空格转义。

### 🟡 M-9 审计/运行日志表无限增长
- **位置**：`runtime_logs`、`user_role_audit_logs`、`notifications` 等表无保留期策略、无清理任务。
- **修复**：增加定时清理（如保留 90 天），可复用现有 ticker 模式。

### 🟡 M-10 `restoreUserRoleHandler` 错误检查顺序错误
- **位置**：`internal/admin/users.go:130-148`：第二个 UPDATE 的 `err` 覆盖后才检查 `sql.ErrNoRows`，导致"用户不存在"永远返回 500 而非 404。

---

## 四、性能与代码质量（低优先级）

| # | 问题 | 位置 | 建议 |
|---|------|------|------|
| L-1 | `matchConfiguredRule` 每行每规则重复 `strings.Split(rule.Rule.Keyword, "\|")` | `parser.go:177` | 在 `compileRules` 阶段预计算 labels 存入 `compiledRule` |
| L-2 | `couldContainRelatedCause` 19 个关键词逐个 `Contains`，每行调用 | `parser.go:270-283` | 合并为单个编译正则或 Aho-Corasick |
| L-3 | `trimLogContent` 按字节截断 4000 可能切断 UTF-8 | `parser.go:374-379` | 参照 `truncateString` 的回退逻辑修正 |
| L-4 | 每文件循环内查询 `IsParseTaskStopped` | `service.go:640` | 心跳续租已能发现租约丢失，可降频为每 N 个文件查一次 |
| L-5 | `decodeProject` 每次触发 3 次 EXISTS 查询 | `admin/service.go:384-392` | 合并为一条查询 |
| L-6 | SPA 每请求 `os.Stat` 两次 | `web/spa.go:34-48` | 可接受，高流量时再优化 |
| L-7 | `fmt.Println` 打印启动信息、魔法状态码（500/401）、auth/admin 重复实现 `writeError` | 多处 | 统一用 `log` + `http.Status*` 常量，抽公共 middleware 包 |
| L-8 | `resultHandler`/`taskHandler` 千行巨型函数、手工字符串切分路由 | `handlers.go:483-649,1284-1602` | Go 1.22+ ServeMux 已支持 `"GET /api/results/{id}/status"` 模式，可大幅简化 |
| L-9 | migration 存在两个 `026_` 前缀文件 | `database/migrations/` | 后续迁移统一从新序号开始 |
| L-10 | `securevalue` 包整体死代码、`ai_analysis_config.llm_api_key_encrypted` 列写空 | `securevalue/` | 接入 M-6 或删除 |
| L-11 | `/api/auth/me` 忽略 Scan 错误可能返回陈旧 role | `auth/handlers.go:147-149` | 处理错误或降级打日志 |
| L-12 | `envInt` 无法配置 0 值（`parsed <= 0` 回退） | `config.go:96,109` | 视语义改为仅 `< 0` 回退 |
| L-13 | `main.go` 启动时未输出配置清单（密钥缺失静默降级） | `main.go:57-61` | 启动时打印各功能是否已配置（不打印值） |

---

## 五、做得好的地方（保持）

- **任务队列设计**：`FOR UPDATE SKIP LOCKED` 认领、租约 + 心跳续期、`ReconcileParseQueue/ReconcileAIQueue` 崩溃恢复、`run_token` 防僵尸任务——这套机制完整且正确。
- **SQL 全参数化**，未发现注入点；审批流程用 `FOR UPDATE` 行锁 + 竞态校验。
- **上传路径安全**：`safeStorageSegment`/`safeUploadFilePath`/`removeUploadStorage` 三处防路径穿越，Windows 保留设备名也处理了。
- **上传幂等**：`client_request_id` + 幂等键查询，避免重复上传。
- **LLM 提示注入防护**：系统指令声明日志正文为不可信数据；输入截断、多样化采样（`selectDiverseMatches`）、响应大小限制齐全。
- **密码**：Argon2id + 恒定时间比较；迁移按文件名排序、每个迁移独立事务。

---

## 六、建议实施顺序

> 注：首个登录用户自动成为超管（`auth/service.go:69-76`）与职位关键词映射角色（`rolepolicy.go`）经确认为既定设计，不在本报告问题清单内。

1. **第一周（安全止血）**：P0-1 令牌泄漏 → H-3 Cookie/令牌加固。
2. **第二周（稳定性）**：H-1 Server 超时 + recover + 优雅停机 → H-4 注入 roleResolver → H-2 会话过期。
3. **第三周（性能）**：H-5/H-6 AI 队列查询优化 → P0-2 搜索限流 → M-2/M-3 admin 查询优化。
4. **第四周（加固）**：M-5 限流 → M-8 CSV 注入 → M-9 数据保留策略 → 其余 L 级清理。
