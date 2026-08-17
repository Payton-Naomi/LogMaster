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
