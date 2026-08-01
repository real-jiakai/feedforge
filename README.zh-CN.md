[English](README.md) | 简体中文

# FeedForge

**把任何网页变成 RSS 订阅源** —— 向经典服务 [Feed43](https://en.wikipedia.org/wiki/Feed43) 致敬的开源自部署复刻。

Feed43 曾经让你指向任意网页、用简单的 `{%}` / `{*}` 搜索模式描述要提取的内容，
然后得到一个稳定的 RSS 地址。它随着域名过期而消失了；FeedForge 用一个 Go
单二进制把同样的工作流带回来 —— 不用注册、没有付费墙，限制由你自己设定。

## 功能特性

- **兼容 Feed43 的模式语法** —— `{%}` 捕获、`{*}` 跳过，全局 + 条目搜索模式，
  `{%1}…{%n}` 输出模板。
- **三步交互式向导** —— 加载页面、实时查看模式匹配结果、保存前预览订阅源。
  界面支持中文 / English。
- **内置配方** —— 为真实网站写好并验证过的模式
  （[OSSInsight](https://ossinsight.io/blog)、[Bytes.dev](https://bytes.dev/archives)、
  Hacker News），一键填满整个向导，方便阅读和改造。每个配方都有测试保护。
- **RSS 2.0 与 JSON Feed 1.1** 双格式输出，地址稳定。
- **稳定的条目日期** —— FeedForge 记住每个条目第一次出现的时间并作为
  `pubDate`（Feed43 从来不给条目日期）。
- **编码友好** —— 自动检测网页编码；也可按订阅源强制指定
  GBK / GB18030 / Big5 / Shift_JIS 等。
- **默认安全** —— 阻止访问内网地址（SSRF 防护）、抓取有大小和时间上限、
  XML 输出严格转义、非 http(s) 链接会被丢弃、可选 API 令牌保护编辑操作。
- **单个静态二进制**，JSON 文件存储，Docker 一条命令部署。
- **内置演示页** `/demo` 用来练习写模式 —— 每天出现一篇新"文章"，
  练习用的订阅源会真的更新，适合课堂演示。

## 快速开始

### Docker Compose（推荐）

```bash
git clone https://github.com/real-jiakai/feedforge.git
cd feedforge
docker compose up -d
```

打开 <http://localhost:8080>，选一个配方，一分钟内就能得到一个可用的订阅源。
订阅源定义保存在 `./data` 目录。

安装到此为止 —— 令牌、HTTPS、升级和备份见
[Docker Compose 用法](#docker-compose-用法)。

### 源码运行

```bash
go build -o feedforge .
./feedforge -addr :8080 -data ./data
```

## 模式怎么写

所有匹配都作用在网页的 **HTML 源码** 上（右键查看源代码看到的内容，
不是渲染后的页面）。只有两个宏：

| 宏 | 含义 |
|----|------|
| `{%}` | 匹配任意文本并**捕获** |
| `{*}` | 匹配任意文本并**跳过** |

1. **全局搜索模式**（可选）只应用一次，用来圈定搜索范围 ——
   例如 `<ul class="news">{%}</ul>` 把后续搜索限制在这个列表内部。
   它的捕获结果可以在订阅源属性模板里用 `{%1}`、`{%2}`…引用。
2. **条目搜索模式**会在该范围内反复匹配；每个匹配就是一条内容，
   捕获结果填入条目模板。

举例。网页源码：

```html
<ul class="news">
  <li><a href="/post/42">大新闻</a><span>2026-07-31</span></li>
  <li><a href="/post/41">旧新闻</a><span>2026-07-30</span></li>
</ul>
```

条目搜索模式：

```
<li><a href="{%}">{%}</a><span>{%}</span></li>
```

条目模板：标题 `{%2}`、链接 `{%1}`、内容 `{%3}` → 一个两条内容的订阅源。
像 `/post/42` 这样的相对链接会自动补全成绝对地址。

来自多年 Feed43 使用经验的提示：

- 宏是**懒惰匹配**的：`{%}` 会在它后面的文字第一次出现处停下。
  唯一的例外是位于模式**末尾的 `{%}`**，它是贪婪的 —— 正因如此，
  单独一个 `{%}` 作为条目模式可以把整个全局捕获变成一条内容。
  末尾的 `{*}` 仍然是懒惰的。
- **智能空白**（默认开启，API 省略该字段时同样默认开启）让模式里的任意空白
  匹配页面上的任意空白，HTML 重新排版或 CRLF/LF 差异都不会弄坏订阅源。
  需要逐字节精确匹配时可关闭。
- 标题会自动清理：去掉 HTML 标签、解码实体、合并空白。条目**内容**保留 HTML。
- 页面上如果有多处相似的列表结构，请用独特的锚点（class 名、外层标签）
  来固定条目模式，不要只写一个普通的 `<li>`。
- **当心结构不同的置顶条目。** 很多博客会把最新一篇渲染成和列表里完全不同的
  「精选」区块。只照着列表写的模式会悄悄漏掉每一篇新文章，直到它滚进列表为止。
  内置的 Bytes.dev 配方演示了解法：把会变的部分**跳过**
  （`<span class="font-{*}>` 同时匹配 `font-bold` 和 `font-semibold`），
  而不是精确匹配它。
- 如果页面是**旧的在前**，请打开「倒序排列」。FeedForge 会保留最后 N 个匹配，
  保证新追加的条目一定进得了订阅源。

### 应对 Tailwind 类名很长的页面

现代网站的 class 属性又长又容易变。不要去匹配它们，直接跳过。与其写

```
<h2 class="mt-2.5 line-clamp-2 text-[17px] font-semibold ...">{%}</h2>
```

不如写

```
<h2 class="{*}>{%}</h2>
```

这样即使网站改了样式也不会失效，而且能匹配标题的所有变体。

## 内置配方

FeedForge 自带四个现成的定义。在首页点选后会填满整个向导 ——
它们是给你读、给你改的，不是黑盒。

| 配方 | 来源 | 演示了什么 |
|---|---|---|
| FeedForge 演示页 | 内置 `/demo` | 最标准清晰的写法 |
| Hacker News 首页 | news.ycombinator.com | 一行模式就够了 |
| OSSInsight 博客 | ossinsight.io/blog | 跳过易变的 Tailwind 类名 |
| Bytes.dev 归档 | bytes.dev/archives | 置顶条目结构与列表不同时怎么办 |

OSSInsight 和 Bytes.dev 两个配方在测试中会针对保存下来的页面副本运行
（`internal/server/testdata/`）。网站结构一旦变化，是测试失败，
而不是订阅源悄悄变空。

Bytes.dev 的条目模式如下：

```
href="/archives/{%}">{*}<div class="grid gap-2"><span class="font-{*}>{%}</span><h3{*}Issue <!-- -->{%}</span><{*}>{%}</
```

`{%1}` 是期号（来自链接），`{%2}` 是日期，`{%3}` 是正文里印出来的期号，
`{%4}` 是标题。注意 `<span class="font-{*}>` 和 `<{*}>` 这两处跳过 ——
正是它们让同一个模式既能匹配最新一期（`font-bold`、标题在 `<p>` 里），
也能匹配所有归档条目（`font-semibold`、标题在 `<div>` 里）。

## HTTP API

| 方法与路径 | 说明 |
|---|---|
| `GET /api/recipes` | 列出内置配方 |
| `GET /api/feeds` | 列出订阅源 |
| `POST /api/feeds` | 创建订阅源 |
| `GET /api/feeds/{id}` | 读取单个定义 |
| `PUT /api/feeds/{id}` | 更新 |
| `DELETE /api/feeds/{id}` | 删除 |
| `POST /api/feeds/{id}/refresh` | 立即强制刷新 |
| `POST /api/preview` | 对页面试运行模式（向导用） |
| `GET /feeds/{id}.xml` | RSS 2.0 输出 |
| `GET /feeds/{id}.json` | JSON Feed 1.1 输出 |
| `GET /demo` | 练习页面 |
| `GET /healthz` | 健康检查 |

设置了 `FEEDFORGE_TOKEN` 后，写操作（`POST`/`PUT`/`DELETE`）需要
`Authorization: Bearer <token>` 请求头；读取和订阅源输出保持公开。
写操作还必须带 `Content-Type: application/json` —— 正是这一条挡住了
跨站 HTML 表单对未设令牌实例的操控。

## 配置

| 环境变量 | 命令行参数 | 默认值 | 含义 |
|---|---|---|---|
| `FEEDFORGE_ADDR` | `-addr` | `:8080` | 监听地址 |
| `FEEDFORGE_DATA` | `-data` | `./data`（Docker 中为 `/data`） | 数据目录 |
| `FEEDFORGE_TOKEN` | `-token` | *（空 = 不设防）* | 编辑操作的 API 令牌 |
| `FEEDFORGE_BASE_URL` | `-base-url` | *（按请求推断）* | 生成订阅源地址时使用的公开域名 |
| `FEEDFORGE_ALLOW_PRIVATE` | `-allow-private` | `false` | 允许抓取内网地址 |
| `FEEDFORGE_MAX_FETCH_MB` | `-max-fetch-mb` | `5` | 源页面大小上限 |

每个订阅源可单独设置：最多条数（1–500，默认 25）、刷新间隔（按需抓取，
每个间隔内最多抓一次）、倒序排列、编码覆盖。

放在反向代理后面时请设置 `FEEDFORGE_BASE_URL`：否则生成的订阅源里
`atom:link rel="self"` 会根据请求的 Host 头推断。

## Docker Compose 用法

`docker compose up -d` 会在构建阶段里编译出 Go 二进制并启动一个容器 ——
宿主机不需要 Go 工具链，也不需要数据库或反向代理。订阅源是 `./data`
下的 JSON 文件，要备份的就只有这个目录。

```bash
git clone https://github.com/real-jiakai/feedforge.git
cd feedforge
cp .env.example .env      # 可选；第一次启动前先改好
docker compose up -d
```

仓库自带的 `docker-compose.yml` 已经包含：

- 开机自启、崩溃自动重启（`restart: unless-stopped`），并让 Docker 探测
  `/healthz`，所以 `docker compose ps` 显示的健康状态是真实的；
- 以非 root 用户运行、根文件系统只读、丢弃所有 Linux capability，
  只保留入口脚本接管新建 `./data` 和降权所需的
  `CHOWN`/`SETUID`/`SETGID`；
- 日志轮转（3 × 10 MB），避免某个源刷屏把磁盘写满；
- 15 秒的停止宽限期，足够正在生成的订阅源收尾。

### 用 `.env` 配置

Compose 会自动读取 `.env`，所有键都是可选的。复制 `.env.example`
（里面每一项都有注释），改完后 `docker compose up -d` 即可生效。

| 变量 | 默认值 | 作用 |
|---|---|---|
| `FEEDFORGE_TOKEN` | *（空）* | 创建/编辑订阅源需要 `Authorization: Bearer …` |
| `FEEDFORGE_BIND` | `0.0.0.0` | 端口发布到宿主机的哪个网卡 |
| `FEEDFORGE_PORT` | `8080` | 宿主机端口（容器内始终是 8080） |
| `FEEDFORGE_BASE_URL` | *（按请求推断）* | 写进订阅源地址的公开域名 |
| `FEEDFORGE_ALLOW_PRIVATE` | `false` | 允许抓取内网/回环地址 |
| `FEEDFORGE_MAX_FETCH_MB` | `5` | 源页面大小上限 |
| `FEEDFORGE_DOMAIN` | — | 下面 HTTPS 叠加文件使用的域名 |
| `ACME_EMAIL` | *（空）* | 该叠加文件里证书到期提醒的邮箱 |

其中 `FEEDFORGE_BIND`、`FEEDFORGE_PORT`、`FEEDFORGE_DOMAIN`、`ACME_EMAIL`
只对 Compose 生效，其余对应[配置](#配置)一节里的命令行参数。

> **要放到公网？** 请务必设置 `FEEDFORGE_TOKEN`。否则任何能访问端口的人
> 都能创建订阅源，借你的服务器抓取任意网址。另外要注意：Docker 自己写的
> iptables 规则排在 ufw/firewalld 之前，发布在 `0.0.0.0` 的端口即使防火墙
> 里没有放行规则也照样能从公网访问。前面有反向代理时，请设置
> `FEEDFORGE_BIND=127.0.0.1`。

### 常用命令

```bash
docker compose ps                 # 状态 + 健康检查
docker compose logs -f            # 跟踪日志
docker compose up -d              # 让改过的 .env 生效（会重建容器）
docker compose restart            # 原地重启 —— 不会重新读取 .env
docker compose stop               # 停止但保留容器
docker compose down               # 停止并删除容器；./data 不受影响
docker compose up -d --build      # git pull 之后重新构建
```

`restart` 只是把现有容器停掉再启动。环境变量和端口映射是在容器**创建**时
就固定下来的，所以刚写进 `.env` 的令牌必须等 `docker compose up -d`
重建容器之后才会生效。

升级就是 `git pull && docker compose up -d --build`。订阅源定义和条目
首次出现时间都在 `./data` 里，重新构建不会动它们。

备份与恢复整个服务：

```bash
# 备份 —— 服务运行中也可以直接执行。
tar czf feedforge-$(date +%F).tar.gz data/

# 恢复。删掉仅存的另一份数据之前，先确认压缩包是完好的。
BACKUP=feedforge-2026-07-31.tar.gz
tar tzf "$BACKUP" >/dev/null
docker compose down
sudo rm -rf data/
sudo tar xzf "$BACKUP"
docker compose up -d
```

需要 `sudo` 是因为入口脚本把 `./data` 的属主改成了容器里的 `feedforge`
用户（uid 100）；如果你本来就以 root 身份操作 Docker，可以去掉。以 root
解压还能保留原有属主，容器启动时就不必再 chown 一次。

### 用自己的域名启用 HTTPS

`docker-compose.caddy.yml` 会额外起一个 [Caddy](https://caddyserver.com)
前端，自动申请并续期证书：

```bash
echo "FEEDFORGE_DOMAIN=feeds.example.com" >> .env
echo "ACME_EMAIL=you@example.com"         >> .env   # 可选
docker compose -f docker-compose.yml -f docker-compose.caddy.yml up -d
```

叠加之后 FeedForge 自己的端口不再发布到宿主机，只能经 Caddy 访问；同时
`FEEDFORGE_BASE_URL` 默认取 `https://$FEEDFORGE_DOMAIN`，保证生成的
订阅源地址正确 —— 所以 `.env` 里的 `FEEDFORGE_BASE_URL` 留空即可。只有在
把 `CADDY_HTTPS_PORT` 改成非 443 时，才需要显式写出**带端口**的地址：
显式设置的值优先级更高。

域名的 DNS 记录必须已经指向本机，且 80、443 端口空闲，否则签发会失败。
证书保存在 `caddy_data` 卷里，升级时不要删除，避免反复签发触发
Let's Encrypt 的频率限制。之后每条命令都要带上两个 `-f`，建议在 shell 或
`.env` 里设置 `COMPOSE_FILE=docker-compose.yml:docker-compose.caddy.yml`。

### 接入已有的反向代理

只发布到回环地址，并告诉 FeedForge 它的公开域名：

```dotenv
FEEDFORGE_BIND=127.0.0.1
FEEDFORGE_BASE_URL=https://feeds.example.com
```

```caddy
feeds.example.com {
	reverse_proxy 127.0.0.1:8080
}
```

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host              $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

`X-Forwarded-Proto` 只在不设 `FEEDFORGE_BASE_URL` 时才重要 —— 有了它，
经 TLS 提供的订阅源才不会对外写成 `http://` 开头的地址。

### 排查

| 现象 | 原因与处理 |
|---|---|
| `bind: address already in use` | 宿主机端口被占用 —— 在 `.env` 里改 `FEEDFORGE_PORT` |
| `warning: could not chown /data` | `./data` 属于别的 UID；执行 `sudo chown -R 100:101 data/`，或删掉让入口脚本重建 |
| 状态一直是 `health: starting` | 首次检查在 5 秒后才跑；`docker compose logs` 里有真正的报错 |
| 订阅源内容不更新 | 每个源在自己的 TTL 内最多抓一次 —— `POST /api/feeds/{id}/refresh` 可强制刷新 |
| 订阅源地址指向 `localhost` | 设置 `FEEDFORGE_BASE_URL`（或改用 Caddy 叠加文件） |
| API 返回 `401` | 设置了 `FEEDFORGE_TOKEN`，请带上 `Authorization: Bearer <token>` |

## 代码结构（也是教学导览）

```
main.go                  参数/环境变量、内嵌 Web UI、优雅停机
internal/pattern/        {%}/{*} → 正则编译器、模板渲染
internal/fetch/          加固的 HTTP 客户端（SSRF 防护、限额、编码）
internal/store/          JSON 文件持久化 + 条目首次出现时间记录
internal/feed/           RSS 2.0 / JSON Feed 1.1 渲染
internal/server/         REST API、预览、TTL 缓存、演示页
web/                     原生 JS 向导（内嵌，无构建步骤）
```

存储就是数据目录下的普通 JSON 文件 —— 备份、对比、排查都一目了然，
不需要数据库。

## 安全说明

- 抓取器在**建立连接时**拒绝回环、RFC1918 内网、运营商级 NAT / Tailscale
  （100.64/10）、链路本地（含云厂商元数据 169.254.169.254）等非公网地址，
  因此重定向和 DNS 重绑定同样被覆盖。内嵌 IPv4 的各种 IPv6 形式
  （IPv4-mapped、IPv4-compatible、NAT64、6to4）会先还原再检查。
  只有当你信任所有能创建订阅源的人时，才设置 `FEEDFORGE_ALLOW_PRIVATE=true`。
- 只有在 `FEEDFORGE_ALLOW_PRIVATE=true` 时才会使用 `HTTP_PROXY`/`HTTPS_PROXY`：
  否则代理会架空上面的连接时检查（真正被拨号的只有代理自己的地址）。
- 只要服务器能从公网访问，就应设置 `FEEDFORGE_TOKEN`，
  否则任何人都能创建订阅源、借用你的服务器抓取网页。
- 抓取到的内容在 XML/JSON 输出中严格转义，在编辑器预览中只按纯文本渲染。
  非 `http`/`https` 的条目链接（比如从恶意页面抓到的 `javascript:`）
  会被直接丢弃，不会传给订阅者的阅读器。

## 许可证

MIT。
