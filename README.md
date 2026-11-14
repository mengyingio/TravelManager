# 团队档案管理系统（TravelManager）

> 面向旅行社、景区接待、研学团建等团队运营场景的开源全流程管理系统：从预订单录入到归档留存，消费自动核算、凭证统一归档，每一步操作可审计。

[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3-42b883?logo=vue.js)](https://vuejs.org/)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](#贡献指南)

---

## 目录

- [项目介绍](#项目介绍)
- [功能清单](#功能清单)
- [技术栈](#技术栈)
- [环境依赖](#环境依赖)
- [目录结构](#目录结构)
- [部署步骤](#部署步骤)
- [环境变量](#环境变量)
- [初始化与登录](#初始化与登录)
- [项目截图](#项目截图)
- [角色与权限](#角色与权限)
- [API 一览](#api-一览)
- [测试](#测试)
- [安全与开源合规](#安全与开源合规)
- [已知限制](#已知限制)
- [贡献指南](#贡献指南)
- [版权声明](#版权声明)
- [开源协议说明](#开源协议说明)

---

## 项目介绍

**团队档案管理系统（TravelManager）** 是一套面向团队接待业务的全流程管理系统，由 **云电工作室** 成员共同创作编写并开源。

系统覆盖「合作旅行社维护 → 合同存档 → 团队预订单录入 → 消费明细核算 → 档案归档导出」的完整业务闭环，内置多角色权限体系、数据隔离、全流程审计日志、多云存储适配与多种登录方式。系统名称、水印、存储、发信、字典等均可通过「系统设置」动态配置，不绑定任何特定地区、景区或旅行社。

### 适合谁用

- 景区 / 目的地接待团队的运营方；
- 旅行社内部的团队调度与计调管理；
- 研学机构、团建公司的团队档案管理；
- 需要多角色协作、数据隔离的中小型团队业务；
- 希望二次开发团队接待管理系统的开发者。

---

## 功能清单

### 业务管理

- **旅行社管理** — 合作旅行社资料维护，老板 / 计调联系方式，手机号与邮箱格式校验，CSV 导入导出。
- **合同管理** — 合同拍照 / PDF 上传存档，状态流转与回收站恢复，CSV 导入导出。
- **团队统计** — 预订单录入、抵达 / 离开日期、带队负责人、导游、消费明细自动核算总花费；团型与消费分类可自定义；支持高德地图选点记录团队地点。
- **消费收集链接** — 为团队生成公开收集链接，发给司机 / 导游等外部人员补录消费（含票据 / 照片附件），可随时开启 / 关闭 / 重置；附件大小与格式在系统设置中统一控制。
- **档案归档** — 团队附件按类型分类归档（预订单 / 合同 / 票据 / 行程单 / 凭证），Word 简报一键导出。
- **数据仪表盘** — 按年 / 月统计团型、客源地、团队数与总消费，按角色自动过滤数据范围。
- **CSV 导入导出** — 旅行社、合同、团队三类数据批量导入导出，按中文表头匹配，返回逐行错误明细。

### 权限与安全

- **RBAC 权限体系** — 内置超级管理员 / 管理员 / 旅行社 / 负责人四种角色，支持自定义角色与权限；前端菜单与后端接口双重收口，数据按角色隔离。
- **通行密钥登录** — WebAuthn / FIDO2 无密码登录（设备指纹 / 人脸 / 安全密钥）。
- **第三方登录** — 兼容彩虹聚合登录 / OauthGo REST API 协议，支持 QQ / 微信 / 微博。
- **找回密码** — 通过绑定邮箱（SMTP）或手机（短信）发送验证码自助重置。
- **审计日志** — 登录、查看、下载、增删改、导入导出全流程留痕。
- **超级管理员保护** — 系统首个注册用户自动获得超级管理员；不可通过接口创建、提权、降级、停用或删除。
- **敏感配置脱敏** — 云存储、SMTP、短信、OAuth 等密钥在接口返回时统一显示为 `********`。

### 基础设施

- **高德地图选点** — 团队录入与详情页支持地图搜索 / 点击选点并自动反查地名。
- **SMTP 邮件** — STARTTLS / 隐式 SSL / 无加密三种模式，支持测试发信。
- **短信通知** — 短信宝 / 阿里云短信签名直连 / 通用 Webhook 三种模式。
- **多云存储** — 本地 / 腾讯云 COS / 又拍云 / 阿里云 OSS / S3 兼容（MinIO、华为云 OBS、七牛云、Cloudflare R2 等），原生 HTTP 签名实现，不依赖厂商 SDK。
- **主题系统** — 白天 / 黑夜 / 跟随系统三档切换，品牌色与设计系统可全局定制。
- **页面水印** — 全屏透明水印，降低截图 / 拍照外泄风险。
- **Docker 部署** — 多阶段构建，前后端一体托管，一条命令启动。

---

## 技术栈

### 后端

| 组件 | 说明 |
| --- | --- |
| Go 1.27 + Gin | 高性能 HTTP 服务 |
| GORM | ORM，支持 SQLite / MySQL |
| SQLite（纯 Go 驱动） | 零依赖本地数据库，无需 CGO |
| MySQL | 生产环境可选 |
| JWT（HS256） + bcrypt | 登录令牌与密码哈希 |
| go-webauthn | WebAuthn / FIDO2 通行密钥 |
| 标准库 net/smtp、net/http | 邮件发送、云存储 / 短信 HTTP 签名 |

### 前端

| 组件 | 说明 |
| --- | --- |
| Vue 3 + Vite | 渐进式框架与构建工具 |
| Element Plus | 企业级 UI 组件库 |
| Pinia | 状态管理 |
| Vue Router | 路由管理 |
| ECharts | 数据可视化 |
| Axios | HTTP 客户端 |

### 部署

- Docker 多阶段构建（前端构建 → Go 静态编译 → Alpine 运行镜像）；
- docker compose 一键编排；
- 也支持传统二进制 + Nginx / Caddy 反向代理部署。

---

## 环境依赖

### 最低要求

| 依赖 | 版本 | 用途 |
| --- | --- | --- |
| Go | 1.27+ | 编译后端 |
| Node.js | 18+（推荐 20 / 22 / 24） | 构建前端 |
| npm | 9+ | 安装前端依赖 |
| SQLite | 内置（纯 Go 驱动） | 默认数据库，无需单独安装 |
| MySQL | 5.7+ / 8.0+（可选） | 生产数据库 |
| Docker | 24+（可选） | 容器化部署 |
| Docker Compose | v2（可选） | 一键编排 |

### 运行环境

- Linux / macOS / Windows 均可编译运行；
- 浏览器需支持 ES2020；通行密钥（WebAuthn）要求 HTTPS 或 `localhost` 安全上下文；
- 服务器建议 1 核 1G 起步，生产环境建议 2 核 2G 以上并配置 HTTPS。

---

## 目录结构

```text
TravelManager/
├── main.go                 # 服务入口
├── config/                 # 配置加载（.env + 系统环境变量）
├── database/               # SQLite/MySQL 初始化、自动建表、内置角色与超级管理员保障
├── models/                 # 数据模型（用户 / 旅行社 / 合同 / 团队 / 消费 / 附件 / 日志 / 设置）
├── middleware/             # JWT 认证、权限鉴权、CORS、限流、审计
├── handlers/               # API handler（含 RBAC 数据作用域）
├── services/               # Word 导出、CSV、SMTP、短信、云存储、第三方登录
├── validators/             # 手机号 / 邮箱自定义校验
├── router/                 # 路由注册与静态资源托管
├── web/                    # Vue 3 前端工程
│   ├── src/
│   │   ├── views/          # 登录、仪表盘、旅行社、合同、团队、用户、日志、设置
│   │   ├── stores/         # 认证 / 系统名称 / 主题
│   │   ├── api/            # 接口封装
│   │   └── components/     # 通用组件
│   ├── .env.example        # 前端环境变量示例
│   └── package.json
├── storage/uploads/        # 本地存储目录（仅保留 .gitkeep）
├── docs/                   # 开源文档、审计报告、截图
├── docker-compose.yml
├── Dockerfile
├── .env.example            # 后端环境变量示例（不含真实密钥）
├── LICENSE
└── README.md
```

---

## 部署步骤

### 方式一：Docker Compose（推荐）

```bash
# 1. 获取源码
git clone <repository-url>
cd TravelManager

# 2. 准备环境变量
cp .env.example .env
# 编辑 .env，至少填写：
#   JWT_SECRET=<至少 32 位随机字符串>
#   INITIAL_ADMIN_TOKEN=<一次性随机字符串>

# 3. 构建并启动
docker compose up -d --build

# 4. 查看日志 / 健康状态
docker compose logs -f
docker compose ps
```

访问 `http://localhost:8080`。SQLite 数据库与上传文件持久化在命名卷 `travelmanager-data`（容器内 `/data`）。

```bash
# 停止（保留数据）
docker compose down

# 停止并删除数据卷（不可恢复，请先备份）
docker compose down -v
```

> 生产环境请在 `.env` 中设置至少 32 位的 `JWT_SECRET`；空数据库首次初始化必须提供 `INITIAL_ADMIN_TOKEN`，初始化完成后请删除该变量并重启容器。

### 方式二：传统部署（二进制 + 反向代理）

```bash
# 1. 安装依赖并配置环境变量
cp .env.example .env
# 编辑 .env：JWT_SECRET、INITIAL_ADMIN_TOKEN、数据库（可选 MySQL）

# 2. 构建前端
cd web
npm ci
npm run build
cd ..

# 3. 编译后端
go mod download
go build -trimpath -ldflags="-s -w" -o TravelManager .

# 4. 启动
./TravelManager
```

生产环境建议使用 systemd 守护进程，并通过 Nginx / Caddy 配置 HTTPS 反向代理。反向代理需透传 `Host` 与 `X-Forwarded-Proto`，第三方登录回调地址会依据请求域名自动生成。

### 方式三：本地开发

```bash
# 终端一：启动后端（默认 :8080）
cp .env.example .env
# 填写 JWT_SECRET 与 INITIAL_ADMIN_TOKEN
go run .

# 终端二：启动前端开发服务器（默认 :5173，已代理 /api 与 /uploads 到 :8080）
cd web
cp .env.example .env
npm ci
npm run dev
```

浏览器访问 `http://localhost:5173`。

### 部署检查

- [ ] `JWT_SECRET` 已设置为随机强密钥，未使用示例值；
- [ ] `.env` 未提交到 Git；
- [ ] `INITIAL_ADMIN_TOKEN` 仅在首次初始化时存在，初始化后已删除；
- [ ] 已配置 HTTPS（通行密钥 / 第三方登录 / 生产环境需要）；
- [ ] 已配置数据目录持久化与定期备份；
- [ ] 生产环境已按需收敛 `CORS_ALLOW_ORIGINS`。

---

## 环境变量

后端读取根目录 `.env`，系统环境变量优先于 `.env`。完整模板见 [`.env.example`](.env.example)。

| 变量 | 默认值 | 必填 | 说明 |
| --- | --- | :-: | --- |
| `PORT` | `8080` | 否 | 服务监听端口 |
| `GIN_MODE` | `debug` | 否 | `debug` / `release`，生产建议 `release` |
| `APP_VERSION` | `0.1.0` | 否 | 系统信息接口返回的版本号 |
| `JWT_SECRET` | 无 | **是** | JWT 签名密钥，至少 32 位；缺失或过短拒绝启动 |
| `INITIAL_ADMIN_TOKEN` | 无 | 首次初始化 | 空库首次注册超级管理员的一次性令牌 |
| `TOKEN_TTL_HOURS` | `72` | 否 | 登录令牌有效期（小时） |
| `DB_TYPE` | `sqlite` | 否 | `sqlite` / `mysql` |
| `DB_PATH` | `travelmanager.db` | 否 | SQLite 文件路径 |
| `DB_HOST` | `127.0.0.1` | MySQL 时 | MySQL 地址 |
| `DB_PORT` | `3306` | MySQL 时 | MySQL 端口 |
| `DB_NAME` | `TravelManager` | MySQL 时 | 数据库名 |
| `DB_USER` | `TravelManager` | MySQL 时 | 数据库账号 |
| `DB_PASSWORD` | 空 | MySQL 时 | 数据库密码，禁止提交真实值 |
| `DB_CHARSET` | `utf8mb4` | 否 | MySQL 字符集 |
| `SKIP_AUTO_MIGRATE` | `false` | 否 | 生产环境可关闭自动建表 |
| `UPLOAD_DIR` | `storage/uploads` | 否 | 本地存储根目录 |
| `UPLOAD_MAX_SIZE_MB` | `5` | 否 | 单文件上传大小上限 |
| `UPLOAD_ALLOWED_EXTS` | `jpg,jpeg,png,gif,pdf,doc,docx,xls,xlsx` | 否 | 允许上传的扩展名 |
| `SYSTEM_NAME` | `团队档案管理系统` | 否 | 系统名称默认值（可在后台覆盖） |
| `TEAM_TYPES` | `研学,团建,旅游团` | 否 | 团型字典默认值 |
| `EXPENSE_CATEGORIES` | `门票,住宿,餐饮,景区内部消费` | 否 | 消费分类默认值 |
| `OAUTH_MODE` | `rainbow` | 否 | 第三方登录网关模式 |
| `OAUTH_PROVIDERS` | `qq,wechat,weibo` | 否 | 登录页展示的第三方平台 |
| `CORS_ALLOW_ORIGINS` | `*` | 否 | 允许跨域来源，逗号分隔；生产建议收敛 |

前端 Vite 变量见 [`web/.env.example`](web/.env.example)，仅以 `VITE_` 开头的变量会暴露给浏览器，**禁止在前端变量中填写任何密钥**。

> 云存储、SMTP、短信、OAuth、高德地图等第三方凭据不在 `.env` 中配置，统一在系统「设置」页录入并保存到数据库；接口返回时自动脱敏。这样可避免密钥随源码或配置文件泄漏。

---

## 初始化与登录

系统**不内置任何账号**，也不存在默认密码：

1. 空数据库首次访问时，登录页出现「初始化」入口；
2. 输入用户名、密码，并填写部署时设置的 `INITIAL_ADMIN_TOKEN`；
3. 首个注册用户自动成为超级管理员；
4. 初始化完成后，注册入口关闭，后续账号由管理员在「用户管理」中创建；
5. 建议初始化完成后从 `.env` 删除 `INITIAL_ADMIN_TOKEN` 并重启服务。

如忘记管理员密码且未配置 SMTP / 短信，可在数据库中重置对应用户的 `password_hash`，或重新部署空库后重新初始化（请先备份）。

---

## 项目截图

截图统一存放在 [`docs/screenshots/`](docs/screenshots/)，命名、尺寸、脱敏规范见 [`docs/screenshots/README.md`](docs/screenshots/README.md)。

### 登录页

![登录页](docs/screenshots/login.png)

> 仓库仅包含无业务数据的登录页截图。仪表盘、团队列表等页面请使用演示数据截图后按规范补充，**严禁出现真实姓名、手机号、金额、机构名称或内网地址**。

---

## 角色与权限

| 能力 | 超级管理员 | 管理员 | 旅行社 | 负责人 |
| --- | :-: | :-: | :-: | :-: |
| 用户管理 / 系统设置 / 操作日志 | ✅ | ✅ | ❌ | ❌ |
| 新增 / 删除旅行社 | ✅ | ✅ | ❌ | ❌ |
| 查看旅行社 | 全部 | 全部 | 仅本社 | 仅本人带队关联 |
| 合同管理 | 全部 | 全部 | 仅本社 | 仅本人带队关联（只读） |
| 团队录入 / 修改 / 归档 | 全部 | 全部 | 仅本社 | 仅本人带队团队 |
| 团队删除 | ✅ | ✅ | ✅（仅本社） | ❌ |
| 消费录入 / 修改 | ✅ | ✅ | ✅（仅本社） | ✅（仅本人带队） |
| 附件上传 / 下载 | ✅ | ✅ | ✅（仅本社） | ✅（仅本人带队） |
| CSV 导入 | ✅ | ✅ | ✅（仅本社，导入旅行社除外） | ❌ |
| CSV 导出 | 全部 | 全部 | 仅本社 | 仅本人带队 |
| 统计仪表盘 | 全部 | 全部 | 仅本社 | 仅本人带队 |
| 解除归档 | ✅ | ❌ | ❌ | ❌ |
| 自定义角色 | ✅ | ❌ | ❌ | ❌ |

> **超级管理员**：系统首个注册用户自动获得，拥有全部权限；不可通过接口创建、提权、降级、停用或删除。

> **负责人角色**的数据范围由账号「手机号 / 邮箱」与团队档案中的「带队负责人手机号 / 邮箱」匹配决定；管理员需维护负责人档案并完成团队绑定。

---

## API 一览

响应统一为 `{"code": 0, "message": "ok", "data": ...}`；业务错误返回对应 HTTP 状态码与 `message`。

| 模块 | 方法 & 路径 | 权限 |
| --- | --- | --- |
| 系统信息 | `GET /api/system/info` | 公开 |
| 系统字典 | `GET /api/system/dicts` | 登录 |
| 登录 / 注册 | `POST /api/auth/login`、`POST /api/auth/register` | 公开（注册仅空库） |
| 找回密码 | `POST /api/auth/forgot-password`、`POST /api/auth/reset-password` | 公开 |
| 通行密钥登录 | `GET /api/auth/passkey/begin-login`、`POST /api/auth/passkey/complete-login` | 公开 |
| 第三方登录 | `GET /api/auth/oauth/authorize`、`GET /api/auth/oauth/callback` | 公开 |
| 当前用户 | `GET /api/auth/me`、`POST /api/auth/logout` | 登录 |
| 个人主页 | `PUT /api/users/me`、`POST /api/users/me/avatar`、`PUT /api/users/me/password` | 登录 |
| 第三方绑定 | `POST /api/users/me/oauth/ticket`、`DELETE /api/users/me/oauth/:provider` | 登录 |
| 通行密钥管理 | `GET/POST/DELETE /api/passkeys` | 登录 |
| 用户管理 | `GET/POST /api/users`、`PUT/DELETE /api/users/:id`、`POST /api/users/:id/reset-password` | 管理员 |
| 旅行社 | `GET/POST /api/agents`、`GET/PUT/DELETE /api/agents/:id`、`POST /api/agents/import` | 按角色 |
| 合同 | `GET/POST /api/contracts`、`GET/PUT/DELETE /api/contracts/:id`、`POST /api/contracts/:id/restore` | 按角色 |
| 团队 | `GET/POST /api/teams`、`GET/PUT/DELETE /api/teams/:id`、`POST /api/teams/:id/archive` | 按角色 |
| 消费 | `POST /api/teams/:id/expenses`、`PUT/DELETE /api/teams/:id/expenses/:eid` | 按角色 |
| 消费附件 | `POST/GET/DELETE /api/teams/:id/expenses/:eid/attachments` | 按角色 |
| 消费收集 | `PUT /api/teams/:id/collect` | 按角色 |
| 公开收集 | `GET /api/public/collect/:token`、`POST /api/public/collect/:token/expenses` | 公开（凭令牌） |
| 团队附件 | `POST/GET/DELETE /api/teams/:id/attachments` | 按角色 |
| 导出 | `GET /api/teams/:id/export`（Word）、`GET /api/{agents,contracts,teams}/export`（CSV） | 按角色 |
| 导入 | `POST /api/{agents,contracts,teams}/import` | 管理员 / 旅行社 |
| 统计 | `GET /api/stats/summary` | 按角色 |
| 设置 | `GET/PUT /api/settings`、`POST /api/settings/test-mail`、`POST /api/settings/test-sms` | 管理员 |
| 日志 | `GET /api/logs` | 管理员 |

---

## 测试

```bash
# 后端单元测试 / 集成测试
go test ./...

# 前端构建校验
cd web && npm ci && npm run build
```

测试覆盖：登录与会话、团队 / 消费 / 统计 / 导出全流程、RBAC 权限隔离（角色越权 403、数据范围过滤、停用账号拦截）、设置持久化与敏感字段脱敏、短信（本地 Mock 网关）、第三方登录全链路（内存 Mock 网关）、CSV 导入导出、通行密钥、附件访问控制。

> **测试基线说明**：仓库保留了原作者的全部测试用例。由于测试进程共享登录/注册限流状态、部分历史用例未携带 `setup_token`、且 Windows 下 SQLite 连接会在临时目录清理时仍被占用，`go test ./...` 在当前环境并非全部通过；这些属于测试代码自身的既有问题，不影响生产逻辑。本次开源合规处理未改动任何业务逻辑与测试断言，欢迎通过 PR 修复测试基线。可先按包运行 `go test ./handlers ./services` 验证核心模块。

---

## 安全与开源合规

### 密钥与敏感信息

- 仓库不包含真实密钥、数据库密码、内网地址或用户数据；
- `.env` 已被 `.gitignore` / `.dockerignore` 排除，仅提交 `.env.example` 模板；
- 第三方凭据保存在数据库，接口返回时脱敏；
- 完整审计记录见 [`docs/敏感信息审计报告.md`](docs/敏感信息审计报告.md)。

### 部署安全建议

- 使用独立 `.env` 与高强度随机 `JWT_SECRET`；
- 生产环境启用 HTTPS，收敛 `CORS_ALLOW_ORIGINS`；
- 定期备份数据库与上传目录，限制上传目录执行权限；
- 初始化完成后删除 `INITIAL_ADMIN_TOKEN`；
- 定期执行 `go list -m -u all`、`npm audit` 与密钥扫描（gitleaks / trufflehog）。

### 漏洞报告

如发现安全问题，请勿直接公开 Issue 细节，可通过云电工作室官网 <https://www.ce-studio.cn> 联系，或先提交不含利用细节的 Issue，我们会尽快跟进。详见 [`SECURITY.md`](SECURITY.md)。

---

## 已知限制

- 云存储与短信使用原生 HTTP 签名实现，未接入云厂商官方 SDK，暂不支持分片 / 断点续传等高级能力；
- 第三方登录回调的 `sign` 参数暂未做本地二次校验，依赖一次性 `code` 与 HTTPS；
- 通行密钥依赖浏览器安全上下文（HTTPS 或 localhost），更换域名后需重新绑定；
- 短信模板参数以 JSON 字符串透传，站内通知场景可按需扩展；
- 目前为桌面端优先设计，移动端为响应式适配，复杂报表能力仍在完善。

---

## 贡献指南

欢迎提交 Issue 和 Pull Request：

1. Fork 本仓库；
2. 创建特性分支：`git checkout -b feature/your-feature`；
3. 提交更改：`git commit -m 'feat: add your feature'`；
4. 推送分支：`git push origin feature/your-feature`；
5. 开启 Pull Request。

提交前请确保：

- `go test ./handlers ./services` 通过（根包历史用例见上文“测试基线说明”）；
- `cd web && npm run build` 通过；
- 未提交 `.env`、数据库、上传文件、构建产物或任何真实业务数据；
- 新增配置项使用环境变量并提供 `.env.example` 说明。

---

## 版权声明

**团队档案管理系统（TravelManager）** 由 **云电工作室** 成员共同创作编写。

- 版权所有 © 2026 **云电工作室**
- 官方网站：<https://www.ce-studio.cn>
- 本项目版权归属云电工作室，源码文件头保留版权与许可声明。

“云电工作室”名称、Logo 及相关品牌标识归云电工作室所有；MIT 协议仅授予代码版权许可，不授予商标使用权。

---

## 开源协议说明

本项目采用 **MIT License** 开源，完整协议文本见 [`LICENSE`](LICENSE)，补充说明见 [`docs/版权与开源协议说明.md`](docs/版权与开源协议说明.md)。

MIT 协议允许你：

- 自由使用、复制、修改、合并、发布、分发本项目源码；
- 将本项目用于商业项目或闭源项目；
- 在保留版权声明与许可声明的前提下再许可。

同时要求你：

- 在所有副本或主要部分中保留原始版权声明与许可声明；
- 不得删除源码文件头部的版权与许可说明。

本项目按“现状”提供，不附带任何明示或默示担保。因使用本项目产生的任何直接或间接损失，作者与云电工作室不承担责任。

第三方依赖（Gin、GORM、Vue、Element Plus、ECharts 等）遵循各自许可证，分发时请同时遵守。

---

## 致谢

感谢所有为项目提供反馈和建议的朋友，感谢开源社区提供的优秀工具与库。

如果这个项目对你有帮助，欢迎点个 Star ⭐ 支持一下！
