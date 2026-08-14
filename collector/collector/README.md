# LogMaster Agent MVP

这是一个运行在 Windows 挂测机上的 Go Agent：它可以采集串口日志、写入本机 SQLite 队列、通过当前后端的 multipart 接口上传，并提供后端调用的 `POST /analyze` 规则诊断服务。

## 快速验证

在 `agent/` 目录执行：

```powershell
go run ./cmd/logmaster-agent -config ./configs/agent.example.yaml -demo
```

启动后可访问：

- `http://127.0.0.1:9000/healthz`
- `http://127.0.0.1:9000/metrics`
- `POST http://127.0.0.1:9000/analyze`

`-demo` 每 500ms 生成一条样例日志，使用与真实串口相同的分段和 SQLite 队列。后端不可用时文件会保留在 `data/spool`，不会丢失。

## 部署前必须手动配置

桌面客户端部署请从 `config_template.yaml` 生成现场配置；命令行 Agent 快速验证仍可使用 `configs/agent.example.yaml`：

1. `backend.base_url` 必须指向实际后端并以 `/api` 结尾；填写 `project_name` 和 `version`。
2. 启用真实串口配置，填写 `device_sn`、COM 名称、VID/PID、USB 序列号及串口参数。
3. 后端设置 `AGENT_ANALYSIS_URL`，例如 `http://127.0.0.1:9000/analyze`。
4. 生产环境设置 `AGENT_ANALYSIS_TOKEN`，Agent 与后端必须一致；示例文件不保存 Token。
5. SQLite、spool、日志目录和磁盘容量上限需要按挂测机调整。
6. 需要模型时再填写 Ollama/Qwen 地址、模型名和 API Key 环境变量；当前示例全部留空并默认使用 `rules`。
7. Windows 服务账号必须拥有串口、SQLite、日志目录和网络权限；防火墙只放行后端 IP 到 9000 端口。
8. 生产环境还需要后端补充 HTTPS、上传鉴权和幂等键。

程序启动时会输出缺失配置的人工提醒，但不会输出 Token、API Key 或完整 Prompt。

## Analyzer 请求示例

```powershell
$body = @{ task_id='11111111-1111-4111-8111-111111111111'; upload_id='22222222-2222-4222-8222-222222222222'; file=@{ id=1; relative_path='demo.log'; size_bytes=10; sha256=('0' * 64); line_count=1 }; total_lines=1; matches=@(@{ level='ERROR'; matched_text='ERROR'; line_number=1; content='ERROR camera initialization failed'; file_path='demo.log' }) } | ConvertTo-Json -Depth 8
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:9000/analyze -ContentType 'application/json' -Body $body
```

## 测试和构建

```powershell
go test ./...
go vet ./...
.\tests\Test-DeliveryArtifacts.ps1
.\build.ps1
```

`build.ps1` 构建 `desktop/frontend` 和 Wails 桌面项目，产物为 `bin/LogCollector.exe`，并输出实际体积与 SHA-256。目标机器必须安装 WebView2 Evergreen Runtime。

## 交付与验收资料

- [用户手册](docs/user-guide.md)
- [部署与运维手册](docs/deployment-operations.md)
- [故障排查](docs/troubleshooting.md)
- [接口对接说明](docs/api-integration.md)
- [已知限制](docs/known-limitations.md)
- [四路和长时测试说明](tests/README.md)
- [验收报告模板](tests/test-report-template.md)

四路虚拟串口稳定性测试使用 `tools/Invoke-StabilityTest.ps1`。工具支持参数化时长、单路断开、Mock 后端中断、强制重启和显式授权的磁盘压力，并输出 NDJSON 原始证据与 Markdown 报告。8 小时与 24 小时需要在发布前实际执行；72 小时在发布后试点工位执行。
