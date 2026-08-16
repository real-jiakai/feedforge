/* FeedForge UI — vanilla JS, no build step. */
"use strict";

/* ---------------- i18n ---------------- */

const I18N = {
  en: {
    tagline: "Turn any web page into an RSS feed",
    username: "Username",
    password: "Password",
    signIn: "Sign in",
    signOut: "Sign out",
    register: "Register",
    createAdmin: "Create admin account",
    setupTitle: "Welcome to FeedForge",
    setupHint: "This is a fresh instance. Create the first account — it becomes the administrator.",
    signInTitle: "Sign in",
    signInHint: "Your feeds are tied to your account.",
    registerTitle: "Create an account",
    registerHint: "Pick a username and a password of at least 8 characters.",
    noAccount: "No account? Register",
    haveAccount: "Have an account? Sign in",
    adminTitle: "Administration",
    allowReg: "Allow new users to register",
    settingsSaved: "Setting saved",
    sessionExpired: "Session expired — please sign in again.",
    signedOut: "Signed out",
    yourFeeds: "Your feeds",
    newFeed: "+ New feed",
    newFeedTitle: "New feed",
    editFeedTitle: "Edit feed",
    backToList: "← All feeds",
    emptyTitle: "No feeds yet",
    emptyHint: "Create your first feed from any web page — or start from one of the recipes below.",
    recipesTitle: "Start from a recipe",
    recipesHint: "Ready-made patterns for sites that have no feed of their own. Each one fills in the whole wizard, so you can see exactly how it works — and change it.",
    useRecipe: "Use this recipe",
    step1: "Source page",
    step1Hint: "Enter the address of the page you want to turn into a feed, then load it to see its HTML source.",
    encAuto: "Encoding: auto",
    loadPage: "Load page",
    reload: "↻ Reload",
    step2: "Extraction rules",
    step2Hint: "Use {%} to capture text and {*} to skip it. The optional global pattern narrows the search area; the item pattern is applied repeatedly to find every item.",
    globalPattern: "Global search pattern (optional)",
    itemPattern: "Item search pattern",
    smartWs: "Smart whitespace (a space in the pattern matches any run of spaces, tabs or newlines)",
    step3: "Output format",
    step3Hint: "Build the feed from your captures: {%1} is the first capture of each item, {%2} the second, and so on. Feed properties may use captures from the global pattern.",
    feedProps: "Feed properties",
    feedTitle: "Feed title",
    feedLink: "Feed link",
    feedDesc: "Feed description",
    itemProps: "Item templates",
    itemTitleT: "Item title",
    itemLinkT: "Item link",
    itemContentT: "Item content (HTML allowed)",
    maxItems: "Max items",
    ttl: "Refresh every (min)",
    reverse: "Reverse item order",
    preview: "Feed preview",
    save: "Save feed",
    saving: "Saving…",
    feedReady: "Your feed is ready",
    copy: "Copy",
    open: "Open",
    copied: "Copied to clipboard",
    copyFailed: "Could not copy — select the URL and copy manually",
    footNote: "a self-hosted, open-source successor to Feed43",
    demoLink: "Demo page",
    edit: "Edit",
    refresh: "Refresh",
    delete: "Delete",
    copyRss: "Copy RSS URL",
    confirmDelete: "Delete this feed? Its URL will stop working.",
    deleted: "Feed deleted",
    refreshed: "Feed refreshed",
    neverFetched: "not fetched yet",
    items: "items",
    globalMatched: "Global pattern matched — captures:",
    matches: "matches",
    showingFirst: "showing first",
    pageLoaded: "loaded",
    chars: "characters",
    truncatedNote: "(view truncated)",
    saveFirstHint: "Load a page and define an item pattern before saving.",
    saveFailed: "Save failed",
    noMatches: "No matches — adjust your item pattern.",
    loading: "Loading…",
  },
  zh: {
    tagline: "把任何网页变成 RSS 订阅源",
    username: "用户名",
    password: "密码",
    signIn: "登录",
    signOut: "退出",
    register: "注册",
    createAdmin: "创建管理员账户",
    setupTitle: "欢迎使用 FeedForge",
    setupHint: "这是一个全新的实例。创建第一个账户 —— 它将成为管理员。",
    signInTitle: "登录",
    signInHint: "订阅源与你的账户绑定。",
    registerTitle: "创建账户",
    registerHint: "取一个用户名，密码至少 8 个字符。",
    noAccount: "没有账户？注册",
    haveAccount: "已有账户？登录",
    adminTitle: "管理",
    allowReg: "允许新用户注册",
    settingsSaved: "设置已保存",
    sessionExpired: "登录已过期，请重新登录。",
    signedOut: "已退出登录",
    yourFeeds: "我的订阅源",
    newFeed: "+ 新建订阅源",
    newFeedTitle: "新建订阅源",
    editFeedTitle: "编辑订阅源",
    backToList: "← 全部订阅源",
    emptyTitle: "还没有订阅源",
    emptyHint: "从任意网页创建你的第一个订阅源，或直接从下面的配方开始。",
    recipesTitle: "从配方开始",
    recipesHint: "为没有 RSS 的网站预先写好的模式。选中后会填满整个向导，你可以直接看清它是怎么写的 —— 也可以随意修改。",
    useRecipe: "使用这个配方",
    step1: "来源页面",
    step1Hint: "输入想要生成订阅源的网页地址，加载后可查看它的 HTML 源码。",
    encAuto: "编码：自动检测",
    loadPage: "加载页面",
    reload: "↻ 重新加载",
    step2: "提取规则",
    step2Hint: "用 {%} 捕获文本，用 {*} 跳过文本。全局模式（可选）先圈定搜索范围；条目模式会被反复匹配，找出每一条内容。",
    globalPattern: "全局搜索模式（可选）",
    itemPattern: "条目搜索模式",
    smartWs: "智能空白（模式中的一个空格可匹配任意空格、制表符或换行）",
    step3: "输出格式",
    step3Hint: "用捕获结果拼出订阅源：{%1} 是每个条目的第一个捕获，{%2} 是第二个，以此类推。订阅源属性可使用全局模式的捕获。",
    feedProps: "订阅源属性",
    feedTitle: "标题",
    feedLink: "链接",
    feedDesc: "描述",
    itemProps: "条目模板",
    itemTitleT: "条目标题",
    itemLinkT: "条目链接",
    itemContentT: "条目内容（可用 HTML）",
    maxItems: "最多条数",
    ttl: "刷新间隔（分钟）",
    reverse: "倒序排列",
    preview: "订阅源预览",
    save: "保存订阅源",
    saving: "保存中…",
    feedReady: "订阅源已生成",
    copy: "复制",
    open: "打开",
    copied: "已复制到剪贴板",
    copyFailed: "复制失败 — 请手动选中地址复制",
    footNote: "开源自部署的 Feed43 继任者",
    demoLink: "演示页面",
    edit: "编辑",
    refresh: "刷新",
    delete: "删除",
    copyRss: "复制 RSS 地址",
    confirmDelete: "确定删除这个订阅源吗？它的地址将立即失效。",
    deleted: "已删除",
    refreshed: "已刷新",
    neverFetched: "尚未抓取",
    items: "条",
    globalMatched: "全局模式已匹配 — 捕获内容：",
    matches: "个匹配",
    showingFirst: "仅显示前",
    pageLoaded: "已加载",
    chars: "个字符",
    truncatedNote: "（源码视图已截断）",
    saveFirstHint: "请先加载页面并填写条目搜索模式。",
    saveFailed: "保存失败",
    noMatches: "没有匹配 — 请调整条目搜索模式。",
    loading: "加载中…",
  },
};

let lang = localStorage.getItem("ff_lang") ||
  ((navigator.language || "").toLowerCase().startsWith("zh") ? "zh" : "en");

function t(key) {
  return (I18N[lang] && I18N[lang][key]) || I18N.en[key] || key;
}

function applyI18n() {
  document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    el.textContent = t(el.dataset.i18n);
  });
  $("#langBtn").textContent = lang === "zh" ? "EN" : "中文";
}

/* ---------------- helpers ---------------- */

const $ = (sel) => document.querySelector(sel);

function show(el, on = true) { el.hidden = !on; }

function toast(msg) {
  const el = $("#toast");
  el.textContent = msg;
  show(el);
  clearTimeout(toast._t);
  toast._t = setTimeout(() => show(el, false), 2400);
}

function safeHref(u) {
  try {
    const p = new URL(u, location.origin);
    if (p.protocol === "http:" || p.protocol === "https:") return p.href;
  } catch (_) { /* fall through */ }
  return "#";
}

// copyText works on plain-HTTP LAN deployments too: navigator.clipboard is
// only defined in secure contexts, so fall back to a temporary textarea.
async function copyText(text) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      toast(t("copied"));
      return;
    }
  } catch (_) { /* fall through to the legacy path */ }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    toast(ok ? t("copied") : t("copyFailed"));
  } catch (_) {
    toast(t("copyFailed"));
  }
}

function stripTags(html) {
  const doc = new DOMParser().parseFromString(html, "text/html");
  return (doc.body.textContent || "").replace(/\s+/g, " ").trim();
}

function relTime(iso) {
  const d = new Date(iso);
  if (isNaN(d)) return "";
  const diff = (d.getTime() - Date.now()) / 1000;
  const rtf = new Intl.RelativeTimeFormat(lang === "zh" ? "zh" : "en", { numeric: "auto" });
  const abs = Math.abs(diff);
  if (abs < 60) return rtf.format(Math.round(diff), "second");
  if (abs < 3600) return rtf.format(Math.round(diff / 60), "minute");
  if (abs < 86400) return rtf.format(Math.round(diff / 3600), "hour");
  return rtf.format(Math.round(diff / 86400), "day");
}

/* ---------------- API ---------------- */

async function api(path, opts = {}) {
  const headers = { ...(opts.headers || {}) };
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(path, { ...opts, headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined });
  let data = null;
  try { data = await res.json(); } catch (_) { /* non-JSON */ }
  if (res.status === 401) {
    const err = new Error((data && data.error) || t("sessionExpired"));
    err.auth = true;
    throw err;
  }
  if (!res.ok) {
    throw new Error((data && data.error) || res.statusText);
  }
  return data;
}

/* ---------------- state & routing ---------------- */

let config = { needsSetup: false, registrationEnabled: false };
let me = null;             // signed-in user, or null
let authMode = "login";    // "setup" | "login" | "register"
let editingId = null;      // feed being edited, or null for a new feed
let pendingPrefill = null; // form values to apply when the editor opens
let previewSeq = 0;        // ignore out-of-order preview responses
let debounceTimer = null;

async function refreshConfig() {
  try {
    config = await api("/api/config");
  } catch (_) { /* keep previous */ }
}

async function fetchMe() {
  try {
    return await api("/api/auth/me");
  } catch (_) {
    return null;
  }
}

async function route() {
  const hash = location.hash || "#/";
  show($("#view-auth"), false);
  show($("#view-list"), false);
  show($("#view-editor"), false);
  show($("#userBox"), !!me);
  if (!me) {
    showAuth(config.needsSetup ? "setup" : "login");
    return;
  }
  $("#userName").textContent = me.username;
  if (hash === "#/new") {
    editingId = null;
    openEditor(null);
  } else if (hash.startsWith("#/edit/")) {
    const id = hash.slice("#/edit/".length);
    try {
      const f = await api("/api/feeds/" + encodeURIComponent(id));
      editingId = f.id;
      openEditor(f);
    } catch (e) {
      handleAuthError(e);
      location.hash = "#/";
    }
  } else {
    editingId = null;
    renderList();
  }
}

/* ---------------- auth view ---------------- */

function showAuth(mode) {
  authMode = mode;
  show($("#view-auth"));
  show($("#authError"), false);
  const title = $("#authTitle"), hint = $("#authHint"),
    btn = $("#authBtn"), sw = $("#authSwitch");
  if (mode === "setup") {
    title.textContent = t("setupTitle");
    hint.textContent = t("setupHint");
    btn.textContent = t("createAdmin");
    show(sw, false);
  } else if (mode === "register") {
    title.textContent = t("registerTitle");
    hint.textContent = t("registerHint");
    btn.textContent = t("register");
    sw.textContent = t("haveAccount");
    show(sw);
  } else {
    title.textContent = t("signInTitle");
    hint.textContent = t("signInHint");
    btn.textContent = t("signIn");
    sw.textContent = t("noAccount");
    show(sw, !!config.registrationEnabled);
  }
  $("#a-password").autocomplete = mode === "login" ? "current-password" : "new-password";
  $("#a-username").focus();
}

async function submitAuth() {
  const username = $("#a-username").value.trim();
  const password = $("#a-password").value;
  const path = authMode === "login" ? "/api/auth/login" : "/api/auth/register";
  const btn = $("#authBtn");
  btn.disabled = true;
  try {
    me = await api(path, { method: "POST", body: { username, password } });
    $("#a-password").value = "";
    await refreshConfig();
    if (location.hash && location.hash !== "#/") location.hash = "#/";
    route();
  } catch (e) {
    $("#authError").textContent = e.message;
    show($("#authError"));
  } finally {
    btn.disabled = false;
  }
}

async function signOut() {
  try {
    await api("/api/auth/logout", { method: "POST", body: {} });
  } catch (_) { /* session may already be gone */ }
  me = null;
  await refreshConfig();
  toast(t("signedOut"));
  location.hash = "#/";
  route();
}

/* ---------------- feed list ---------------- */

async function renderList() {
  show($("#view-list"));
  let feeds = [];
  try {
    feeds = (await api("/api/feeds")) || [];
  } catch (e) {
    handleAuthError(e);
    return;
  }
  const wrap = $("#feedCards");
  wrap.textContent = "";
  show($("#emptyState"), feeds.length === 0);
  for (const f of feeds) {
    wrap.appendChild(feedCard(f));
  }
  renderRecipes();
  renderAdminPanel();
}

async function renderAdminPanel() {
  const isAdmin = !!(me && me.isAdmin);
  show($("#adminPanel"), isAdmin);
  if (!isAdmin) return;
  try {
    const s = await api("/api/admin/settings");
    $("#regToggle").checked = !!s.registrationEnabled;
  } catch (_) { /* leave as-is */ }
}

function feedCard(f) {
  const card = document.createElement("div");
  card.className = "card";

  const h = document.createElement("h3");
  h.textContent = f.title || f.sourceUrl;
  card.appendChild(h);

  const src = document.createElement("div");
  src.className = "src";
  src.textContent = f.sourceUrl;
  card.appendChild(src);

  const status = document.createElement("div");
  status.className = "status";
  const dot = document.createElement("span");
  dot.className = "dot " + (f.lastError ? "err" : (f.lastFetchAt ? "ok" : ""));
  status.appendChild(dot);
  const stext = document.createElement("span");
  if (f.lastError) {
    stext.textContent = f.lastError;
  } else if (f.lastFetchAt) {
    stext.textContent = `${f.lastItemCount} ${t("items")} · ${relTime(f.lastFetchAt)}`;
  } else {
    stext.textContent = t("neverFetched");
  }
  status.appendChild(stext);
  card.appendChild(status);

  const rssUrl = `${location.origin}/feeds/${f.id}.xml`;
  const actions = document.createElement("div");
  actions.className = "actions";

  const copyBtn = mkBtn(t("copyRss"), () => copyText(rssUrl));
  const openA = document.createElement("a");
  openA.className = "btn small ghost";
  openA.textContent = t("open");
  openA.href = rssUrl;
  openA.target = "_blank";
  openA.rel = "noopener";
  const editBtn = mkBtn(t("edit"), () => { location.hash = "#/edit/" + f.id; });
  const refreshBtn = mkBtn(t("refresh"), async () => {
    refreshBtn.disabled = true;
    try {
      // The empty body sets Content-Type: application/json, which the
      // server requires on every mutating call (CSRF guard).
      await api(`/api/feeds/${f.id}/refresh`, { method: "POST", body: {} });
      toast(t("refreshed"));
      renderList();
    } catch (e) {
      handleAuthError(e);
      renderList();
    }
  });
  const delBtn = mkBtn(t("delete"), async () => {
    if (!confirm(t("confirmDelete"))) return;
    try {
      await api("/api/feeds/" + f.id, { method: "DELETE" });
      toast(t("deleted"));
      renderList();
    } catch (e) {
      handleAuthError(e);
    }
  });
  delBtn.classList.add("danger");

  actions.append(copyBtn, openA, editBtn, refreshBtn, delBtn);
  card.appendChild(actions);
  return card;
}

function mkBtn(label, onClick) {
  const b = document.createElement("button");
  b.className = "btn small ghost";
  b.textContent = label;
  b.addEventListener("click", onClick);
  return b;
}

function handleAuthError(e) {
  toast(e.message);
  if (e.auth) {
    me = null;
    refreshConfig().then(route);
  }
}

/* ---------------- editor ---------------- */

const FIELDS = ["sourceUrl", "encoding", "globalPattern", "itemPattern",
  "title", "link", "description", "itemTitle", "itemLink", "itemContent"];

function setForm(f) {
  for (const k of FIELDS) $("#f-" + k).value = (f && f[k]) || "";
  $("#f-smartWhitespace").checked = f ? !!f.smartWhitespace : true;
  $("#f-reverse").checked = f ? !!f.reverse : false;
  $("#f-maxItems").value = (f && f.maxItems) || 25;
  $("#f-ttlMinutes").value = (f && f.ttlMinutes) || 30;
}

function collectForm() {
  const out = {};
  for (const k of FIELDS) out[k] = $("#f-" + k).value;
  out.sourceUrl = out.sourceUrl.trim();
  out.smartWhitespace = $("#f-smartWhitespace").checked;
  out.reverse = $("#f-reverse").checked;
  out.maxItems = parseInt($("#f-maxItems").value, 10) || 25;
  out.ttlMinutes = parseInt($("#f-ttlMinutes").value, 10) || 30;
  return out;
}

function openEditor(feedOrNull) {
  show($("#view-editor"));
  $("#editorTitle").textContent = feedOrNull ? t("editFeedTitle") : t("newFeedTitle");
  show($("#savedPanel"), false);
  show($("#fetchError"), false);
  show($("#pagePanel"), false);
  show($("#globalResult"), false);
  show($("#itemError"), false);
  show($("#itemsResult"), false);
  show($("#feedPreviewPanel"), false);
  $("#saveStatus").textContent = "";

  setForm(feedOrNull);
  if (!feedOrNull && pendingPrefill) {
    setForm(pendingPrefill);
    pendingPrefill = null;
  }
  if ($("#f-sourceUrl").value.trim()) {
    runPreview({ includePage: true });
  }
}

function schedulePreview() {
  clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => runPreview({}), 500);
}

async function runPreview({ includePage = false, force = false }) {
  const form = collectForm();
  if (!form.sourceUrl) return;
  const seq = ++previewSeq;
  const req = { ...form, includePage, forceRefetch: force };
  let resp;
  try {
    resp = await api("/api/preview", { method: "POST", body: req });
  } catch (e) {
    if (seq === previewSeq) handleAuthError(e);
    return;
  }
  if (seq !== previewSeq) return; // a newer preview superseded this one
  renderPreview(resp, includePage);
}

function renderPreview(resp, includedPage) {
  // fetch status
  if (resp.fetchError) {
    $("#fetchError").textContent = resp.fetchError;
    show($("#fetchError"));
    return;
  }
  show($("#fetchError"), false);

  if (includedPage && resp.pageExcerpt !== undefined) {
    $("#pageSource").textContent = resp.pageExcerpt;
    $("#pageInfo").textContent =
      `${t("pageLoaded")}: ${resp.finalUrl} — ${resp.pageLength} ${t("chars")} ` +
      (resp.pageTruncated ? t("truncatedNote") : "");
    show($("#pagePanel"));
  }

  // global pattern result
  const g = $("#globalResult");
  g.textContent = "";
  if (resp.globalError) {
    g.className = "error-box";
    g.textContent = resp.globalError;
    show(g);
  } else if (resp.globalCaptures && resp.globalCaptures.length) {
    g.className = "result";
    const lbl = document.createElement("div");
    lbl.textContent = t("globalMatched");
    g.appendChild(lbl);
    resp.globalCaptures.forEach((c, i) => {
      const chip = document.createElement("span");
      chip.className = "capture-chip";
      const label = document.createElement("span");
      label.className = "chip-label";
      label.textContent = `{%${i + 1}} `;
      chip.appendChild(label);
      chip.appendChild(document.createTextNode(
        c.length > 120 ? c.slice(0, 120) + "…" : c));
      g.appendChild(chip);
    });
    show(g);
  } else {
    show(g, false);
  }

  // item pattern result
  if (resp.itemError) {
    $("#itemError").textContent = resp.itemError;
    show($("#itemError"));
    show($("#itemsResult"), false);
  } else {
    show($("#itemError"), false);
    renderItemsTable(resp);
  }

  renderFeedPreview(resp);
}

function renderItemsTable(resp) {
  const items = resp.items || [];
  const table = $("#itemsTable");
  table.textContent = "";
  if (!items.length) {
    if (collectForm().itemPattern.trim()) {
      $("#itemError").textContent = t("noMatches");
      show($("#itemError"));
    }
    show($("#itemsResult"), false);
    return;
  }
  let info = `${resp.totalMatches} ${t("matches")}`;
  if (resp.totalMatches > items.length) {
    info += ` · ${t("showingFirst")} ${items.length}`;
  }
  $("#itemsInfo").textContent = info;

  const nCaps = Math.max(...items.map((it) => it.captures.length));
  const thead = document.createElement("tr");
  const thN = document.createElement("th");
  thN.textContent = "#";
  thead.appendChild(thN);
  for (let i = 1; i <= nCaps; i++) {
    const th = document.createElement("th");
    th.textContent = `{%${i}}`;
    thead.appendChild(th);
  }
  table.appendChild(thead);
  items.forEach((it, idx) => {
    const tr = document.createElement("tr");
    const tdN = document.createElement("td");
    tdN.className = "n";
    tdN.textContent = String(idx + 1);
    tr.appendChild(tdN);
    for (let i = 0; i < nCaps; i++) {
      const td = document.createElement("td");
      const v = it.captures[i] || "";
      td.textContent = v.length > 160 ? v.slice(0, 160) + "…" : v;
      tr.appendChild(td);
    }
    table.appendChild(tr);
  });
  show($("#itemsResult"));
}

function renderFeedPreview(resp) {
  const items = resp.items || [];
  const box = $("#feedPreview");
  box.textContent = "";
  if (!items.length || resp.itemError) {
    show($("#feedPreviewPanel"), false);
    return;
  }
  for (const it of items.slice(0, 10)) {
    const div = document.createElement("div");
    div.className = "pv-item";
    const a = document.createElement("a");
    a.className = "pv-title";
    a.textContent = it.title || "(no title)";
    a.href = safeHref(it.link);
    a.target = "_blank";
    a.rel = "noopener";
    div.appendChild(a);
    if (it.link) {
      const l = document.createElement("div");
      l.className = "pv-link";
      l.textContent = it.link;
      div.appendChild(l);
    }
    if (it.content) {
      const c = document.createElement("div");
      c.className = "pv-content";
      c.textContent = stripTags(it.content); // untrusted HTML → text only
      div.appendChild(c);
    }
    box.appendChild(div);
  }
  show($("#feedPreviewPanel"));
}

/* ---------------- save ---------------- */

async function saveFeed() {
  const form = collectForm();
  if (!form.sourceUrl || !form.itemPattern.trim()) {
    toast(t("saveFirstHint"));
    return;
  }
  const btn = $("#saveBtn");
  btn.disabled = true;
  $("#saveStatus").textContent = t("saving");
  try {
    let saved;
    if (editingId) {
      saved = await api("/api/feeds/" + editingId, { method: "PUT", body: form });
    } else {
      saved = await api("/api/feeds", { method: "POST", body: form });
      editingId = saved.id;
      history.replaceState(null, "", "#/edit/" + saved.id);
    }
    $("#saveStatus").textContent = "";
    const rss = `${location.origin}/feeds/${saved.id}.xml`;
    const json = `${location.origin}/feeds/${saved.id}.json`;
    $("#savedRssUrl").textContent = rss;
    $("#savedJsonUrl").textContent = json;
    $("#savedRssOpen").href = rss;
    show($("#savedPanel"));
    $("#savedPanel").scrollIntoView({ behavior: "smooth", block: "center" });
  } catch (e) {
    $("#saveStatus").textContent = `${t("saveFailed")}: ${e.message}`;
    handleAuthError(e);
  } finally {
    btn.disabled = false;
  }
}

/* ---------------- recipes ---------------- */

let recipes = [];

function renderRecipes() {
  const wrap = $("#recipeCards");
  if (!wrap) return;
  wrap.textContent = "";
  for (const r of recipes) {
    const card = document.createElement("div");
    card.className = "card recipe-card";

    const h = document.createElement("h3");
    h.textContent = r.name;
    card.appendChild(h);

    const note = document.createElement("div");
    note.className = "src";
    note.textContent = lang === "zh" ? r.noteZh : r.note;
    card.appendChild(note);

    const pat = document.createElement("code");
    pat.className = "recipe-pattern";
    pat.textContent = r.feed.itemPattern;
    card.appendChild(pat);

    const actions = document.createElement("div");
    actions.className = "actions";
    const use = document.createElement("button");
    use.className = "btn small primary";
    use.textContent = t("useRecipe");
    use.addEventListener("click", () => useRecipe(r));
    actions.appendChild(use);
    card.appendChild(actions);

    wrap.appendChild(card);
  }
}

// useRecipe drops a recipe's definition into a fresh editor. The patterns
// are shown, not hidden: the point is to be able to read and adapt them.
function useRecipe(r) {
  pendingPrefill = { ...r.feed };
  if (location.hash === "#/new") route();
  else location.hash = "#/new";
}

/* ---------------- wire up ---------------- */

document.addEventListener("DOMContentLoaded", async () => {
  applyI18n();

  $("#langBtn").addEventListener("click", () => {
    lang = lang === "zh" ? "en" : "zh";
    localStorage.setItem("ff_lang", lang);
    applyI18n();
    // Re-rendering the editor would call setForm() and wipe unsaved work,
    // so only the card list (whose labels are built in JS) is redrawn.
    if (!$("#view-auth").hidden) {
      showAuth(authMode);
    } else if ($("#view-editor").hidden) {
      route();
    } else {
      $("#editorTitle").textContent = editingId ? t("editFeedTitle") : t("newFeedTitle");
      schedulePreview();
    }
  });
  $("#logoutBtn").addEventListener("click", signOut);
  $("#authBtn").addEventListener("click", submitAuth);
  $("#authSwitch").addEventListener("click", (e) => {
    e.preventDefault();
    showAuth(authMode === "login" ? "register" : "login");
  });
  for (const id of ["a-username", "a-password"]) {
    document.getElementById(id).addEventListener("keydown", (e) => {
      if (e.key === "Enter") submitAuth();
    });
  }
  $("#regToggle").addEventListener("change", async () => {
    const box = $("#regToggle");
    try {
      const s = await api("/api/admin/settings",
        { method: "PUT", body: { registrationEnabled: box.checked } });
      config.registrationEnabled = !!s.registrationEnabled;
      toast(t("settingsSaved"));
    } catch (e) {
      box.checked = !box.checked;
      handleAuthError(e);
    }
  });
  $("#loadBtn").addEventListener("click", () => runPreview({ includePage: true, force: true }));
  $("#reloadBtn").addEventListener("click", () => runPreview({ includePage: true, force: true }));
  $("#saveBtn").addEventListener("click", saveFeed);

  document.querySelectorAll("[data-copy]").forEach((btn) => {
    btn.addEventListener("click", () => {
      copyText(document.querySelector(btn.dataset.copy).textContent);
    });
  });

  for (const id of ["f-globalPattern", "f-itemPattern", "f-itemTitle", "f-itemLink",
    "f-itemContent", "f-title", "f-link", "f-description", "f-maxItems", "f-reverse",
    "f-smartWhitespace", "f-encoding"]) {
    const el = document.getElementById(id);
    el.addEventListener("input", schedulePreview);
    el.addEventListener("change", schedulePreview);
  }
  $("#f-sourceUrl").addEventListener("keydown", (e) => {
    if (e.key === "Enter") runPreview({ includePage: true, force: true });
  });

  await refreshConfig();
  me = await fetchMe();

  try {
    recipes = (await api("/api/recipes")) || [];
  } catch (_) { recipes = []; }

  window.addEventListener("hashchange", route);
  route();
});
