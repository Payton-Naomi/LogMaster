# 验收测试

先运行静态交付检查：

```powershell
powershell -ExecutionPolicy Bypass -File .\tests\Test-DeliveryArtifacts.ps1
```

四路虚拟串口测试需要先用 com0com 或等价工具建立四组串口对，例如 `COM10<->COM11`、`COM12<->COM13`、`COM14<->COM15`、`COM16<->COM17`。采集端使用偶数端口，模拟器使用奇数端口。

十分钟预检：

```powershell
.\tools\Invoke-StabilityTest.ps1 -Duration '00:10:00' -WriterPorts COM11,COM13,COM15,COM17
```

网络中断测试必须额外提供可由工具启停的 Mock 后端，例如增加 `-Faults serial,network,restart -MockServerExecutable <path> -MockServerArguments <args>`。正式验收默认要求 `/metrics` 中四个设备的接收字节均大于零；只有调试工具自身时才可临时使用 `-SkipCollectorMetrics`。

正式分层验收分别使用 `08:00:00`、`1.00:00:00` 和 `3.00:00:00`。72 小时测试属于发布后试点验证。每次运行在 `tests/results/<UTC时间>/` 下生成 NDJSON 原始证据和 Markdown 报告。
