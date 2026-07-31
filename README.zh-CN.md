# FeedForge

**把任何网页变成 RSS 订阅源** —— 向经典服务 [Feed43](https://en.wikipedia.org/wiki/Feed43) 致敬的开源自部署复刻。

[English README](README.md)

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

如果服务器可以从公网访问，建议设置令牌（订阅源地址仍然公开可读）：

```bash
FEEDFORGE_TOKEN=change-me docker compose up -d
```

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
