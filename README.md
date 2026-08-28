<div align="center">
  <img src="collector/collector/desktop/frontend/public/app-icon.png" width="96" alt="LogMaster Logo" />
  <h1>LogMaster</h1>
  <p>面向设备测试场景的端云一体日志采集与 AI 辅助分析平台</p>

  <p>
    <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26" />
    <img src="https://img.shields.io/badge/Vue-3-42B883?logo=vuedotjs&logoColor=white" alt="Vue 3" />
    <img src="https://img.shields.io/badge/Wails-v2-DF0000" alt="Wails v2" />
    <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white" alt="PostgreSQL 16" />
  </p>
</div>

LogMaster 用于采集 Windows 挂测机上的设备串口日志，并将日志可靠上传至统一平台。系统通过规则筛选异常，再由 AI 结合上下文生成根因、证据、影响和处理建议，同时保留原始日志位置供工程师复核。

> 项目原则：AI 提供辅助诊断，不替代工程师的最终判断；所有结论应能回到原始日志证据。

## 核心能力

- **多路串口采集**：动态发现 Windows 串口，每个通道独立采集、保存和重连。
- **本地优先存储**：日志先写入分段文件，网络或界面异常不阻塞采集。
- **可靠上传**：SQLite 持久化上传队列，支持失败重试、断点恢复和响应未知核对。
- **规则异常检测**：支持普通关键字与正则规则，实时记录命中次数和日志位置。
- **AI 辅助诊断**：提供文件级诊断和任务级风险汇总，输出可追溯的结构化结论。
- **任务协同闭环**：支持结果查询、负责人、评论、通知及 Jira 关联。
- **运行可观测**：采集状态、上传进度、解析任务和诊断日志均可追踪。

## 工作流程

```text
设备串口
   │
   ▼
Windows 采集端 ── 本地文件 + SQLite 队列
   │
   ▼
日志管理平台 ── 接收、存储、任务管理
   │
   ├── 规则引擎 ── 异常筛选与上下文定位
   │
   └── AI 分析 ── 文件诊断与任务风险汇总
                         │
                         ▼
                证据复核与问题协同
```

## 技术架构

| 模块 | 技术栈 | 职责 |
| --- | --- | --- |
| 桌面采集端 | Go、Wails v2、Vue 3、SQLite | 串口采集、本地保存、关键字匹配、可靠上传 |
| Web 管理端 | Vue 3、Element Plus、ECharts | 任务管理、日志查询、结果展示和系统配置 |
| 服务端 | Go、PostgreSQL | 鉴权、上传、解析、任务调度和 API |
| AI 分析 | OpenAI 兼容接口 | 文件级诊断、任务级汇总和建议生成 |
| 部署 | Docker Compose | 应用与 PostgreSQL 的容器化部署 |

## 仓库结构

```text
LogMaster/
├── collector/collector/   # Windows 桌面采集端
│   ├── desktop/           # Wails 桌面层与 Vue 界面
│   ├── internal/          # 串口、分段存储、队列和上传核心
│   ├── configs/           # 默认配置与配置模板
│   ├── docs/              # 采集端用户、运维和接口文档
│   └── tests/             # 采集端验收与稳定性测试
├── server/
│   ├── backend/           # Go 服务端
│   ├── frontend/          # Vue 管理端
│   ├── docker-compose.yml
│   └── Dockerfile
└── docs/                  # 项目、部署、接口和答辩资料
```

## 快速开始

### 启动服务端

前置条件：Docker 与 Docker Compose。

```powershell
Set-Location server
Copy-Item .env.example .env
```

根据注释填写 `.env` 中的数据库密码、访问地址和飞书应用配置。不要提交真实密钥，然后启动服务：

```powershell
docker compose up -d --build
```

服务默认监听 `http://localhost:8080`，健康检查地址为 `http://localhost:8080/api/health`。

### 运行采集端

当前 Windows 交付版本为 `LogCollector 0.0.10`：

```text
collector/collector/release/LogCollector-0.0.10/
```

保持 `LogCollector-0.0.10.exe` 与 `config` 目录位于同一目录。首次运行前确认：

1. 已安装 Microsoft Edge WebView2 Evergreen Runtime。
2. 后端地址、项目、任务和关键字配置符合现场环境。
3. 当前 Windows 用户拥有串口、日志目录和网络访问权限。
4. 真实 Token 通过受控配置或环境变量提供，不写入仓库。

### 从源码构建采集端

前置条件：Go、Node.js、pnpm、Wails CLI 和 WebView2 构建环境。

```powershell
Set-Location collector/collector
./build.ps1
```

构建产物输出到 `collector/collector/bin/LogCollector.exe`。

## 验证

采集端：

```powershell
Set-Location collector/collector
go test ./...
./tests/Test-DeliveryArtifacts.ps1
```

管理端：

```powershell
Set-Location server/frontend
npm install
npm run build
```

> 多路并发和长时间稳定性结论应以对应测试报告中的设备、吞吐和 Windows 环境为准。代码支持上限不等于所有规模均已完成验收。

## 可靠性说明

上传队列包含 `pending`、`uploading`、`uploaded`、`uncertain` 和 `dead` 五种状态。请求可能已送达但确认响应丢失时，批次进入 `uncertain`，系统不会无条件自动重传，需结合文件 SHA-256、大小、设备和时间范围在平台核对，以避免重复任务。

当前版本不宣称服务端 exactly-once。磁盘阈值属于应用保护机制，也不能替代操作系统级容量监控。

## 文档

- [课题说明与成果汇报](docs/课题说明文档-LogMaster日志分析平台-汇报版.md)
- [采集端答辩知识手册](docs/采集端答辩知识手册.md)
- [服务器部署文档](docs/服务器部署文档.md)
- [Docker 部署文档](docs/Docker部署文档.md)
- [后端 API](docs/backend-api.md)
- [采集端接口对接](docs/api-to-collector.md)
- [数据库设计](docs/数据库设计文档.md)
- [采集端用户手册](collector/collector/docs/user-guide.md)
- [采集端故障排查](collector/collector/docs/troubleshooting.md)
- [已知限制](collector/collector/docs/known-limitations.md)

## 安全提示

- 不要提交 `.env`、访问令牌、飞书应用密钥或大模型 API Key。
- 生产环境应启用 HTTPS，并限制采集端 Token 的权限和使用范围。
- 未上传日志不得自动删除；清理前必须确认平台已完整接收。
- AI 输出仅作为辅助诊断，关键问题必须由工程师结合原始证据复核。

## 当前状态

项目已打通“串口采集 → 本地保存 → 云端上传 → 规则解析 → AI 诊断 → 结果展示”链路，适合功能演示和小范围试用。后续重点是补充更多真实设备的长时间运行记录、量化 AI 诊断效果，并完善部署与升级流程。
