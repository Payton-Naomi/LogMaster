# LogMaster

接口说明见 [docs/API接口文档.md](docs/API接口文档.md)，数据库说明见 [docs/数据库设计文档.md](docs/数据库设计文档.md)，服务器升级与部署见 [docs/服务器部署文档.md](docs/服务器部署文档.md)。

软测日志分析平台 — 嵌入式行车记录仪串口日志的自动采集、集中存储、智能解析与可视化展示平台。

## 项目概述

嵌入式行车记录仪在研发和测试过程中通过串口（UART）输出大量运行日志，本平台实现多产线日志的自动采集、集中存储、智能解析与可视化展示。核心目标：让设备日志「说话」——从被动查看到主动告警，从人工排查到 AI 辅助诊断。

## 技术栈

| 层级 | 技术 |
|------|------|
| 串口采集 Agent | Go + go.bug.st/serial + robfig/cron |
| Web 后端 | Go + Gin + GORM + Zap |
| 前端 | Vue 3 + Vite + Element Plus + ECharts + Pinia |
| 业务数据库 | PostgreSQL 15+ |
| 日志存储 | Grafana Loki |
| 可视化 | Grafana |
| AI 分析 | 通义千问 Qwen3-Max |

## 系统架构

```
行车记录仪设备 (UART 串口)
        │
        ▼
挂测 PC 层 (Go 日志采集 Agent)
  · Goroutine per Device → Channel → Worker Pool → HTTP Upload
        │
        ▼
中心服务器层
  ├── Go Web 后端 (Gin)  ←──→  PostgreSQL (业务数据)
  ├── Grafana Loki (日志存储+索引)
  ├── AI 分析引擎 (通义千问 API)
  └── Vue 3 前端 Web 平台
```

## 核心功能

- **串口日志采集**：多设备并发采集、本地缓冲、断线重连、定时批量上传
- **日志检索**：按项目/设备/级别/模块多维度筛选，全文搜索
- **规则引擎**：关键字 + 正则匹配，自动分类和打标签
- **AI 智能分析**：异常日志自动提交大模型进行语义分析，生成诊断报告
- **告警中心**：异常自动告警，含 AI 修复建议
- **仪表板**：设备状态、日志量统计、异常趋势可视化

## 快速开始

### 环境要求

- Go 1.22+
- Node.js 20 LTS
- PostgreSQL 15+
- Grafana Loki 3.x
- Grafana 10.x

### 单服务启动

```powershell
npm.cmd --prefix frontend install
npm.cmd --prefix frontend run build
$env:DATABASE_URL="postgres://logmaster:logmaster@127.0.0.1:5432/logmaster?sslmode=disable"
go run .
```

访问 `http://localhost:8080`，Go 会同时提供 Vue 前端和 `/api` 后端接口。

### 前端开发模式

```powershell
cd frontend
npm.cmd run dev
```

开发模式访问 `http://localhost:3000`，Vite 会将 `/api` 请求代理到 8080。

### Agent 部署

```bash
cd agent
cp config/config.yaml.example config/config.yaml
# 编辑配置文件，填写设备串口信息及后端地址
go run main.go
```

## 项目结构

```
LogMaster/
├── agent/           # Go 串口日志采集 Agent
├── backend/         # Go Web 后端 (Gin)
├── frontend/        # Vue 3 前端
└── docs/            # 文档
```

## 数据库表

| 表名 | 说明 |
|------|------|
| projects | 项目/产线管理 |
| devices | 设备信息与串口配置 |
| log_sessions | 日志上传会话 |
| parse_rules | 解析规则配置 |
| keywords | 关键字规则 |
| parse_tasks | 解析任务记录 |
| alerts | 告警记录 |
| users | 用户管理 |

## 团队

- 成员 A：后端 + 基础设施
- 成员 B：前端 + 可视化
- 成员 C：Agent + AI 集成

## 许可证

MIT
