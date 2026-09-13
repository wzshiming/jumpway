"use strict";

(() => {
  const STR = {
    en: {
      appName: "JumpWay",
      checking: "Status unknown",
      running: "Running",
      stopped: "Stopped",
      reload: "Reload from disk",
      save: "Save",
      saveApply: "Save & Apply",
      unsaved: "Unsaved changes",
      discardChanges: "Discard unsaved changes?",
      navigation: "Configuration pages",
      contextTabs: "Context editors",
      yaml: "Advanced YAML",
      listen: "Listen address",
      host: "Host",
      port: "Port",
      listenHint: "The web UI moves with the listen address. An empty host uses 127.0.0.1; port 0 chooses an available port.",
      contexts: "Contexts",
      contextName: "Context name",
      current: "Current",
      currentBadge: "Current",
      switch: "Switch",
      switched: "Switched.",
      edit: "Edit",
      deleteContext: "Delete",
      confirmDelete: "Delete context \"{name}\"?",
      deleted: "Context deleted.",
      newContext: "New context",
      noContexts: "No contexts yet. Connections go direct.",
      noHops: "No hops: this context connects directly.",
      hop: "Hop {number}",
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
      recoveryHint: "The proxy may have stopped. Use the tray's Edit Config to check ~/.jumpway/config.yaml, then Reload Config to restart it.",
      requestFailed: "The request could not be completed.",
      invalidResponse: "JumpWay returned an unreadable response. Try reloading from disk."
    },
    zh: {
      appName: "JumpWay",
      checking: "状态未知",
      running: "运行中",
      stopped: "已停止",
      reload: "从磁盘重新加载",
      save: "保存",
      saveApply: "保存并应用",
      unsaved: "未保存的修改",
      discardChanges: "放弃未保存的修改吗？",
      navigation: "配置页面",
      contextTabs: "上下文编辑器",
      yaml: "高级 YAML",
      listen: "监听地址",
      host: "主机",
      port: "端口",
      listenHint: "网页配置的地址会随监听地址改变。主机留空时使用 127.0.0.1；端口为 0 时自动分配可用端口。",
      contexts: "上下文管理",
      contextName: "上下文名称",
      current: "当前上下文",
      currentBadge: "当前",
      switch: "切换",
      switched: "已切换。",
      edit: "编辑",
      deleteContext: "删除",
      confirmDelete: "删除上下文“{name}”吗？",
      deleted: "已删除上下文。",
      newContext: "新建上下文",
      noContexts: "尚无上下文，连接将直接访问，不走代理。",
      noHops: "没有跳板节点：此上下文使用直接连接。",
      hop: "跳板节点 {number}",
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
      recoveryHint: "代理可能已停止。请通过托盘菜单的“编辑配置”检查 ~/.jumpway/config.yaml，再点击“重新加载配置”启动代理。",
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

  function normalize(context) {
    return {
      name: text(context.name),
      way: list(context.way).map(node => ({
        lb: (typeof node === "string" ? node.split("|")
          : Array.isArray(node) ? node : list(node?.lb)).map(text)
      }))
    };
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
    builder: find("#url-builder"), builderForm: find("#builder-form"),
    builderProtocol: find("#builder-protocol"), builderFields: find("#builder-fields"),
    builderPreview: find("#builder-preview"), builderHint: find("#builder-hint"),
    builderUse: find("#builder-use"), builderCancel: find("#builder-cancel")
  };
  let page = null;
  let activeHash = "";
  let newContext = false;
  let dirty = false;
  let busy = false;
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
    const panel = find("#context-panel");
    if (panel) panel.setAttribute("aria-busy", String(value));
    all("#context-tabs button, #retry").forEach(button => { button.disabled = value; });
    all("a[href^='#/']").forEach(link => {
      if (value) link.setAttribute("aria-disabled", "true");
      else link.removeAttribute("aria-disabled");
    });
  }

  function routeFor(hash) {
    const pages = { "#/": "current", "#/contexts": "contexts", "#/proxy": "proxy", "#/no-proxy": "no-proxy", "#/yaml": "yaml" };
    if (Object.prototype.hasOwnProperty.call(pages, hash)) return { hash, kind: pages[hash], name: null };
    const match = /^#\/contexts\/([^/]+)$/.exec(hash);
    if (match) {
      try { return { hash, kind: "contexts", name: decodeURIComponent(match[1]) }; } catch {}
    }
    return { hash: "#/", kind: "current", name: null };
  }

  const contextRoute = name => "#/contexts/" + encodeURIComponent(name);
  const contextPath = name => "/contexts/" + encodeURIComponent(name);

  function mayLeave() {
    return !busy && (!dirty || confirm(t("discardChanges")));
  }

  async function navigate(hash, { replace = false, create = false, confirmed = false, notify = "" } = {}) {
    if (!confirmed && !mayLeave()) return;
    const route = routeFor(hash);
    history[replace ? "replaceState" : "pushState"](null, "", route.hash);
    newContext = create;
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
    newContext = false;
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
    const title = { current: "current", contexts: "contexts", proxy: "listen", "no-proxy": "noProxy", yaml: "yaml" };
    find("#page-heading").textContent = t(title[route.kind]);
    document.title = t(title[route.kind]) + " | " + t("appName");
    all("[data-page]", ui.nav).forEach(link => {
      if (link.dataset.page === route.kind) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });
    find("#retry").addEventListener("click", () => navigate(activeHash, { replace: true, create: newContext }));
    clearErrors();
    setBusy(true);
    activity("loading");
    try {
      const loaders = { current: loadCurrent, contexts: loadContexts, proxy: loadProxy, "no-proxy": loadNoProxy, yaml: loadYAML };
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
        const target = route.kind === "contexts" ? find("#context-tabs [aria-selected=true]") : null;
        (target || find("#page-heading")).focus();
      }
    }
  }

  async function loadCurrent() {
    const [current, contexts] = await Promise.all([api("/current-context"), api("/contexts")]);
    const entries = list(contexts).map(normalize);
    const form = clone("current");
    find("#page-content").append(form);
    const active = entries.find(context => context.name === current.name);
    if (active) {
      find("#active-context").hidden = false;
      find("#active-name").textContent = active.name;
      find("#active-chain").textContent = chainSummary(active);
    }
    find("#no-contexts").hidden = entries.length !== 0;
    find("#current-options").hidden = entries.length === 0;
    entries.forEach(context => {
      const row = clone("choice");
      const selected = context.name === current.name;
      row.classList.toggle("is-current", selected);
      find(".choice-name", row).textContent = context.name;
      find(".choice-chain", row).textContent = chainSummary(context);
      find(".current-badge", row).hidden = !selected;
      const radio = find("input", row);
      radio.value = context.name;
      radio.checked = selected;
      find(".edit-context", row).href = contextRoute(context.name);
      find("#context-choices").append(row);
    });
    form.addEventListener("change", () => {
      const changed = find("input[name=current]:checked")?.value !== current.name;
      find("#switch").disabled = !changed;
      setDirty(changed);
    });
    page.save = async () => {
      const name = find("input[name=current]:checked")?.value;
      if (name === undefined || name === current.name) return;
      await write("/current-context", { method: "PUT", body: { name } }, {
        toast: "switched", after: () => navigate("#/", { replace: true, confirmed: true, notify: "switched" })
      });
    };
    form.addEventListener("submit", event => { event.preventDefault(); save(); });
  }

  async function loadContexts(route) {
    const [contexts, current] = await Promise.all([api("/contexts"), api("/current-context")]);
    const entries = list(contexts);
    const originalName = route.name !== null ? route.name : newContext ? null : entries[0]?.name ?? null;
    newContext = originalName === null;
    const view = clone("contexts");
    find("#page-content").append(view);
    const tabs = find("#context-tabs");
    entries.forEach((context, index) => {
      const tab = clone("context-tab");
      tab.id = "context-tab-" + index;
      find(".tab-name", tab).textContent = context.name;
      find(".current-badge", tab).hidden = context.name !== current.name;
      selectContextTab(tab, context.name === originalName);
      tab.addEventListener("click", () => {
        if (context.name !== originalName) navigate(contextRoute(context.name));
      });
      tabs.append(tab);
    });
    const add = clone("new-tab");
    selectContextTab(add, newContext);
    add.addEventListener("click", () => {
      if (!newContext) navigate("#/contexts", { create: true });
    });
    tabs.append(add);
    bindContextTabs(tabs);
    setBusy(true);
    const context = originalName === null ? { name: "", way: [{ lb: [""] }] }
      : normalize(await api(contextPath(originalName)));
    page.context = context;
    find("#context-name").value = context.name;
    find("#new-context-heading").hidden = !newContext;
    find("#delete-context").hidden = newContext;
    renderHops();
    find("#add-hop").addEventListener("click", () => changeContext(() => {
      context.way.push({ lb: [""] });
      return "#url-" + (context.way.length - 1) + "-0";
    }));
    bindEditor(find("#context-form"), async () => {
      readContext();
      const body = { name: context.name.trim(), way: context.way.map(node => ({ lb: cleanLines(node.lb) })) };
      await write(originalName === null ? "/contexts" : contextPath(originalName), {
        method: originalName === null ? "POST" : "PUT", body
      }, { after: () => navigate(contextRoute(body.name), { replace: true, confirmed: true, notify: "saved" }) });
    });
    find("#delete-context").addEventListener("click", async () => {
      if (busy || !confirm(t("confirmDelete", { name: originalName }))) return;
      if (dirty && !confirm(t("discardChanges"))) return;
      await write(contextPath(originalName), { method: "DELETE" }, {
        toast: "deleted", after: () => navigate("#/contexts", { replace: true, confirmed: true, notify: "deleted" })
      });
    });
  }

  async function loadProxy() {
    const proxy = await api("/proxy");
    const form = clone("proxy");
    find("#page-content").append(form);
    find("#proxy-host").value = text(proxy.host);
    find("#proxy-port").value = proxy.port ?? 0;
    bindEditor(form, () => {
      const body = { host: find("#proxy-host").value.trim(), port: Number(find("#proxy-port").value) };
      return write("/proxy", { method: "PUT", body }, { mayMove: true, address: submittedAddress(body) });
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

  function save() {
    if (!busy && !ui.builder.open) page?.save?.();
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
    activity(toast, true);
    let moved = false;
    try {
      const status = await refreshStatus();
      moved = Boolean(status.address && !sameAddress(status.address));
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

  function submittedAddress(proxy) {
    if (!proxy.port) return "";
    let host = proxy.host || "127.0.0.1";
    if (["0.0.0.0", "::", "[::]"].includes(host)) host = "127.0.0.1";
    if (host.includes(":") && !host.startsWith("[")) host = "[" + host + "]";
    return host + ":" + proxy.port;
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
      if (sameAddress(status.address)) ui.moved.hidden = true;
      else showMoved(status.address);
    }
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

  function chainSummary(context) {
    return context.way.length ? context.way.map((node, index) =>
      t("hop", { number: index + 1 }) + ": " + node.lb.join(", ")).join(" \u2192 ") : t("noHops");
  }

  function selectContextTab(tab, selected) {
    tab.setAttribute("aria-selected", String(selected));
    tab.tabIndex = selected ? 0 : -1;
    if (selected) find("#context-panel").setAttribute("aria-labelledby", tab.id);
  }

  function bindContextTabs(strip) {
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

  function readContext() {
    page.context.name = find("#context-name").value;
    all(".hop", find("#hops")).forEach((hop, index) => {
      page.context.way[index].lb = all(".proxy-url", hop).map(input => input.value);
    });
  }

  function changeContext(change) {
    if (busy) return;
    readContext();
    const focus = change();
    setDirty(true);
    renderHops();
    if (focus) find(focus)?.focus();
  }

  function renderHops() {
    const hops = find("#hops");
    hops.replaceChildren(...page.context.way.map(renderHop));
    if (!page.context.way.length) {
      const empty = document.createElement("p");
      empty.className = "hint";
      empty.textContent = t("noHops");
      hops.append(empty);
    }
  }

  function renderURL(value, hopIndex, urlIndex) {
    const row = clone("url");
    const input = find("input", row);
    input.id = "url-" + hopIndex + "-" + urlIndex;
    input.value = value;
    find("label", row).htmlFor = input.id;
    find(".build-url", row).addEventListener("click", () => openURLBuilder(input));
    find(".remove-url", row).addEventListener("click", () => changeContext(() => {
      const urls = page.context.way[hopIndex].lb;
      urls.splice(urlIndex, 1);
      return urls.length ? "#url-" + hopIndex + "-" + Math.min(urlIndex, urls.length - 1)
        : "#hop-" + hopIndex + " .add-url";
    }));
    return row;
  }

  function renderHop(node, hopIndex) {
    const hop = clone("hop");
    const way = page.context.way;
    hop.id = "hop-" + hopIndex;
    const title = find(".hop-title", hop);
    title.id = hop.id + "-title";
    title.textContent = t("hop", { number: hopIndex + 1 });
    hop.setAttribute("aria-labelledby", title.id);
    const rows = find(".url-rows", hop);
    node.lb.forEach((url, urlIndex) => rows.append(renderURL(url, hopIndex, urlIndex)));
    const move = offset => changeContext(() => {
      const target = hopIndex + offset;
      [way[hopIndex], way[target]] = [way[target], way[hopIndex]];
      return "#hop-" + target + " .add-url";
    });
    find(".hop-up", hop).disabled = hopIndex === 0;
    find(".hop-down", hop).disabled = hopIndex === way.length - 1;
    find(".hop-up", hop).addEventListener("click", () => move(-1));
    find(".hop-down", hop).addEventListener("click", () => move(1));
    find(".delete-hop", hop).addEventListener("click", () => changeContext(() => {
      way.splice(hopIndex, 1);
      return "#add-hop";
    }));
    find(".add-url", hop).addEventListener("click", () => changeContext(() => {
      node.lb.push("");
      return "#url-" + hopIndex + "-" + (node.lb.length - 1);
    }));
    return hop;
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