# ============================================================
# TravelManager 多阶段构建
# 阶段 1：构建前端（Vue 3 + Vite）
# 阶段 2：编译后端（Go 纯静态二进制，SQLite 无 CGO 依赖）
# 阶段 3：Alpine 运行镜像（内置前端产物，由后端托管 SPA）
# ============================================================

# ---------- 阶段 1：前端构建 ----------
FROM node:24-alpine AS web-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# ---------- 阶段 2：Go 编译 ----------
FROM golang:1.27-alpine AS go-builder
WORKDIR /app
# 先复制依赖清单以利用构建缓存
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/travelmanager .

# ---------- 阶段 3：运行 ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai \
    PORT=8080 \
    GIN_MODE=release \
    DB_PATH=/data/travelmanager.db \
    UPLOAD_DIR=/data/uploads
WORKDIR /app
# 后端二进制 + 前端构建产物（后端检测到 web/dist 后自动托管 SPA）
COPY --from=go-builder /out/travelmanager .
COPY --from=web-builder /app/web/dist ./web/dist
RUN mkdir -p /data/uploads
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/travelmanager"]
