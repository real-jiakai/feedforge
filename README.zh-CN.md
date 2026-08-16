[English](README.md) | 简体中文

# FeedForge

**把任何网页变成 RSS 订阅源** —— 开源自部署的 [Feed43](https://en.wikipedia.org/wiki/Feed43) 继任者。

指向一个网页，用 `{%}` / `{*}` 模式描述要提取的内容，就能得到一个稳定的 RSS 地址。一个 Go 单二进制 + JSON 文件存储 —— 不用注册，不需要数据库。

## 功能特性

- **兼容 Feed43 的模式语法** —— `{%}` 捕获、`{*}` 跳过，`{%1}…{%n}` 输出模板。
- **三步向导**，实时预览匹配结果。中文 / English 界面。
- **内置配方** —— [Bytes.dev](https://bytes.dev/archives) 和 [OSSInsight](https://ossinsight.io/blog)，都有针对页面副本的测试保护。
- **RSS 2.0 与 JSON Feed 1.1** 双格式输出，条目日期取首次出现时间、保持稳定。
- **编码友好** —— 自动检测网页编码，也可按订阅源强制 GBK/Big5/Shift_JIS 等。
- **多用户** —— 每个账户管理自己的订阅源。第一个账户是管理员，由它决定是否开放注册（默认关闭）。
- **默认安全** —— SSRF 防护、大小和时间上限、输出严格转义、带 CSRF 防护的会话登录。
- **单个静态二进制**，Docker 一条命令部署，自带 `/demo` 练习页。

## 快速开始

```bash
git clone https://github.com/real-jiakai/feedforge.git
cd feedforge
docker compose up -d        # → http://localhost:8080
```

源码运行：

```bash
go build -o feedforge .
./feedforge -addr :8080 -data ./data
```

第一次访问会引导你创建**管理员账户**；全新实例会自动为它建好两个配方订阅源（Bytes.dev、OSSInsight）。所有数据都在 `./data` 目录 —— 备份它就是备份一切（`tar czf backup.tar.gz data/`）。

## 一分钟学会写模式

模式作用在网页的 HTML 源码上。`{%}` 匹配并捕获任意文本；`{*}` 匹配并跳过。

1. 可选的**全局模式**先应用一次、圈定搜索范围，如 `<ul class="news">{%}</ul>`。
2. **条目模式**在该范围内反复匹配，每个匹配就是一条订阅内容。

页面源码：

```html
<li><a href="/post/42">Big news</a><span>2026-07-31</span></li>
```

条目模式：

```
<li><a href="{%}">{%}</a><span>{%}</span></li>
```

条目模板：标题 `{%2}`、链接 `{%1}`、内容 `{%3}`。相对链接会自动补全。

技巧：

- 宏是惰性的 —— 遇到后面的文字就停；只有结尾的 `{%}` 是贪婪的。
- **智能空白**（默认开启）让任意空白互相匹配，网页重排版也不怕。
- 不要匹配冗长的 Tailwind 类名 —— 跳过它们：`<h2 class="{*}>{%}</h2>`。
- 小心结构不同的置顶条目；模式写宽松些让两种都能匹配（参考 Bytes.dev 配方）。
- 页面按从旧到新排列时，勾选*倒序排列*，新条目才不会被挤掉。

## 内置配方

| 配方 | 来源 | 演示了什么 |
|---|---|---|
| OSSInsight 博客 | ossinsight.io/blog | 跳过易变的 Tailwind 类名 |
| Bytes.dev 归档 | bytes.dev/archives | 置顶条目结构与列表不同时怎么办 |

这正是本实例要提供的两个订阅源。两个配方都会针对保存下来的页面副本运行测试（`internal/server/testdata/`）—— 网站结构一旦变化，是测试失败，而不是订阅源悄悄变空。

## HTTP API

| 方法与路径 | 说明 |
|---|---|
| `POST /api/auth/register`、`…/login`、`…/logout` | 账户与会话（注册：史上第一个用户 = 管理员，之后仅在开放注册时可用） |
| `GET /api/auth/me` | 当前用户 |
| `GET`、`PUT /api/admin/settings` | 管理员：切换 `registrationEnabled` |
| `GET /api/recipes` | 列出内置配方 |
| `GET /api/feeds`、`POST /api/feeds` | 列出 / 创建自己的订阅源 |
| `GET`、`PUT`、`DELETE /api/feeds/{id}` | 读取 / 更新 / 删除自己的订阅源 |
| `POST /api/feeds/{id}/refresh` | 立即强制重新抓取 |
| `POST /api/preview` | 对页面试运行模式 |
| `GET /feeds/{id}.xml`、`GET /feeds/{id}.json` | RSS 2.0 / JSON Feed 输出 |
| `GET /demo`、`GET /healthz` | 练习页、健康检查 |

除 `config`、`recipes`、`auth` 外，`/api` 下的所有接口都需要登录会话（HttpOnly Cookie）；订阅源只有属主可见。写操作必须带 `Content-Type: application/json`。订阅源输出始终公开 —— 阅读器不需要登录。

## 配置

| 环境变量 | 参数 | 默认值 | 含义 |
|---|---|---|---|
| `FEEDFORGE_ADDR` | `-addr` | `:8080` | 监听地址 |
| `FEEDFORGE_DATA` | `-data` | `./data`（Docker 内 `/data`） | 数据目录 |
| `FEEDFORGE_BASE_URL` | `-base-url` | *（从请求推导）* | 生成订阅地址所用的公开域名 |
| `FEEDFORGE_ALLOW_PRIVATE` | `-allow-private` | `false` | 允许抓取内网地址 |
| `FEEDFORGE_MAX_FETCH_MB` | `-max-fetch-mb` | `5` | 源页面大小上限 |

每个订阅源可单独设置：最多条数（1–500）、刷新间隔、倒序、编码。仅 Compose 使用的设置（`FEEDFORGE_BIND`、`FEEDFORGE_PORT`、`FEEDFORGE_DOMAIN`、`ACME_EMAIL`）见 [`.env.example`](.env.example)。

## 部署

- **首次启动后立刻创建管理员账户** —— 在它存在之前，谁先访问谁就能注册成管理员。注意 Docker 发布的端口会绕过 ufw/firewalld。
- **自动 HTTPS**：在 `.env` 里设置 `FEEDFORGE_DOMAIN`，然后 `docker compose -f docker-compose.yml -f docker-compose.caddy.yml up -d`，Caddy 会自动签发和续期证书。
- **已有反向代理**：设置 `FEEDFORGE_BIND=127.0.0.1`，并把 `FEEDFORGE_BASE_URL` 设为公开地址。
- **升级**：`git pull && docker compose up -d --build`。**备份**：`./data` 目录。

## 安全说明

- 对回环、RFC1918、链路本地（云元数据）、CGNAT 等非公网地址的抓取在**建立连接时**就被拒绝，重定向和 DNS 重绑定也逃不掉。代理环境变量仅在 `FEEDFORGE_ALLOW_PRIVATE=true` 时生效。
- 密码以 bcrypt 哈希存储；会话是 HttpOnly、`SameSite=Lax` 的 Cookie，令牌只存哈希；写操作的 JSON Content-Type 要求可拦截跨站表单 CSRF。
- 抓取内容在 XML/JSON 输出中严格转义；非 `http`/`https` 的条目链接（如 `javascript:`）会被丢弃。

## 架构

```
main.go                  参数/环境变量、内嵌 Web UI、优雅退出
internal/pattern/        {%}/{*} → 正则编译器、模板渲染
internal/fetch/          加固的 HTTP 客户端（SSRF 防护、限额、编码）
internal/store/          JSON 文件存储 + 条目首见时间
internal/feed/           RSS 2.0 / JSON Feed 1.1 渲染
internal/server/         REST API、预览、TTL 缓存、演示页
web/                     向导 UI —— TypeScript 源码在 web/src，编译为普通 app.js
```

前端是单个 TypeScript 文件（`web/src/app.ts`），编译成普通的 `web/app.js`（无模块、无打包器）。编译产物已提交，因此构建 Go 二进制或 Docker 镜像都不需要 Node 工具链 —— 只有修改前端时才需要 `npm install && npm run build`。

## 许可证

MIT。
