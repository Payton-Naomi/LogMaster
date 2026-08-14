# LogMaster：从 Windows 构建并部署到 Ubuntu

本文档用于将 Windows 电脑上的 LogMaster 项目构建成 Linux 二进制文件，并将后端程序和前端页面发布到 Ubuntu 服务器。

## 一、终端识别

部署过程中会使用两个命令端，请根据提示符判断命令应该在哪里运行。

### Windows PowerShell

提示符类似：

```text
PS C:\Users\wangzhanying\Desktop\ai\26728\LogMaster>
```

用于：

- 构建前端；
- 交叉编译 Linux 后端；
- 通过 SSH 和 SCP 连接 Ubuntu；
- 上传发布文件。

### Ubuntu 服务器终端

提示符类似：

```text
root@VM-0-7-ubuntu:~#
```

用于：

- 校验上传文件；
- 备份当前版本；
- 安装新版本；
- 启动服务并检查日志；
- 必要时回滚。

不要把提示符本身复制进命令，只复制代码块中的命令。

## 二、部署信息

本文档使用以下路径和名称：

| 项目 | 值 |
|---|---|
| Windows 项目目录 | `C:\Users\wangzhanying\Desktop\ai\26728\LogMaster` |
| Ubuntu 地址 | `124.222.162.103` |
| Ubuntu 临时上传目录 | `/tmp/logmaster-release` |
| 后端安装位置 | `/opt/logmaster/logmaster-server` |
| 前端安装位置 | `/opt/logmaster/frontend/dist` |
| systemd 服务 | `logmaster.service` |
| 默认本地端口 | `8080` |

## 三、Windows：构建发布文件

以下命令全部在 Windows PowerShell 中执行。

### 1. 进入项目目录

```powershell
Set-Location 'C:\Users\wangzhanying\Desktop\ai\26728\LogMaster'
```

### 2. 构建前端

```powershell
npm.cmd --prefix frontend run build
```

出现 `built in ...` 表示构建成功。关于大于 500 kB chunk 或 `PURE` annotation 的信息是构建警告，不会阻止本次发布。

确认前端首页已经生成：

```powershell
Test-Path .\frontend\dist\index.html
```

输出必须为：

```text
True
```

### 3. 将 Go 后端编译成 Linux AMD64 二进制文件

```powershell
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -trimpath -ldflags='-s -w' -o logmaster .
```

确认二进制文件存在：

```powershell
Get-Item .\logmaster
```

如果 Ubuntu 服务器运行 `uname -m` 后显示 `aarch64`，需要把 `$env:GOARCH` 改成 `arm64` 后重新编译。常见的 `x86_64` 服务器使用本文档的 `amd64`。

## 四、Windows：上传发布文件

以下命令仍然在 Windows PowerShell 中执行。

### 1. 创建服务器临时目录

```powershell
ssh root@124.222.162.103 "rm -rf /tmp/logmaster-release && mkdir -p /tmp/logmaster-release/frontend"
```

根据提示输入 Ubuntu 服务器的 root 密码。

### 2. 上传后端

```powershell
scp .\logmaster root@124.222.162.103:/tmp/logmaster-release/
```

### 3. 上传前端

```powershell
scp -r .\frontend\dist root@124.222.162.103:/tmp/logmaster-release/frontend/
```

文件显示 `100%` 表示传输完成。Windows OpenSSH 偶尔会在完成后显示：

```text
close - IO is still pending on closed socket
```

只要文件均已显示 `100%`，可以继续执行服务器校验；最终以服务器上的文件校验结果为准。

## 五、进入 Ubuntu 服务器

在 Windows PowerShell 中执行：

```powershell
ssh root@124.222.162.103
```

输入密码。看到下面这种提示符后，说明已经进入 Ubuntu：

```text
root@VM-0-7-ubuntu:~#
```

从下一节开始，所有命令都在 Ubuntu 服务器终端执行。

## 六、Ubuntu：部署前校验

先校验文件，再停止服务。不要跳过本节。

```bash
file /tmp/logmaster-release/logmaster
test -s /tmp/logmaster-release/logmaster && echo "后端文件正常"
test -f /tmp/logmaster-release/frontend/dist/index.html && echo "前端首页正常"
test -d /tmp/logmaster-release/frontend/dist/assets && echo "前端资源目录正常"
```

`file` 输出中必须包含 `ELF 64-bit`，并且随后应显示三行“正常”。如果显示 `PE32` 或没有显示“正常”，停止部署并返回 Windows 重新构建或上传。

确认服务器架构：

```bash
uname -m
```

- `x86_64` 对应 Windows 构建时的 `GOARCH=amd64`；
- `aarch64` 对应 `GOARCH=arm64`。

## 七、Ubuntu：备份当前版本

保持在同一个 Ubuntu 会话中执行：

```bash
BACKUP_DIR=/opt/logmaster-backup/$(date +%Y%m%d-%H%M%S)
mkdir -p "$BACKUP_DIR"

if [ -f /opt/logmaster/logmaster-server ]; then
  cp -a /opt/logmaster/logmaster-server "$BACKUP_DIR/logmaster-server"
fi

if [ -d /opt/logmaster/frontend/dist ]; then
  cp -a /opt/logmaster/frontend/dist "$BACKUP_DIR/frontend-dist"
fi

echo "本次备份位置：$BACKUP_DIR"
```

记住命令输出的备份位置。`BACKUP_DIR` 变量只在当前 Ubuntu 登录会话中有效。

## 八、Ubuntu：准备新前端

在停止服务前先把新前端复制到临时安装目录：

```bash
rm -rf /opt/logmaster/frontend/dist.new
cp -a /tmp/logmaster-release/frontend/dist /opt/logmaster/frontend/dist.new
chown -R logmaster:logmaster /opt/logmaster/frontend/dist.new
test -f /opt/logmaster/frontend/dist.new/index.html && echo "新前端准备完成"
```

必须看到“新前端准备完成”后才能继续。

## 九、Ubuntu：安装并启动新版本

### 1. 停止服务并安装后端

```bash
systemctl stop logmaster.service

install -o logmaster -g logmaster -m 0750 \
  /tmp/logmaster-release/logmaster \
  /opt/logmaster/logmaster-server
```

### 2. 切换前端目录

```bash
rm -rf /opt/logmaster/frontend/dist.old

if [ -d /opt/logmaster/frontend/dist ]; then
  mv /opt/logmaster/frontend/dist /opt/logmaster/frontend/dist.old
fi

mv /opt/logmaster/frontend/dist.new /opt/logmaster/frontend/dist
chown -R logmaster:logmaster /opt/logmaster/frontend/dist
```

### 3. 检查运行权限

```bash
sudo -u logmaster test -x /opt/logmaster/logmaster-server \
  && echo "后端权限正常"

sudo -u logmaster test -r /opt/logmaster/frontend/dist/index.html \
  && echo "前端权限正常"
```

### 4. 启动服务

```bash
systemctl start logmaster.service
systemctl status logmaster.service --no-pager
```

状态中出现下面内容表示服务正在运行：

```text
Active: active (running)
```

## 十、Ubuntu：发布后验证

### 1. 检查首页

```bash
curl -I http://127.0.0.1:8080/
```

正常情况下会返回 `HTTP/1.1 200 OK`。如果实际配置的端口不是 `8080`，请使用 `/etc/logmaster.env` 中配置的端口。

### 2. 检查服务日志

```bash
journalctl -u logmaster.service -n 50 --no-pager
```

### 3. 确认安装的后端与上传文件一致

```bash
sha256sum \
  /tmp/logmaster-release/logmaster \
  /opt/logmaster/logmaster-server
```

两行 SHA-256 值应当完全相同。

### 4. 浏览器验证

在 Windows 浏览器中打开实际配置的 LogMaster 网站地址，检查：

- 首页能正常显示；
- 登录和主要页面可以打开；
- 浏览器强制刷新后仍能正常加载；
- 新功能或修复已经生效。

## 十一、部署失败时回滚

只有部署后服务无法恢复时才执行本节。以下命令在 Ubuntu 服务器执行。

如果仍在创建备份时的同一个 Ubuntu 会话中，可以直接使用 `$BACKUP_DIR`。如果重新登录过，先找到最近一次备份：

```bash
BACKUP_DIR=$(ls -dt /opt/logmaster-backup/* | head -1)
echo "准备回滚到：$BACKUP_DIR"
ls -la "$BACKUP_DIR"
```

确认输出确实是本次部署前的备份，然后执行：

```bash
systemctl stop logmaster.service

if [ -f "$BACKUP_DIR/logmaster-server" ]; then
  install -o logmaster -g logmaster -m 0750 \
    "$BACKUP_DIR/logmaster-server" \
    /opt/logmaster/logmaster-server
fi

if [ -d "$BACKUP_DIR/frontend-dist" ]; then
  rm -rf /opt/logmaster/frontend/dist.failed
  if [ -d /opt/logmaster/frontend/dist ]; then
    mv /opt/logmaster/frontend/dist /opt/logmaster/frontend/dist.failed
  fi
  cp -a "$BACKUP_DIR/frontend-dist" /opt/logmaster/frontend/dist
  chown -R logmaster:logmaster /opt/logmaster/frontend/dist
fi

systemctl start logmaster.service
systemctl status logmaster.service --no-pager
journalctl -u logmaster.service -n 50 --no-pager
```

## 十二、退出 Ubuntu

部署和验证完成后，在 Ubuntu 终端执行：

```bash
exit
```

随后会返回 Windows PowerShell 提示符。

## 十三、最简顺序清单

每次发布严格按以下顺序执行：

1. Windows：`npm.cmd --prefix frontend run build`；
2. Windows：设置 `GOOS=linux`、`GOARCH=amd64`、`CGO_ENABLED=0`；
3. Windows：执行 `go build`；
4. Windows：通过 `ssh` 创建 `/tmp/logmaster-release`；
5. Windows：通过 `scp` 上传后端和 `frontend/dist`；
6. Windows：通过 `ssh` 登录 Ubuntu；
7. Ubuntu：校验 ELF 二进制、`index.html` 和 `assets`；
8. Ubuntu：备份当前后端和前端；
9. Ubuntu：准备 `dist.new`；
10. Ubuntu：停止服务；
11. Ubuntu：安装后端并切换前端；
12. Ubuntu：启动服务；
13. Ubuntu：检查状态、日志、HTTP 响应和 SHA-256；
14. Ubuntu：验证完成后执行 `exit`。
