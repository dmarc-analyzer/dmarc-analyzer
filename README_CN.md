# DMARC Analyzer

> 自托管的 DMARC 聚合报告解析、存储与可视化平台。
> 直接从邮件服务商接收 `rua=` 报告,解码并附加发件人情报,
> 在仪表盘里查看"谁在以你的域名发邮件"。

[![Apache 2.0 License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg?logo=go)](https://go.dev/)
[![Vue 3](https://img.shields.io/badge/Vue-3.5-42b883.svg?logo=vue.js)](https://vuejs.org/)
[![PostgreSQL 14+](https://img.shields.io/badge/PostgreSQL-14%2B-336791.svg?logo=postgresql)](https://www.postgresql.org/)
[![Docker Image](https://img.shields.io/badge/image-ghcr.io%2Fdmarc--analyzer%2Fdmarc--analyzer-2496ED.svg?logo=docker)](https://github.com/dmarc-analyzer/dmarc-analyzer/pkgs/container/dmarc-analyzer)

[English](README.md) · [简体中文](README_CN.md)

---

## 目录

- [项目简介](#项目简介)
- [核心功能](#核心功能)
- [整体架构](#整体架构)
- [技术栈](#技术栈)
- [快速开始(Docker Compose)](#快速开始docker-compose)
- [环境变量](#环境变量)
- [文档](#文档)
- [API 概览](#api-概览)
- [代码结构](#代码结构)
- [参与贡献](#参与贡献)
- [许可证](#许可证)

---

## 项目简介

**DMARC**(Domain-based Message Authentication, Reporting & Conformance)
允许域名持有者在 DNS 中发布一条策略,告诉收件服务器:如果有未经身份认证的
邮件冒充你的域名应该怎么处理。

各大邮件服务商(Google、Microsoft、Yahoo、Apple……)会按照 `_dmarc` TXT
记录里 `rua=` 指定的邮箱地址,定期回寄 **聚合报告(aggregate RUA report)**
—— 以 gzip / zip 压缩的 XML 附件形式发送,告诉你它们在某段时间内看到
多少封"以你域名身份"的邮件,以及 SPF / DKIM 是否通过、来源 IP 是什么。

报告本身格式规范但实际处理起来很烦:

- 各家服务商在 MIME / 压缩格式上略有差异;
- 大量原始 IP 没有上下文,人类几乎读不出有用信息;
- 数据量与时间维度高度相关,需要时序聚合才看得出趋势。

**DMARC Analyzer** 是一条端到端流水线,把这些原始附件加工成可查询、可
可视化的数据:

1. AWS SES 接收 DMARC 报告邮件,落地到 **S3 桶**。
2. S3 事件触发 **SQS 通知**,告知有新报告到达。
3. **consumer** 从 S3 取邮件,逐层拆 MIME / gzip / zip / XML;解析后对
   每条记录补充反向 DNS、组织域名、ESP 指纹、SenderBase 地理信息,
   写入 **PostgreSQL**。
4. **Go API 服务器 + Vue 3 SPA** 提供 Web 界面:按域名查看通过率趋势,
   下钻每个发件来源,定位伪造行为。

最终你得到一个完全自托管、单镜像、不把数据交给第三方 SaaS 的 DMARC
仪表盘 —— 所有报告都留在你自己的 AWS 账号里。

---

## 核心功能

### 接收 / 解析

- 通过 **AWS SES → S3 → SQS** 事件链自动接收 DMARC 聚合报告。
- 兼容现实中各种附件组合:multipart MIME、base64、gzip
  (`application/gzip`、`application/x-gzip`、`gzip/document`……)、
  zip(Google / Yahoo 两种风格)、纯 XML、`application/octet-stream`
  搭配 `.zip` / `.gz` 文件名。
- XML 解析器支持非 UTF-8 字符集自动识别。
- **幂等**:同一个 S3 对象 key(= 邮件 message ID)在 SQS 重投时不会
  被重复入库。

### 数据增强

- 对每个 source IP 做反向 DNS(PTR)查询。
- 基于 [Public Suffix List](https://publicsuffix.org/) 提取组织域名。
- 识别常见 ESP(电子邮件服务商):Google Mail、Amazon SES、MailChimp、
  Outlook、Google "unverified forwarding" 等。
- SenderBase(`*.query.senderbase.org`)TXT 查询,获取组织名、托管国家、
  城市、经纬度。
- IPv6 路径自带 Outlook / Google forwarding 启发式判断。

### 存储与 API

- 单张 PostgreSQL 表,复合主键 `(message_id, record_number)` ——
  详见 [`backend/schema.sql`](backend/schema.sql)。
- 版本化的 **OpenAPI 3 规范**:[`api/openapi.json`](api/openapi.json)。
- Go(Gin)路由 + 参数绑定通过 `openapi-generator` **代码生成**,CI
  会校验 spec 与生成文件的一致性。

### 前端

- **Vue 3 + Vite + Vuetify + Pinia + Chart.js** 单页应用,生产环境
  由 Go 服务器作为静态资源直接 serve。
- 域名总览页:展示近 30 天总量与通过率。
- 域名详情页:通过/失败时序图、按来源(ESP / 域名 / 主机 / IP)聚合
  的摘要表、单个来源的明细下钻。
- 日期范围选择器(支持快捷选项),URL 可深度链接。

### 运维

- GitHub Actions 自动构建多架构(linux/amd64 + linux/arm64)镜像并
  推送到 GHCR。
- backfill 子命令可一次性导入 S3 中的历史报告。
- consumer 支持 SIGINT / SIGTERM 优雅退出。
- AWS 配置全部走标准环境变量,在 EC2 / EKS / ECS 上可直接用实例
  角色,不必下发 access key。

---

## 整体架构

```mermaid
flowchart LR
    G[Google / Microsoft<br/>Yahoo / Apple<br/>邮件服务商]:::ext

    subgraph AWS[你的 AWS 账号]
        direction LR
        SES[AWS SES<br/>邮件接收规则]
        S3[(S3 桶<br/>原始报告邮件)]
        SQS[[SQS 队列<br/>ObjectCreated 事件]]
    end

    subgraph App[DMARC Analyzer]
        direction TB
        C[consumer<br/>cmd/consumer]
        BF[backfill<br/>cmd/backfill]
        SRV[server<br/>cmd/server<br/>Gin API + SPA]
        DB[(PostgreSQL<br/>dmarc_report_entries)]
        C -->|解析+增强| DB
        BF -->|扫 S3+解析| DB
        SRV -->|查询| DB
    end

    SB[(SenderBase TXT<br/>+ 反向 DNS<br/>+ Public Suffix List)]:::ext
    U([用户 / 浏览器]):::user

    G -- "rua= 报告邮件" --> SES
    SES -- "保存邮件" --> S3
    S3 -- "s3:ObjectCreated:*" --> SQS
    SQS -- "拉取消息" --> C
    C -- "GetObject" --> S3
    BF -- "ListObjects + GetObject" --> S3
    C -. "DNS 查询" .-> SB

    U -- "https://your.domain" --> SRV
    SRV -- "GET / (SPA)" --> U

    classDef ext fill:#fffbe6,stroke:#bfa73a,color:#5a4a00;
    classDef user fill:#e6f7ff,stroke:#3a7abf,color:#003a66;
```

若渲染器不支持 Mermaid,等价 ASCII 流程图见英文版 README。模块级别详细
说明见 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)。

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.25、[Gin](https://github.com/gin-gonic/gin)、[GORM](https://gorm.io/) |
| 数据库 | PostgreSQL 14+(使用 `inet` 与 `text[]` 列类型) |
| AWS SDK | [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2) —— SES(收件)、S3、SQS |
| 前端 | Vue 3(Composition API)、Vite 7、Vuetify 3、Pinia、Vue Router、Chart.js、date-fns |
| API 契约 | OpenAPI 3(`api/openapi.json`);Gin 路由与 TS axios 客户端均由它生成 |
| 构建 / CI | GitHub Actions(多架构 Docker 镜像、OpenAPI 漂移检查)、多阶段 Dockerfile |
| 数据增强 | `net.LookupAddr`、`net.LookupTXT`、[Public Suffix List](https://pkg.go.dev/golang.org/x/net/publicsuffix)、SenderBase(`*.query.senderbase.org`) |

---

## 快速开始(Docker Compose)

最快的方式:使用 GHCR 上的预构建镜像 + 本地 Postgres。

### 1. 前置条件

- Docker Engine 24+,带 `docker compose`(或 `docker-compose`)。
- 已经配置好 **SES、S3、SQS** 的 AWS 账号(一次性,详见
  [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md))。
- 一个域名,其 `_dmarc` TXT 记录中 `rua=mailto:` 指向 SES 能接收的邮箱。

### 2. 创建 `.env`

```env
# AWS 凭证(在 EC2/ECS/EKS 上可省略,使用实例角色)
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=us-east-1

# DMARC 报告接收
S3_BUCKET_NAME=your-org-dmarc-reports
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789012/your-org-dmarc-reports
```

### 3. `docker-compose.yml`

下面这份是推荐写法(仓库中默认的 `docker-compose.yml` 没有包含
consumer,推荐用以下版本):

```yaml
services:
  server:
    image: ghcr.io/dmarc-analyzer/dmarc-analyzer:latest
    command: ["./server"]
    ports:
      - "6767:6767"
    environment:
      DATABASE_URL: postgresql://postgres:postgres@postgres:5432/dmarc_analyzer?sslmode=disable
    depends_on:
      postgres:
        condition: service_healthy

  consumer:
    image: ghcr.io/dmarc-analyzer/dmarc-analyzer:latest
    command: ["./consumer"]
    env_file: .env
    environment:
      DATABASE_URL: postgresql://postgres:postgres@postgres:5432/dmarc_analyzer?sslmode=disable
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:14
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: dmarc_analyzer
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./backend/schema.sql:/docker-entrypoint-initdb.d/schema.sql:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 10

volumes:
  postgres-data:
```

### 4. 启动

```sh
docker compose up -d
docker compose logs -f consumer   # 跟踪入库日志
# 浏览器打开 http://localhost:6767
```

### 5. 回填历史报告(可选)

S3 桶里如果已经有历史报告,可以一次性导入:

```sh
docker compose run --rm \
  -e DATABASE_URL=postgresql://postgres:postgres@postgres:5432/dmarc_analyzer?sslmode=disable \
  --env-file .env \
  server ./backfill
```

新手完整部署流程(IAM、SES、S3 事件、DNS……)请看
**[`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)**。

---

## 环境变量

| 变量 | 使用方 | 必填 | 说明 |
|------|--------|------|------|
| `DATABASE_URL` | `server`、`consumer`、`backfill` | 是 | GORM 用的 PostgreSQL DSN,例如:`postgresql://user:pass@host:5432/dmarc_analyzer?sslmode=disable` |
| `S3_BUCKET_NAME` | `consumer`、`backfill` | 是 | SES 写入 DMARC 邮件的 S3 桶名 |
| `SQS_QUEUE_URL` | `consumer` | 是(实时接收时) | 订阅了 `s3:ObjectCreated:*` 的 SQS 队列 URL |
| `AWS_REGION` | 所有 AWS 调用 | 是 | AWS 区域 |
| `AWS_ACCESS_KEY_ID` | 所有 AWS 调用 | 可选 | 使用实例角色时可省略 |
| `AWS_SECRET_ACCESS_KEY` | 所有 AWS 调用 | 可选 | 同上 |
| `AWS_SESSION_TOKEN` | 所有 AWS 调用 | 可选 | 临时凭证场景使用 |

> ⚠️ 早期 README 里出现过 `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` /
> `DB_NAME` / `DB_SSLMODE`,但**当前代码只读 `DATABASE_URL`**(详见
> [`backend/db/db.go`](backend/db/db.go))。请使用 DSN 形式。

---

## 文档

详细文档统一收纳在 [`docs/`](docs/) 目录下:

| 文档 | 目标读者 | 内容 |
|------|----------|------|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 工程师 | 组件图、各 Go 包职责、数据流、数据库 schema、增强流水线、OpenAPI 代码生成 |
| [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) | 新运维 | AWS 端到端配置(SES / S3 / SQS / IAM)、DNS、环境变量、Docker Compose、k8s、安全加固 |
| [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) | 贡献者 | 本地开发环境、测试、OpenAPI 重新生成、schema 重新生成、提交规范、代码风格 |
| [`docs/API.md`](docs/API.md) | API 调用方 | 四个端点的参数、响应、`curl` 示例 |
| [`docs/DMARC_PRIMER.md`](docs/DMARC_PRIMER.md) | 入门者 | DMARC / SPF / DKIM 是什么,为什么有聚合报告,怎么读 |
| [`docs/TROUBLESHOOTING.md`](docs/TROUBLESHOOTING.md) | 运维 | 常见问题(没数据、SQS 不消费、解析失败)与排查方法 |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | 贡献者 | PR 清单、提交格式、开发环境入口 |
| [`AGENTS.md`](AGENTS.md) | 仓库 agent | 自动化 / AI 贡献者约定 |

---

## API 概览

服务器在 `/api` 下提供四个只读 JSON 端点。所有日期参数支持
`YYYY-MM-DD`(推荐)或 RFC3339Nano。

| 方法 | 路径 | 用途 |
|------|------|------|
| `GET` | `/api/domains` | 列出所有域名及近 30 天总量 / 通过量 |
| `GET` | `/api/domains/{domain}/report?start=&end=` | 按来源(ESP / 域名 / 主机 / IP)聚合的摘要 |
| `GET` | `/api/domains/{domain}/report/detail?start=&end=&source=&source_type=` | 单个来源的明细行,用于下钻 |
| `GET` | `/api/domains/{domain}/chart/dmarc?start=&end=` | 趋势图所需的每日 pass/fail 时序数据 |

示例:

```sh
curl 'http://127.0.0.1:6767/api/domains'
curl 'http://127.0.0.1:6767/api/domains/example.com/report?start=2026-04-20&end=2026-05-20'
```

完整 schema、错误语义与各端点的 `curl` 示例见
**[`docs/API.md`](docs/API.md)**。

机器可读规范:**[`api/openapi.json`](api/openapi.json)** —— 可直接导入
Postman / Insomnia,或用 `openapi-generator` 生成任意语言的客户端。

---

## 代码结构

```
.
├── api/
│   ├── openapi.json                  # OpenAPI 3 规范(单一来源)
│   └── openapi-generator/            # 生成器模板覆盖
├── backend/
│   ├── cmd/
│   │   ├── server/server.go          # HTTP API + 静态 SPA
│   │   ├── consumer/consumer.go      # SQS 驱动的入库
│   │   ├── backfill/backfill.go      # 一次性历史回填
│   │   └── generate_sql.go           # 用 GORM AutoMigrate 导出 schema.sql
│   ├── handler/                      # API handler + 生成的路由
│   ├── model/                        # GORM 模型 + XML 模型 + 自定义类型
│   ├── messageprocessor/             # SQS 拉取循环 + 去重
│   ├── s3client/, sqsclient/         # AWS 客户端初始化
│   ├── senderbase/                   # IP 地理 / ESP 指纹
│   ├── util/                         # publicsuffix、日期解析
│   ├── process.go                    # MIME / gzip / zip / XML 解码
│   └── schema.sql                    # 自动生成的 Postgres schema
├── frontend/                         # Vue 3 SPA(Vite + Vuetify + Pinia)
│   └── src/
│       ├── views/                    # DomainsView, ReportView
│       ├── components/               # LineChart, DateRange, DetailDialog
│       ├── stores/                   # Pinia store
│       └── services/openapi/         # 生成的 typescript-axios 客户端
├── scripts/gen-routes.sh             # 重新生成 routes.gen.go + handlers.gen.go
├── .github/workflows/                # docker-build.yml, openapi-routes.yml
├── docker-compose.yml                # 本地开发便捷启动
├── Dockerfile                        # 多阶段构建:node → go → alpine
├── Makefile                          # 构建 / 运行 / Docker 目标
└── go.mod
```

---

## 参与贡献

欢迎 PR、Issue 与讨论。先看:

1. **[`CONTRIBUTING.md`](CONTRIBUTING.md)** —— 简版指引。
2. **[`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md)** —— 完整本地开发流程
   (Postgres、Go 测试、OpenAPI 重新生成、schema 重新生成、前端客户端
   重新生成)。
3. 在 issue 列表里看是否已有相同议题;较大改动建议先
   [新建 issue](https://github.com/dmarc-analyzer/dmarc-analyzer/issues/new)
   讨论方案。

速查规则:

- push 之前跑 `go test ./...` 和 `cd frontend && yarn build`。
- commit message:简短的祈使句,自然时可用 `fix:` / `feat:` / `docs:` /
  `ci:` 这类前缀。尽量加 `-s`(sign-off)与 `-S`(GPG 签名)。
- 改动 API 时同步更新 [`api/openapi.json`](api/openapi.json),并执行
  `./scripts/gen-routes.sh` 保证生成的 Go 文件同步。

---

## 许可证

本项目使用 Apache License 2.0 —— 详见 [`LICENSE`](LICENSE)。
