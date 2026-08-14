# 部署与运维手册

## 环境要求

- Windows 10 1809、较新 Windows 10 或 Windows 11，64 位。
- Microsoft Edge WebView2 Evergreen Runtime。
- 构建机需要 Go 1.26.4、Node.js、npm 和 Wails v2 CLI；运行机不需要 Go 或 Node.js。
- 运行账号需要串口、配置目录、SQLite、spool、日志目录和后端网络权限。

## 构建

在 `agent` 目录运行：

```powershell
.\build.ps1
```

脚本依次运行 Go 测试与静态检查、`desktop/frontend` 的 `npm ci` 和前端构建、`desktop` 的 Wails Windows 构建。发布产物为 `bin/LogCollector.exe`，脚本输出实际体积与 SHA-256。`-SkipDependencyInstall` 仅适用于已经存在锁文件对应 `node_modules` 的离线构建；`-RequireWebView2` 可将构建机缺少 WebView2 视为失败。

## 首次部署

1. 建立独立目录，例如 `C:\Program Files\LogMaster` 放 EXE，`C:\ProgramData\LogMaster` 放配置和数据。
2. 基于 `config_template.yaml` 生成现场配置，前四路改为 `enabled: true` 并填写真实端口信息。
3. 设置 `AGENT_ANALYSIS_TOKEN` 等机密环境变量，禁止把机密写入模板或日志。
4. 放通到后端 HTTPS 地址的出站访问。分析接口若对外监听，应只允许受控后端访问。
5. 启动后检查界面、健康状态、四路串口、数据落盘和一次完整上传。

## 日常巡检

每天检查进程存活、四路连接状态、待上传和 `uncertain/dead` 数量、磁盘余量、最新文件时间和后端可达性。每周抽查文件 SHA-256、异常恢复和重启后队列恢复。任何自动清理只能处理已经明确 `uploaded` 且超过保留期的文件。

## 备份、升级与回退

升级前停止客户端，备份配置、SQLite 和 spool 索引，记录旧 EXE SHA-256。只替换 EXE 后执行短时回归；失败时停止新版本并恢复旧 EXE，继续使用原数据目录。禁止在客户端运行时复制 SQLite 主文件作为一致性备份。

## 长时验收

使用 `tools/Invoke-StabilityTest.ps1` 执行 8 小时冒烟和 24 小时回归，报告必须注明四路吞吐、平均行长、上传频率、内存变化及故障注入结果。72 小时在试点工位执行，并保留整个 `tests/results/<run-id>` 目录。
