# LogMaster Docker 部署文档

本文档记录 LogMaster 服务端（Go 后端 + Vue 前端 + PostgreSQL）在 Linux 服务器上通过 Docker 部署的完整流程。所有命令均已在实际环境（阿里云 ECS，Ubuntu 22.04）验证通过。

## 1. 部署架构

```text
浏览器 / 采集端
      │  http://<公网IP>:8080
      ▼
┌─────────────────────────────────┐
│  Docker Compose                 │
│                                 │
│  ┌───────────────┐  ┌─────────┐ │
│  │ app 容器       │──│ postgres │ │
│  │ (Go 后端+前端) │  │ 容器     │ │
│  │ 监听 8080      │  │ 5432     │ │
│  └───────────────┘  └─────────┘ │
│         │                │      │
│         ▼                ▼      │
│    logs 数据卷       pgdata 卷   │
└─────────────────────────────────┘
```

- `app` 容器：Go 二进制 + `frontend/dist`，同时提供页面和 `/api/*` 接口。
- `postgres` 容器：PostgreSQL 16，数据持久化在 `pgdata` 卷。
- 日志文件持久化在 `logs` 卷。

## 2. 前置条件

- 一台 Linux 服务器（Ubuntu 22.04 / 24.04 LTS），有公网 IP。
- 已安装 Docker Engine + Docker Compose v2 插件。
- 飞书开放平台已创建应用，取得 `App ID` 和 `App Secret`。
- 服务器能访问飞书 `open.feishu.cn`（TCP 443 出站）。

## 3. 文件清单

在 `server/` 目录（`backend/` 与 `frontend/` 的父目录）下准备 4 个文件。

### 3.1 `Dockerfile`

```dockerfile
# 阶段1：构建前端（Node 24，与本地构建环境一致）
FROM node:24-bookworm AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --registry=https://registry.npmmirror.com
COPY frontend/ ./
# 接口前缀与登录回跳地址都走同源路径（VITE_API_BASE_URL 补齐 /api 前缀）
ENV VITE_API_BASE_URL=/api \
    VITE_FEISHU_LOGIN_URL=/api/auth/feishu-login
RUN npm run build

# 阶段2：构建后端（Go 1.26.4，静态编译，无 CGO）
FROM golang:1.26.4 AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY backend/ ./
RUN mkdir -p /out && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/logmaster .

# 阶段3：最小运行镜像
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /opt/logmaster
COPY --from=backend /out/logmaster /opt/logmaster/logmaster-server
COPY --from=frontend /src/frontend/dist /opt/logmaster/frontend/dist
ENV FRONTEND_DIST_DIR=/opt/logmaster/frontend/dist
EXPOSE 8080
ENTRYPOINT ["/opt/logmaster/logmaster-server"]
```

### 3.2 `docker-compose.yml`

```yaml
services:
  postgres:
    image: postgres:16
    restart: unless-stopped
    environment:
      POSTGRES_USER: logmaster
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: logmaster
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U logmaster -d logmaster"]
      interval: 5s
      timeout: 5s
      retries: 12

  app:
    build: .
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://logmaster:${DB_PASSWORD}@postgres:5432/logmaster?sslmode=disable
      FRONTEND_DIST_DIR: /opt/logmaster/frontend/dist
      LOG_STORAGE_DIR: /var/lib/logmaster/logs
      PUBLIC_BASE_URL: ${PUBLIC_BASE_URL}
      FEISHU_APP_ID: ${FEISHU_APP_ID}
      FEISHU_APP_SECRET: ${FEISHU_APP_SECRET}
      FEISHU_REDIRECT_URI: ${FEISHU_REDIRECT_URI}
      FEISHU_SUPER_ADMIN_NAMES: ${FEISHU_SUPER_ADMIN_NAMES}
      MAX_UPLOAD_BYTES: "2147483648"
      MAX_EXTRACT_BYTES: "8589934592"
      LLM_API_BASE_URL: ${LLM_API_BASE_URL:-}
      LLM_API_KEY: ${LLM_API_KEY:-}
      LLM_MODEL: ${LLM_MODEL:-qwen-plus}
      LLM_TIMEOUT_SECONDS: ${LLM_TIMEOUT_SECONDS:-120}
      LLM_MAX_MATCHES: ${LLM_MAX_MATCHES:-50}
      LLM_MAX_INPUT_BYTES: ${LLM_MAX_INPUT_BYTES:-200000}
    ports:
      - "8080:8080"
    volumes:
      - logs:/var/lib/logmaster/logs

volumes:
  pgdata:
  logs:
```

### 3.3 `.dockerignore`

```text
frontend/node_modules
frontend/dist
backend/logmaster
.env
.git
**/*.log
```

### 3.4 `.env.example`（复制为 `.env` 后填写真实值）

```dotenv
# 数据库密码：只允许字母和数字，不要用 @ : / # ? 等特殊字符
DB_PASSWORD=改成你的密码仅字母数字

# 公网访问地址（试用期用 IP + 端口即可）
PUBLIC_BASE_URL=http://121.43.234.112:8080

# 飞书应用凭证
FEISHU_APP_ID=cli_aac4efb073789bd0
FEISHU_APP_SECRET=改成你的飞书secret

# 飞书 OAuth 回调地址，必须与飞书开放平台后台登记的一致
FEISHU_REDIRECT_URI=http://121.43.234.112:8080/api/auth/callback

# 自动成为管理员的飞书姓名（对应你在飞书上的真实姓名）
FEISHU_SUPER_ADMIN_NAMES=改成你的飞书姓名

# ===== AI 分析（可选；不配置 LLM_API_BASE_URL 则 AI 分析不启用，关键字规则兜底） =====
# 大模型 OpenAI 兼容接口地址（末尾不要带 /chat/completions）
#   通义千问：https://dashscope.aliyuncs.com/compatible-mode/v1
#   DeepSeek：https://api.deepseek.com/v1
#   本地 Ollama：http://<ollama主机>:11434/v1
LLM_API_BASE_URL=
LLM_API_KEY=
LLM_MODEL=qwen-plus
LLM_TIMEOUT_SECONDS=120
LLM_MAX_MATCHES=50
LLM_MAX_INPUT_BYTES=200000
```

## 4. 部署步骤

### 4.1 本机生成 `.env` 并填写

```powershell
cd server
copy .env.example .env
notepad .env   # 填写数据库密码、飞书凭证、姓名，并确认 IP
```

### 4.2 打包源码并上传

```powershell
cd server
tar --exclude=frontend/node_modules --exclude=frontend/dist -czf logmaster-src.tar.gz frontend backend Dockerfile docker-compose.yml .dockerignore .env
scp .\logmaster-src.tar.gz root@<服务器IP>:/opt/
```

### 4.3 服务器解压

```bash
mkdir -p /opt/logmaster-server
tar -xzf /opt/logmaster-src.tar.gz -C /opt/logmaster-server
cd /opt/logmaster-server
```

### 4.4 确认 Docker 环境

```bash
docker --version
docker compose version
systemctl is-active docker
```

- 已有输出版本号且状态为 `active` 则跳过安装。
- 未安装则执行：`apt update && apt install -y docker.io docker-compose-v2 && systemctl enable --now docker`。
- 若安装 `docker.io` 报 `containerd.io: Conflicts: containerd`，说明机器已预装 Docker CE，直接跳过安装即可（本机实际就是这种情况）。

### 4.5 配置镜像加速（中国大陆必做）

```bash
mkdir -p /etc/docker
cat > /etc/docker/daemon.json <<'EOF'
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me"
  ]
}
EOF
systemctl restart docker
docker pull hello-world   # 验证能拉镜像
```

> DaoCloud（`docker.m.daocloud.io`）实测速度快，放在第一位优先使用。

### 4.6 构建并启动

```bash
cd /opt/logmaster-server
docker compose up -d --build
```

首次构建需拉取 4 个基础镜像 + npm 装依赖 + go 编译，约 5~15 分钟。

### 4.7 验证服务

```bash
docker compose ps               # 两个服务应为 Up / healthy
docker compose logs app --tail 30
curl -s http://127.0.0.1:8080/api/health
```

预期 `/api/health` 返回 `{"code":0,"message":"success","data":{"status":"ok"}}`，日志出现 `LogMaster running at http://localhost:8080`。

### 4.8 放行公网端口

服务器防火墙（可选，Ubuntu 默认 ufw 关闭）：

```bash
ufw allow 22/tcp && ufw allow 8080/tcp && ufw enable
```

**阿里云安全组（必须，网页操作）**：ECS 实例 → 安全组 → 入方向规则 → 添加：协议 TCP、端口 `8080/8080`、授权对象 `0.0.0.0/0`。

### 4.9 配置飞书回调

在飞书开放平台「安全设置 → 重定向 URL」填：

```text
http://<公网IP>:8080/api/auth/callback
```

必须与 `.env` 中 `FEISHU_REDIRECT_URI` 逐字符一致。完成后浏览器访问 `http://<公网IP>:8080/` 登录验证。

## 5. 本次实际踩过的坑及修复

| 现象 | 原因 | 修复 |
|---|---|---|
| 拉 golang/node 镜像几十 KB/s | 公共镜像源限速 | 换 DaoCloud 镜像源 |
| `go mod download` 报 `dial tcp ...:443 i/o timeout` | 访问 `proxy.golang.org` 超时 | `ENV GOPROXY=https://goproxy.cn,direct` |
| `npm ci` 慢/超时 | 访问 npmjs.org 慢 | `npm ci --registry=https://registry.npmmirror.com` |
| 登录后接口全部 404 | Dockerfile 漏了 `VITE_API_BASE_URL=/api`，前端请求缺 `/api` 前缀 | 补 `ENV VITE_API_BASE_URL=/api` |
| 飞书报 `20029 重定向 URL 有误` | 回调地址用了 `localhost` 或与后台不一致 | 改为 `http://<公网IP>:8080/api/auth/callback` 并两边对齐 |
| 浏览器访问公网 IP 超时 | 阿里云安全组未放行 8080 | 安全组入方向放行 `8080/8080` |

## 6. 日常运维命令

```bash
cd /opt/logmaster-server

docker compose ps                 # 看状态
docker compose logs -f app        # 跟随后端日志
docker compose restart app        # 重启后端
docker compose up -d --build      # 代码改了重新构建上线
docker compose down               # 停止（数据卷保留）
docker compose down -v            # 停止并清空数据（慎用，删库删日志）
```

查看数据库（可选）：

```bash
docker compose exec postgres psql -U logmaster -d logmaster -c "SELECT version, applied_at FROM logmaster_api.schema_migrations ORDER BY version;"
```

## 7. 注意事项

1. **`.env` 含密钥，勿提交 git**。已确认根目录 `.gitignore` 忽略 `.env`；打包上传用的 `logmaster-src.tar.gz` 内含 `.env`，用完注意从工作区删除，避免误提交。
2. **数据库密码首次启动后勿随意改**。PostgreSQL 数据卷只在第一次初始化时按 `DB_PASSWORD` 建用户；之后改 `.env` 密码不会同步到库，会导致连接失败。
3. **磁盘是主要消耗**。单次上传上限 2GB、解压上限 8GB，且当前后端无自动清理旧日志逻辑，日志会持续占用 `logs` 卷。需定期清理或后续补充 retention 逻辑。
4. **试用 ECS 不支持备案**。试用期用 `http://IP:8080` 直连即可；若飞书不认 IP 回调或需 HTTPS，需购买域名（必要时备案）并加 nginx 做 80/443 转发。
5. **飞书 App Secret 属于敏感信息**，不要明文写入聊天记录或提交到仓库；泄露后应在飞书后台重置密钥并更新 `.env`。

## 8. 与手工部署（无 Docker）的关系

手工部署（`docs/服务器部署文档.md`）与本方案等效，差别在于：

- Docker 免去「安装 PostgreSQL、建库建用户、交叉编译、搬文件、配 systemd」等手工步骤；
- Docker 镜像把「后端二进制 + frontend/dist」在同一次构建中固化，避免版本错配；
- Docker 便于换机器、回滚（换镜像 tag）、重复部署。

生产环境如需 HTTPS 域名，在 Docker 方案前再加一层 nginx（或使用 `caddy`）即可，后端容器本身无需改动。
