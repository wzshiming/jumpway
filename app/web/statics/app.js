"use strict";

(() => {
  const STR = {
    en: {
      appName: "JumpWay",
      checking: "Status unknown",
      running: "Running",
      stopped: "Stopped",
      retrying: "Retrying ({attempt})",
      remote: "remote",
      reload: "Reload from disk",
      saveApply: "Save & Apply",
      unsaved: "Unsaved changes",
      discardChanges: "Discard unsaved changes?",
      navigation: "Configuration pages",
      ruleTabs: "Rule editors",
      yaml: "Advanced YAML",
      webUI: "Web UI",
      webUIHint: "Address of this page and the REST API. Only local addresses make sense here.",
      host: "Host",
      port: "Port",
      invalidPort: "Port must be a whole number between 0 and 65535.",
      rules: "Rules",
      rule: "Rule name",
      enabled: "Enabled",
      entry: "Entry",
      outbound: "Outbound",
      listenThrough: "Listen through",
      outboundChain: "Outbound chain",
      username: "Username",
      password: "Password",
      credentialsHint: "Optional credentials for HTTP Basic / SOCKS5 clients. Usernames must not contain \":\".",
      listenThroughHint: "Leave empty to open the port on this machine. Otherwise the first hop binds host:port on its side (SSH remote forwarding; the remote sshd binds loopback unless GatewayPorts is enabled) and the last hop is dialed from this machine. Only ssh://, cmd: and nc hops can bind.",
      deleteRule: "Delete",
      confirmDeleteRule: "Delete rule \"{name}\"?",
      deleted: "Rule deleted.",
      newRule: "New rule",
      noRules: "No rules yet.",
      direct: "direct",
      hop: "Hop {number}",
      hopExit: "exit node",
      hopBinds: "binds the port",
      hopDialed: "dialed from this machine",
      hopsHint: "Hop 1 is the exit node, closest to the target; the last hop is dialed from this machine. Leave empty for a direct connection.",
      chainLocal: "this machine",
      chainTarget: "target",
      up: "Up",
      down: "Down",
      deleteHop: "Delete hop",
      addHop: "+ Hop",
      proxyURL: "Proxy URL",
      remove: "Remove",
      addURL: "+ URL",
      build: "Build…",
      builderTitle: "Build proxy URL",
      protocol: "Protocol",
      useURL: "Use URL",
      cancel: "Cancel",
      optional: "(optional)",
      preview: "Preview",
      builderHint: "Host and a numeric port are required.",
      builderShadowsocksHint: "Host, a numeric port, encryption method, and password are required.",
      builderLoading: "Loading protocols...",
      builderLoadFailed: "Cannot load protocols. Close the dialog and try again.",
      "field.username": "Username",
      "field.password": "Password",
      "field.host": "Host",
      "field.port": "Port",
      "field.identity": "Identity file",
      "field.encrypto": "Encryption method",
      noProxy: "No proxy",
      hostsCIDRs: "Hosts / CIDRs",
      fromEnv: "From environment variables",
      fromFiles: "From files or URLs",
      listHint: "Hosts, domains, or CIDRs that bypass the proxy, one per line.",
      envHint: "One environment variable name per line; each holds a comma-separated bypass list.",
      filesHint: "One file path or URL per line; each source lists one bypass entry per line.",
      yamlSource: "Configuration YAML",
      resources: "Resources",
      apiDocs: "API docs",
      pprof: "pprof",
      github: "GitHub",
      loading: "Loading...",
      saving: "Saving and applying...",
      saved: "Saved and applied.",
      reloaded: "Reloaded from disk.",
      retry: "Retry",
      movedTo: "The web UI has moved to",
      movedUnknown: "Saved and applied. The listen address changed. Open the address shown in the tray status item.",
      unreachable: "Cannot reach JumpWay.",
      recoveryHint: "The Web UI may have stopped. Use the tray's Edit Config to check ~/.jumpway/config.yaml, then Reload Config to restart it.",
      requestFailed: "The request could not be completed.",
      invalidResponse: "JumpWay returned an unreadable response. Try reloading from disk."
    },
    zh: {
      appName: "JumpWay",
      checking: "状态未知",
      running: "运行中",
      stopped: "已停止",
      retrying: "重试中 ({attempt})",
      remote: "远端",
      reload: "从磁盘重新加载",
      saveApply: "保存并应用",
      unsaved: "未保存的修改",
      discardChanges: "放弃未保存的修改吗？",
      navigation: "配置页面",
      ruleTabs: "规则编辑器",
      yaml: "高级 YAML",
      webUI: "网页配置",
      webUIHint: "本页面和 REST API 的地址。此处应使用本机地址。",
      host: "主机",
      port: "端口",
      invalidPort: "端口必须是 0 到 65535 之间的整数。",
      rules: "规则",
      rule: "规则名称",
      enabled: "启用",
      entry: "入口",
      outbound: "出口",
      listenThrough: "经由监听",
      outboundChain: "出口链路",
      username: "用户名",
      password: "密码",
      credentialsHint: "HTTP Basic / SOCKS5 客户端的可选认证信息。用户名不能包含冒号（:）。",
      listenThroughHint: "留空时在本机打开端口；否则由第一个节点在其所在机器上绑定主机和端口（SSH 远程端口转发；远端 sshd 默认仅绑定回环地址，启用 GatewayPorts 后才能绑定其他地址），最后一个节点由本机直接连接。只有 ssh://、cmd: 和 nc 节点支持监听。",
      deleteRule: "删除",
      confirmDeleteRule: "删除规则“{name}”吗？",
      deleted: "已删除规则。",
      newRule: "新建规则",
      noRules: "尚无规则。",
      direct: "直连",
      hop: "跳板节点 {number}",
      hopExit: "出口节点",
      hopBinds: "绑定端口",
      hopDialed: "由本机连接",
      hopsHint: "节点 1 是最靠近目标的出口节点，最后一个节点由本机直接连接。留空时直连目标。",
      chainLocal: "本机",
      chainTarget: "目标",
      up: "上移",
      down: "下移",
      deleteHop: "删除节点",
      addHop: "+ 跳板节点",
      proxyURL: "代理 URL",
      remove: "移除",
      addURL: "+ URL",
      build: "拼装…",
      builderTitle: "拼装代理 URL",
      protocol: "协议",
      useURL: "使用此 URL",
      cancel: "取消",
      optional: "（可选）",
      preview: "预览",
      builderHint: "主机和数字端口为必填项。",
      builderShadowsocksHint: "主机、数字端口、加密方式和密码为必填项。",
      builderLoading: "正在加载协议...",
      builderLoadFailed: "无法加载协议，请关闭对话框后重试。",
      "field.username": "用户名",
      "field.password": "密码",
      "field.host": "主机",
      "field.port": "端口",
      "field.identity": "私钥文件",
      "field.encrypto": "加密方式",
      noProxy: "不走代理",
      hostsCIDRs: "主机 / CIDR",
      fromEnv: "从环境变量读取",
      fromFiles: "从文件或 URL 读取",
      listHint: "不走代理的主机、域名或 CIDR，每行一项。",
      envHint: "每行一个环境变量名；变量值为逗号分隔的不走代理列表。",
      filesHint: "每行一个文件路径或 URL；每个来源的内容为每行一项的不走代理列表。",
      yamlSource: "YAML 配置",
      resources: "相关链接",
      apiDocs: "API 文档",
      pprof: "pprof",
      github: "GitHub",
      loading: "正在加载...",
      saving: "正在保存并应用...",
      saved: "已保存并应用。",
      reloaded: "已从磁盘重新加载。",
      retry: "重试",
      movedTo: "网页配置已移至",
      movedUnknown: "已保存并应用。监听地址已改变，请打开托盘状态栏显示的新地址。",
      unreachable: "无法连接到 JumpWay。",
      recoveryHint: "网页配置可能已停止。请通过托盘菜单的“编辑配置”检查 ~/.jumpway/config.yaml，再点击“重新加载配置”重新启动。",
      requestFailed: "请求未能完成。",
      invalidResponse: "JumpWay 返回了无法读取的响应，请尝试从磁盘重新加载。"
    }
  };

  const override = new URLSearchParams(location.search).get("lang");
  const languages = [navigator.language, ...(navigator.languages || [])];
  const language = ["en", "zh"].includes(override) ? override
    : languages.some(value => /^zh/i.test(value || "")) ? "zh" : "en";
  const t = (key, values = {}) => STR[language][key].replace(/\{(\w+)\}/g, (_, name) => values[name]);
  document.documentElement.lang = language === "zh" ? "zh-CN" : "en";

  async function api(route, { method = "GET", body } = {}) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 15000);
    try {
      const response = await fetch("/apis/configs" + route, {
        method,
        headers: body === undefined ? { Accept: "application/json" }
          : { Accept: "application/json", "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
        cache: "no-store",
        signal: controller.signal
      });
      if (!response.ok) throw new Error(await response.text());
      return await response.json();
    } catch (error) {
      if (error instanceof SyntaxError) throw new Error(t("invalidResponse"));
      throw error;
    } finally {
      clearTimeout(timeout);
    }
  }

  const list = value => Array.isArray(value) ? value : [];
  const text = value => String(value ?? "");
  const cleanLines = value => value.map(entry => text(entry).trim()).filter(Boolean);

  function normalizeWay(way) {
    return list(way).map(node => ({
      lb: (typeof node === "string" ? node.split("|")
        : Array.isArray(node) ? node : list(node?.lb)).map(text)
    }));
  }

  const find = (selector, root = document) => root.querySelector(selector);
  const all = (selector, root = document) => Array.from(root.querySelectorAll(selector));
  const ui = {
    main: find("#main"), nav: find("#page-nav"),
    dirty: find("#dirty-badge"), activity: find("#activity"),
    unreachable: find("#unreachable-banner"), moved: find("#moved-banner"), movedLink: find("#moved-link"),
    movedMessage: find("#moved-message"),
    statusDot: find("#status-dot"), statusLabel: find("#status-label"),
    statusAddress: find("#status-address"), statusError: find("#status-error"),
    statusRules: find("#rule-status"),
    builder: find("#url-builder"), builderForm: find("#builder-form"),
    builderProtocol: find("#builder-protocol"), builderFields: find("#builder-fields"),
    builderPreview: find("#builder-preview"), builderHint: find("#builder-hint"),
    builderUse: find("#builder-use"), builderCancel: find("#builder-cancel")
  };
  let page = null;
  let activeHash = "";
  let newRule = false;
  let dirty = false;
  let busy = false;
  let movedAddress = "";
  let statusRequest = null;
  let requestUnreachable = false;
  let builderLayouts = [];
  let builderRequest = null;
  let builderTarget = null;
  let toastTimer;

  function setDirty(value) {
    dirty = value;
    ui.dirty.hidden = !value;
  }

  function setBusy(value) {
    busy = value;
    ui.main.setAttribute("aria-busy", String(value));
    const fields = find("#page-fields");
    if (fields) fields.disabled = value || !page.loaded;
    const panel = find("#rule-panel");
    if (panel) panel.setAttribute("aria-busy", String(value));
    all("#rule-tabs button, #retry").forEach(button => { button.disabled = value; });
    all("a[href^='#/']").forEach(link => {
      if (value) link.setAttribute("aria-disabled", "true");
      else link.removeAttribute("aria-disabled");
    });
  }

  function routeFor(hash) {
    const pages = { "#/": "rules", "#/rules": "rules", "#/web-ui": "web-ui", "#/no-proxy": "no-proxy", "#/yaml": "yaml" };
    if (Object.prototype.hasOwnProperty.call(pages, hash)) return { hash, kind: pages[hash], name: null };
    const match = /^#\/rules\/([^/]+)$/.exec(hash);
    if (match) {
      try { return { hash, kind: "rules", name: decodeURIComponent(match[1]) }; } catch {}
    }
    return { hash: "#/", kind: "rules", name: null };
  }

  const ruleRoute = name => "#/rules/" + encodeURIComponent(name);
  const rulePath = name => "/rules/" + encodeURIComponent(name);

  function mayLeave() {
    return !busy && (!dirty || confirm(t("discardChanges")));
  }

  async function navigate(hash, { replace = false, create = false, confirmed = false, notify = "" } = {}) {
    if (!confirmed && !mayLeave()) return;
    const route = routeFor(hash);
    history[replace ? "replaceState" : "pushState"](null, "", route.hash);
    newRule = create;
    await renderRoute(route, notify);
  }

  async function hashChanged() {
    const route = routeFor(location.hash);
    if (route.hash === activeHash) {
      if (location.hash !== route.hash) history.replaceState(null, "", activeHash);
      return;
    }
    if (!mayLeave()) {
      history.replaceState(null, "", activeHash);
      return;
    }
    if (location.hash !== route.hash) history.replaceState(null, "", route.hash);
    newRule = false;
    await renderRoute(route);
  }

  async function renderRoute(route, notify = "") {
    const focusPage = Boolean(activeHash);
    activeHash = route.hash;
    page = { kind: route.kind, save: null, loaded: false };
    setDirty(false);
    if (ui.builder.open) ui.builder.close();
    ui.main.replaceChildren(clone("page"));
    ui.main.dataset.page = route.kind;
    const title = { rules: "rules", "web-ui": "webUI", "no-proxy": "noProxy", yaml: "yaml" };
    find("#page-heading").textContent = t(title[route.kind]);
    document.title = t(title[route.kind]) + " | " + t("appName");
    all("[data-page]", ui.nav).forEach(link => {
      if (link.dataset.page === route.kind) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });
    find("#retry").addEventListener("click", () => navigate(activeHash, { replace: true, create: newRule }));
    clearErrors();
    setBusy(true);
    activity("loading");
    try {
      const loaders = { rules: loadRules, "web-ui": loadWebUI, "no-proxy": loadNoProxy, yaml: loadYAML };
      await loaders[route.kind](route);
      page.loaded = true;
      activity(notify, Boolean(notify));
    } catch (error) {
      activity("");
      showError(error, true);
      find("#retry").hidden = false;
    } finally {
      setBusy(false);
      if (focusPage) {
        const target = route.kind === "rules" ? find("#rule-tabs [aria-selected=true]") : null;
        (target || find("#page-heading")).focus();
      }
    }
  }

  async function loadRules(route) {
    const entries = list(await api("/rules"));
    const originalName = route.name !== null ? route.name : newRule ? null : entries[0]?.name ?? null;
    newRule = originalName === null;
    const view = clone("rules");
    find("#page-content").append(view);
    const tabs = find("#rule-tabs");
    entries.forEach((rule, index) => {
      const tab = clone("rule-tab");
      tab.id = "rule-tab-" + index;
      find(".tab-name", tab).textContent = rule.name;
      selectRuleTab(tab, rule.name === originalName);
      tab.addEventListener("click", () => {
        if (rule.name !== originalName) navigate(ruleRoute(rule.name));
      });
      tabs.append(tab);
    });
    const add = clone("new-tab");
    selectRuleTab(add, newRule);
    add.addEventListener("click", () => {
      if (!newRule) navigate("#/rules", { create: true, replace: activeHash === "#/rules" });
    });
    tabs.append(add);
    bindRuleTabs(tabs);
    if (!find("[aria-selected=true]", tabs)) add.tabIndex = 0;
    setBusy(true);
    const rule = originalName === null ? { name: "", listen: { host: "127.0.0.1", port: 0 }, way: [] }
      : await api(rulePath(originalName));
    find("#rule-name").value = rule.name;
    find("#rule-enabled").checked = !rule.disabled;
    for (const field of ["host", "port", "username", "password"]) {
      find("#listen-" + field).value = text(rule.listen[field]);
    }
    find("#new-rule-heading").hidden = !newRule;
    find("#no-rules").hidden = entries.length !== 0;
    find("#delete-rule").hidden = newRule;
    const editors = {
      listen: createHopEditor({ container: find("#listen-editor"), model: normalizeWay(rule.listen.way),
        idPrefix: "listen", roles: { first: "hopBinds", last: "hopDialed" } }),
      outbound: createHopEditor({ container: find("#outbound-editor"), model: normalizeWay(rule.way),
        idPrefix: "outbound", roles: { first: "hopExit", last: "hopDialed" } })
    };
    Object.values(editors).forEach(editor => editor.render());
    bindEditor(find("#rule-form"), () => {
      const body = readRule(editors);
      return write(originalName === null ? "/rules" : rulePath(originalName), {
        method: originalName === null ? "POST" : "PUT", body
      }, { after: () => navigate(ruleRoute(body.name), { replace: true, confirmed: true, notify: "saved" }) });
    });
    find("#delete-rule").addEventListener("click", async () => {
      if (busy || !confirm(t("confirmDeleteRule", { name: originalName }))) return;
      if (dirty && !confirm(t("discardChanges"))) return;
      await write(rulePath(originalName), { method: "DELETE" }, {
        toast: "deleted", after: () => navigate("#/rules", { replace: true, confirmed: true, notify: "deleted" })
      });
    });
  }

  function readPort(input) {
    const port = Number(input.value);
    if (!input.value.trim() || !Number.isInteger(port) || port < 0 || port > 65535) {
      throw new Error(t("invalidPort"));
    }
    return port;
  }

  function readRule(editors) {
    const listen = { host: find("#listen-host").value.trim(), port: readPort(find("#listen-port")) };
    for (const field of ["username", "password"]) {
      const value = find("#listen-" + field).value;
      if (value) listen[field] = value;
    }
    const way = editors.listen.read();
    if (way.length) listen.way = way;
    const rule = { name: find("#rule-name").value.trim(), listen, way: editors.outbound.read() };
    if (!find("#rule-enabled").checked) rule.disabled = true;
    return rule;
  }

  async function loadWebUI() {
    const address = await api("/web-ui");
    const form = clone("web-ui");
    find("#page-content").append(form);
    find("#web-ui-host").value = text(address.host);
    find("#web-ui-port").value = address.port ?? 0;
    bindEditor(form, () => {
      const body = { host: find("#web-ui-host").value.trim(), port: readPort(find("#web-ui-port")) };
      return write("/web-ui", { method: "PUT", body }, { mayMove: true, address: submittedAddress(body) });
    });
  }

  async function loadNoProxy() {
    const bypass = await api("/no-proxy");
    const form = clone("no-proxy");
    find("#page-content").append(form);
    const fields = { list: "#no-proxy-list", from_env: "#no-proxy-env", from_file: "#no-proxy-files" };
    Object.entries(fields).forEach(([key, selector]) => { find(selector).value = list(bypass[key]).join("\n"); });
    bindEditor(form, () => {
      const body = Object.fromEntries(Object.entries(fields).map(([key, selector]) => [key, cleanLines(find(selector).value.split(/\r?\n/))]));
      return write("/no-proxy", { method: "PUT", body });
    });
  }

  async function loadYAML() {
    const raw = await api("/raw");
    const form = clone("yaml");
    find("#page-content").append(form);
    find("#yaml-source").value = raw.yaml;
    bindEditor(form, () => write("/raw", { method: "PUT", body: { yaml: find("#yaml-source").value } }, {
      mayMove: true, after: async () => { find("#yaml-source").value = (await api("/raw")).yaml; }
    }));
    find("#reload").addEventListener("click", () => navigate("#/yaml", { replace: true, notify: "reloaded" }));
  }

  function localize(root) {
    all("[data-i18n]", root).forEach(element => { element.textContent = t(element.dataset.i18n); });
    all("[data-i18n-aria]", root).forEach(element => {
      element.setAttribute("aria-label", t(element.dataset.i18nAria));
    });
    all("[data-i18n-title]", root).forEach(element => { element.title = t(element.dataset.i18nTitle); });
  }

  function clone(name) {
    const fragment = find("#" + name + "-template").content.cloneNode(true);
    localize(fragment);
    return fragment.firstElementChild;
  }

  function bindEditor(form, submit) {
    page.save = submit;
    ["input", "change"].forEach(event => form.addEventListener(event, () => {
      if (!busy) setDirty(true);
    }));
    form.addEventListener("submit", event => { event.preventDefault(); save(); });
  }

  async function save() {
    if (busy || ui.builder.open) return;
    try {
      await page?.save?.();
    } catch (error) {
      showError(error, true);
    }
  }

  async function write(resource, request, { toast = "saved", after, mayMove = false, address = "" } = {}) {
    setBusy(true);
    clearErrors();
    activity("saving");
    try {
      if (statusRequest) await statusRequest.catch(() => {});
      await api(resource, request);
    } catch (error) {
      activity("");
      showError(error, true);
      setBusy(false);
      return;
    }
    setDirty(false);
    if (mayMove) movedAddress = address;
    activity(toast, true);
    let moved = false;
    try {
      const status = await refreshStatus();
      moved = showMoved(movedAddress || status.address || "");
    } catch (error) {
      if (mayMove) {
        moved = showMoved(address);
        if (!moved && !address) {
          ui.movedMessage.textContent = t("movedUnknown");
          ui.movedLink.hidden = true;
          ui.moved.hidden = false;
          moved = true;
        }
        if (!moved) showError(error, true);
      } else showError(error, true);
    }
    try {
      if (!moved && after) await after();
    } catch (error) {
      showError(error, true);
    } finally {
      setBusy(false);
    }
  }

  function activity(key, success = false) {
    clearTimeout(toastTimer);
    ui.activity.textContent = key ? t(key) : "";
    ui.activity.classList.toggle("success", success);
    if (success) toastTimer = setTimeout(() => { ui.activity.textContent = ""; }, 6000);
  }

  function clearErrors() {
    find("#request-error").hidden = true;
    find("#request-error-text").textContent = "";
    ui.unreachable.hidden = true;
    requestUnreachable = false;
  }

  function showError(error, persistent = false) {
    if (error instanceof TypeError || error.name === "AbortError") {
      requestUnreachable = requestUnreachable || persistent;
      ui.unreachable.hidden = !ui.moved.hidden && ui.movedLink.hidden;
      ui.statusDot.className = "status-dot unknown";
      ui.statusLabel.textContent = t("checking");
    } else {
      const message = find("#request-error-text");
      if (message) {
        message.textContent = error.message || t("requestFailed");
        find("#request-error").hidden = false;
      }
    }
  }

  function addressURL(address) {
    try {
      const url = new URL("http:" + "//" + address);
      return url.username || url.password || url.pathname !== "/" || url.search || url.hash ? null : url;
    } catch {
      return null;
    }
  }

  function sameAddress(address) {
    const listener = addressURL(address);
    const origin = addressURL(location.host);
    if (!listener || !origin) return false;
    const host = url => url.hostname === "localhost" ? "127.0.0.1" : url.hostname;
    return (listener.port || "80") === (origin.port || "80")
      && (host(listener) === "127.0.0.1" || host(listener) === host(origin));
  }

  function showMoved(address) {
    const url = addressURL(address);
    if (!url || sameAddress(address)) return false;
    if (["en", "zh"].includes(override)) url.searchParams.set("lang", override);
    ui.movedMessage.textContent = t("movedTo");
    ui.movedLink.hidden = false;
    ui.movedLink.href = url.href;
    ui.movedLink.textContent = address;
    ui.moved.hidden = false;
    return true;
  }

  function submittedAddress(address) {
    if (!address.port) return "";
    let host = address.host || "127.0.0.1";
    if (["0.0.0.0", "::", "[::]"].includes(host)) host = "127.0.0.1";
    if (host.includes(":") && !host.startsWith("[")) host = "[" + host + "]";
    return host + ":" + address.port;
  }

  function renderStatus(status) {
    const kind = status.running === true ? "running" : status.running === false ? "stopped" : "unknown";
    ui.statusDot.className = "status-dot " + kind;
    ui.statusLabel.textContent = t(kind === "unknown" ? "checking" : kind);
    ui.statusError.textContent = status.error || "";
    const url = addressURL(status.address || "");
    ui.statusAddress.hidden = !url;
    if (url) {
      ui.statusAddress.href = url.href;
      ui.statusAddress.textContent = status.address;
      if (!showMoved(movedAddress || status.address)) ui.moved.hidden = true;
    }
    ui.statusRules.replaceChildren(...list(status.rules).map(rule => {
      const chip = clone("rule-status");
      const state = rule.running ? "running" : rule.attempt > 0 ? "retrying" : "stopped";
      chip.classList.add(state);
      chip.href = ruleRoute(rule.name);
      chip.title = text(rule.error);
      find(".chip-label", chip).textContent = rule.name + " \u00b7 " + rule.address;
      find(".chip-state", chip).textContent = t(state, { attempt: rule.attempt });
      find(".remote-marker", chip).hidden = !rule.remote;
      if (busy) chip.setAttribute("aria-disabled", "true");
      return chip;
    }));
    ui.unreachable.hidden = !requestUnreachable;
  }

  function refreshStatus() {
    if (statusRequest) return statusRequest;
    statusRequest = api("/status").then(status => {
      renderStatus(status);
      return status;
    }).finally(() => { statusRequest = null; });
    return statusRequest;
  }

  function chainSummary(way) {
    if (!way.length) return t("direct");
    const hops = way.map((node, index) => t("hop", { number: index + 1 }));
    return [t("chainLocal"), ...hops.reverse(), t("chainTarget")].join(" \u2192 ");
  }

  function selectRuleTab(tab, selected) {
    tab.setAttribute("aria-selected", String(selected));
    tab.tabIndex = selected ? 0 : -1;
    if (selected) find("#rule-panel").setAttribute("aria-labelledby", tab.id);
  }

  function bindRuleTabs(strip) {
    strip.addEventListener("keydown", event => {
      if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key) || busy) return;
      event.preventDefault();
      const tabs = all("[role=tab]", strip);
      const position = tabs.indexOf(document.activeElement);
      const next = event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1
        : (position + (event.key === "ArrowRight" ? 1 : -1) + tabs.length) % tabs.length;
      tabs[next].focus();
      tabs[next].click();
    });
  }

  function createHopEditor({ container, model, idPrefix, roles: { first, last } }) {
    const hops = find(".hops", container);
    const add = find(".add-hop", container);
    const summary = find(".chain-summary", container);
    add.id = idPrefix + "-add-hop";

    function readModel() {
      all(".hop", hops).forEach((hop, index) => {
        model[index].lb = all(".proxy-url", hop).map(input => input.value);
      });
    }

    function read() {
      readModel();
      return model.map(node => ({ lb: cleanLines(node.lb) }));
    }

    function change(update) {
      if (busy) return;
      readModel();
      const focus = update();
      setDirty(true);
      render();
      if (focus) find(focus, container)?.focus();
    }

    function render() {
      hops.replaceChildren(...model.map(renderHop));
      if (summary) summary.textContent = chainSummary(model);
    }

    function renderURL(value, hopIndex, urlIndex) {
      const row = clone("url");
      const input = find("input", row);
      input.id = idPrefix + "-url-" + hopIndex + "-" + urlIndex;
      input.value = value;
      find("label", row).htmlFor = input.id;
      find(".build-url", row).addEventListener("click", () => openURLBuilder(input));
      find(".remove-url", row).addEventListener("click", () => change(() => {
        const urls = model[hopIndex].lb;
        urls.splice(urlIndex, 1);
        return urls.length ? "#" + idPrefix + "-url-" + hopIndex + "-" + Math.min(urlIndex, urls.length - 1)
          : "#" + idPrefix + "-hop-" + hopIndex + " .add-url";
      }));
      return row;
    }

    function renderHop(node, hopIndex) {
      const hop = clone("hop");
      hop.id = idPrefix + "-hop-" + hopIndex;
      const title = find(".hop-title", hop);
      title.id = hop.id + "-title";
      const labels = [t("hop", { number: hopIndex + 1 })];
      if (hopIndex === 0) labels.push(t(first));
      if (hopIndex === model.length - 1) labels.push(t(last));
      title.textContent = labels.join(" \u00b7 ");
      hop.setAttribute("aria-labelledby", title.id);
      const rows = find(".url-rows", hop);
      node.lb.forEach((url, urlIndex) => rows.append(renderURL(url, hopIndex, urlIndex)));
      const move = offset => change(() => {
        const target = hopIndex + offset;
        [model[hopIndex], model[target]] = [model[target], model[hopIndex]];
        return "#" + idPrefix + "-hop-" + target + " .add-url";
      });
      find(".hop-up", hop).disabled = hopIndex === 0;
      find(".hop-down", hop).disabled = hopIndex === model.length - 1;
      find(".hop-up", hop).addEventListener("click", () => move(-1));
      find(".hop-down", hop).addEventListener("click", () => move(1));
      find(".delete-hop", hop).addEventListener("click", () => change(() => {
        model.splice(hopIndex, 1);
        return "#" + add.id;
      }));
      find(".add-url", hop).addEventListener("click", () => change(() => {
        node.lb.push("");
        return "#" + idPrefix + "-url-" + hopIndex + "-" + (node.lb.length - 1);
      }));
      return hop;
    }

    add.addEventListener("click", () => change(() => {
      model.push({ lb: [""] });
      return "#" + idPrefix + "-url-" + (model.length - 1) + "-0";
    }));
    return { render, read };
  }

  function updateBuilderPreview() {
    const layout = builderLayouts[ui.builderProtocol.selectedIndex];
    if (!layout) return;
    const values = Object.fromEntries(all("[name]", ui.builderFields).map(input => [input.name, input.value]));
    const enc = encodeURIComponent;
    const shadowsocks = layout.name === "shadowsocks";
    const username = values.username || "";
    const password = values.password || "";
    const userinfo = shadowsocks ? enc(values.encrypto) + ":" + enc(password) + "@"
      : username || password ? enc(username) + (password ? ":" + enc(password) : "") + "@" : "";
    let host = values.host.trim();
    if (host.includes(":") && !(host.startsWith("[") && host.endsWith("]"))) host = "[" + host + "]";
    const port = values.port || text(layout.inputs.find(field => field.name === "port")?.value);
    const scheme = layout.inputs.find(field => field.kind === "span").value;
    let url = scheme + userinfo + host + ":" + port;
    if (layout.name === "ssh" && values.identity) url += "?identity_file=" + enc(values.identity);
    ui.builderPreview.textContent = url;
    ui.builderUse.disabled = !host || !/^\d+$/.test(port) || (shadowsocks && (!values.encrypto || !password));
    ui.builderHint.textContent = t(shadowsocks ? "builderShadowsocksHint" : "builderHint");
    ui.builderHint.hidden = !ui.builderUse.disabled;
  }

  function renderBuilderFields(values = {}) {
    const layout = builderLayouts[ui.builderProtocol.selectedIndex];
    ui.builderFields.replaceChildren(...layout.inputs.filter(field => field.kind !== "span").map(field => {
      const row = clone("builder-field");
      const input = clone(field.kind === "select" ? "builder-select" : "builder-input");
      input.id = "builder-" + field.name;
      input.name = field.name;
      input.required = !field.option && !(field.name === "port" && field.value);
      find("label", row).htmlFor = input.id;
      find(".builder-field-name", row).textContent = t("field." + field.name);
      find(".optional", row).hidden = !field.option;
      if (field.kind === "select") {
        input.replaceChildren(...field.items.map(value => new Option(value, value)));
      } else {
        input.type = field.kind === "password" ? "password" : "text";
        if (field.kind === "file") input.placeholder = "~/.ssh/id_ed25519";
        if (field.name === "port") {
          input.inputMode = "numeric";
          input.pattern = "[0-9]+";
          input.placeholder = text(field.value);
        }
      }
      input.value = text(values[field.name] ?? field.value);
      row.append(input);
      return row;
    }));
    updateBuilderPreview();
  }

  async function openURLBuilder(input) {
    if (busy || ui.builder.open) return;
    builderTarget = input;
    ui.builderFields.replaceChildren();
    ui.builderPreview.textContent = "";
    ui.builderProtocol.replaceChildren();
    ui.builderProtocol.disabled = true;
    ui.builderUse.disabled = true;
    ui.builderHint.textContent = t("builderLoading");
    ui.builderHint.hidden = false;
    ui.builder.showModal();
    try {
      if (!builderRequest) {
        builderRequest = fetch("/data/proxy_layouts.json", { headers: { Accept: "application/json" } })
          .then(response => {
            if (!response.ok) throw new Error(t("builderLoadFailed"));
            return response.json();
          }).catch(error => {
            builderRequest = null;
            throw error;
          });
      }
      builderLayouts = await builderRequest;
      if (!ui.builder.open || builderTarget !== input) return;
      ui.builderProtocol.replaceChildren(...builderLayouts.map(layout => new Option(layout.name, layout.name)));
      let values = {};
      try {
        const url = new URL(input.value);
        const protocol = url.protocol.slice(0, -1);
        const aliases = { ss: "shadowsocks" };
        const layout = builderLayouts.find(layout => layout.name === (aliases[protocol] || protocol));
        if (layout) {
          const decode = value => {
            try { return decodeURIComponent(value); } catch { return value; }
          };
          ui.builderProtocol.value = layout.name;
          values = {
            username: decode(url.username), password: decode(url.password),
            host: url.hostname, port: url.port || undefined,
            identity: url.searchParams.get("identity_file") ?? "", encrypto: decode(url.username)
          };
        }
      } catch {}
      ui.builderProtocol.disabled = false;
      renderBuilderFields(values);
      find("input, select", ui.builderFields)?.focus();
    } catch {
      if (ui.builder.open && builderTarget === input) ui.builderHint.textContent = t("builderLoadFailed");
    }
  }

  function bindEvents() {
    document.addEventListener("click", event => {
      const link = event.target.closest("a[href^='#/']");
      if (!link || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      event.preventDefault();
      const hash = link.getAttribute("href");
      if (hash !== activeHash) navigate(hash);
    });
    window.addEventListener("hashchange", hashChanged);
    ["input", "change"].forEach(event => {
      ui.builderFields.addEventListener(event, updateBuilderPreview);
    });
    ui.builderProtocol.addEventListener("change", () => renderBuilderFields());
    ui.builderCancel.addEventListener("click", () => ui.builder.close());
    ui.builder.addEventListener("close", () => {
      const input = builderTarget;
      builderTarget = null;
      input?.focus();
    });
    ui.builderForm.addEventListener("submit", event => {
      event.preventDefault();
      updateBuilderPreview();
      if (ui.builderUse.disabled || !builderTarget || busy) return;
      builderTarget.value = ui.builderPreview.textContent;
      builderTarget.dispatchEvent(new Event("input", { bubbles: true }));
      ui.builder.close();
    });
    document.addEventListener("keydown", event => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
        event.preventDefault();
        save();
      }
    });
    window.addEventListener("beforeunload", event => {
      if (!dirty) return;
      event.preventDefault();
      event.returnValue = "";
    });
  }

  localize(document);
  bindEvents();
  navigate(location.hash, { replace: true });
  refreshStatus().catch(showError);
  setInterval(() => {
    if (!busy) refreshStatus().catch(showError);
  }, 10000);
})();