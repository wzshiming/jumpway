"use strict";

(() => {
  // Mirrors configs.SavedPrefix: the config was written, only the reload failed.
  const SAVED_PREFIX = "saved, but ";
  const STR = {
    en: {
      appName: "JumpWay",
      checking: "Status unknown",
      running: "Running",
      stopped: "Stopped",
      retrying: "Retrying ({attempt})",
      remote: "remote",
      forward: "forward",
      reload: "Reload from disk",
      saveApply: "Save & Apply",
      unsaved: "Unsaved changes",
      discardChanges: "Discard unsaved changes?",
      ruleTabs: "Rules",
      yaml: "Advanced YAML",
      settings: "Settings",
      webUI: "Web UI",
      webUIHint: "Address of this page and the REST API. Only local addresses make sense here.",
      host: "Host",
      port: "Port",
      invalidPort: "Port must be a whole number between 0 and 65535.",
      invalidForwardPort: "Port must be a whole number between 1 and 65535.",
      rule: "Rule name",
      enabled: "Enabled",
      entry: "Entry",
      exit: "Exit",
      mode: "Mode",
      proxy: "Proxy",
      portForward: "Port forward",
      targetHost: "Target host",
      targetPort: "Target port",
      onExitNode: "on the exit node",
      listenThrough: "Listen through",
      exitChain: "Exit chain",
      username: "Username",
      password: "Password",
      credentialsHint: "Optional credentials for HTTP Basic / SOCKS5 clients. Usernames must not contain \":\".",
      credentialsProxyOnly: "Credentials apply to proxy rules only",
      listenThroughHint: "Leave empty to open the port on this machine. Otherwise the first hop binds host:port on its side (SSH remote forwarding; the remote sshd binds loopback unless GatewayPorts is enabled) and the last hop is dialed from this machine. Only ssh://, cmd: and nc hops can bind.",
      deleteRule: "Delete",
      confirmDeleteRule: "Delete rule \"{name}\"?",
      deleted: "Rule deleted.",
      newRule: "New rule",
      noRules: "No rules yet.",
      direct: "direct",
      connectionsShort: "{active}/{total} conns",
      latencyPair: "{last} (avg {average})",
      failedCount: "{count} failed",
      stats: "Statistics",
      rateUp: "Upload rate",
      rateDown: "Download rate",
      totalUp: "Total upload",
      totalDown: "Total download",
      rateWindow: "Last 1 s",
      statsLifetime: "Since last reset",
      connections: "Connections",
      activeConns: "Active connections",
      totalConns: "Total connections",
      connectionsOrder: "Active / total",
      latency: "Latency",
      avgLatency: "Average",
      latencyOrder: "Last / avg",
      dialFailures: "Dial failures",
      dialAttempts: "dial attempts",
      lastActive: "Last activity",
      secondsAgo: "{count} s ago",
      minutesAgo: "{count} min ago",
      hoursAgo: "{count} h ago",
      daysAgo: "{count} d ago",
      via: "Via",
      viaHop: "via {parent}",
      saveStats: "Save to see live stats",
      currentConnections: "Connections",
      client: "Client",
      target: "Target",
      duration: "Duration",
      totalShort: "total",
      disconnect: "Disconnect",
      disconnected: "Connection closed.",
      noConnections: "No current connections",
      showingConnections: "Showing {shown} of {total}",
      state: "State",
      address: "Address",
      statsSince: "Since {time}",
      resetStats: "Reset statistics",
      resetStatsConfirm: "Reset statistics for all rules?",
      statsReset: "Statistics reset.",
      prometheus: "Prometheus metrics",
      hop: "Hop {number}",
      hopExit: "exit node",
      hopBinds: "binds the port",
      hopDialed: "dialed from this machine",
      hopsHint: "Hop 1 is the exit node; the last hop is dialed from this machine. Leave empty to connect directly from this machine.",
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
      savedWithErrors: "Saved, but applying it reported errors. The affected rules keep retrying.",
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
      forward: "转发",
      reload: "从磁盘重新加载",
      saveApply: "保存并应用",
      unsaved: "未保存的修改",
      discardChanges: "放弃未保存的修改吗？",
      ruleTabs: "规则",
      yaml: "高级 YAML",
      settings: "设置",
      webUI: "网页配置",
      webUIHint: "本页面和 REST API 的地址。此处应使用本机地址。",
      host: "主机",
      port: "端口",
      invalidPort: "端口必须是 0 到 65535 之间的整数。",
      invalidForwardPort: "端口必须是 1 到 65535 之间的整数。",
      rule: "规则名称",
      enabled: "启用",
      entry: "入口",
      exit: "出口",
      mode: "模式",
      proxy: "代理",
      portForward: "端口转发",
      targetHost: "目标主机",
      targetPort: "目标端口",
      onExitNode: "位于出口节点",
      listenThrough: "经由监听",
      exitChain: "出口链路",
      username: "用户名",
      password: "密码",
      credentialsHint: "HTTP Basic / SOCKS5 客户端的可选认证信息。用户名不能包含冒号（:）。",
      credentialsProxyOnly: "凭据仅用于代理规则",
      listenThroughHint: "留空时在本机打开端口；否则由第一个节点在其所在机器上绑定主机和端口（SSH 远程端口转发；远端 sshd 默认仅绑定回环地址，启用 GatewayPorts 后才能绑定其他地址），最后一个节点由本机直接连接。只有 ssh://、cmd: 和 nc 节点支持监听。",
      deleteRule: "删除",
      confirmDeleteRule: "删除规则“{name}”吗？",
      deleted: "已删除规则。",
      newRule: "新建规则",
      noRules: "尚无规则。",
      direct: "直连",
      connectionsShort: "连接 {active}/{total}",
      latencyPair: "{last}（平均 {average}）",
      failedCount: "失败 {count}",
      stats: "统计",
      rateUp: "上传速率",
      rateDown: "下载速率",
      totalUp: "累计上传",
      totalDown: "累计下载",
      rateWindow: "最近 1 秒",
      statsLifetime: "自上次重置以来",
      connections: "连接数",
      activeConns: "当前连接",
      totalConns: "累计连接",
      connectionsOrder: "当前 / 累计连接",
      latency: "延迟",
      avgLatency: "平均",
      latencyOrder: "最近 / 平均",
      dialFailures: "连接失败",
      dialAttempts: "次连接尝试",
      lastActive: "最近活动",
      secondsAgo: "{count} 秒前",
      minutesAgo: "{count} 分钟前",
      hoursAgo: "{count} 小时前",
      daysAgo: "{count} 天前",
      via: "上一级",
      viaHop: "上一级：{parent}",
      saveStats: "保存后查看实时统计",
      currentConnections: "当前连接",
      client: "客户端",
      target: "目标",
      duration: "持续时间",
      totalShort: "累计",
      disconnect: "断开",
      disconnected: "已断开连接。",
      noConnections: "当前没有连接",
      showingConnections: "显示 {total} 个连接中的 {shown} 个",
      state: "状态",
      address: "地址",
      statsSince: "统计起始：{time}",
      resetStats: "重置统计",
      resetStatsConfirm: "重置所有规则的统计吗？",
      statsReset: "已重置统计。",
      prometheus: "Prometheus 指标",
      hop: "跳板节点 {number}",
      hopExit: "出口节点",
      hopBinds: "绑定端口",
      hopDialed: "由本机连接",
      hopsHint: "节点 1 是出口节点，最后一个节点由本机连接。留空时由本机直接连接目标。",
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
      savedWithErrors: "已保存，但应用时出现错误。受影响的规则会持续重试。",
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

  async function api(route, { method = "GET", body, base = "/apis/configs" } = {}) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 15000);
    try {
      const response = await fetch(base + route, {
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
  const countFormatter = new Intl.NumberFormat(language, { maximumFractionDigits: 0 });
  const formatCount = value => countFormatter.format(value || 0);

  function formatBytes(value) {
    const bytes = Math.max(0, Number(value) || 0);
    const unit = Math.max(0, Math.min(4, Math.floor(Math.log2(bytes || 1) / 10)));
    return (bytes / 1024 ** unit).toFixed(unit ? 1 : 0) + " " + ["B", "KB", "MB", "GB", "TB"][unit];
  }

  const formatRate = value => formatBytes(value) + "/s";

  function formatLatency(stats = {}, key = "latency_ms") {
    return (stats.dials || 0) <= (stats.dial_failures || 0)
      ? "\u2014" : Number(stats[key] || 0).toFixed(1) + " ms";
  }

  function renderStats(element, stats) {
    element.title = "";
    if (!stats) {
      element.textContent = "\u2014";
      return;
    }
    const parts = ["\u2191 " + formatRate(stats.rate_up) + " \u2193 " + formatRate(stats.rate_down),
      t("connectionsShort", { active: formatCount(stats.active), total: formatCount(stats.total) }),
      t("latencyPair", { last: formatLatency(stats), average: formatLatency(stats, "avg_latency_ms") })];
    if (stats.dial_failures) parts.push(t("failedCount", { count: formatCount(stats.dial_failures) }));
    element.textContent = parts.join(" \u00b7 ");
    element.title = t("totalUp") + ": " + formatBytes(stats.up) + " \u00b7 "
      + t("totalDown") + ": " + formatBytes(stats.down) + " \u00b7 "
      + formatCount(stats.dials) + " " + t("dialAttempts");
  }

  function formatLastActive(value) {
    const timestamp = Date.parse(value);
    if (!Number.isFinite(timestamp)) return "\u2014";
    const seconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
    const [key, unit] = seconds < 60 ? ["secondsAgo", 1] : seconds < 3600 ? ["minutesAgo", 60]
      : seconds < 86400 ? ["hoursAgo", 3600] : ["daysAgo", 86400];
    return t(key, { count: formatCount(Math.floor(seconds / unit)) });
  }

  function formatDuration(value) {
    const timestamp = Date.parse(value);
    if (!Number.isFinite(timestamp)) return "\u2014";
    const seconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
    if (seconds < 60) return seconds + " s";
    if (seconds < 3600) return Math.floor(seconds / 60) + " min " + seconds % 60 + " s";
    return Math.floor(seconds / 3600) + " h " + Math.floor(seconds % 3600 / 60) + " min";
  }

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
  let statusSnapshot = null;
  let statsRequest = null;
  let statsTimer = null;
  let pendingWarning = null;
  let requestUnreachable = false;
  let builderLayouts = [];
  let builderRequest = null;
  let builderTarget = null;
  let toastTimer;

  function setDirty(value) {
    dirty = value;
    ui.dirty.hidden = !value;
    if (value) page?.updateStats?.();
  }

  function setBusy(value) {
    busy = value;
    ui.main.setAttribute("aria-busy", String(value));
    const fields = find("#page-fields");
    if (fields) fields.disabled = value || !page.loaded;
    const panel = find("#rule-panel");
    if (panel) panel.setAttribute("aria-busy", String(value));
    all("#retry, #reset-stats").forEach(button => { button.disabled = value; });
    all("a[href^='#/']").forEach(link => {
      if (value) link.setAttribute("aria-disabled", "true");
      else link.removeAttribute("aria-disabled");
    });
  }

  function routeFor(hash) {
    if (["#/web-ui", "#/no-proxy"].includes(hash)) hash = "#/settings";
    const pages = { "#/": "rules", "#/rules": "rules", "#/stats": "stats", "#/settings": "settings", "#/yaml": "yaml" };
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
    startStats();
    setDirty(false);
    if (ui.builder.open) ui.builder.close();
    ui.main.replaceChildren(clone("page"));
    ui.main.dataset.page = route.kind;
    find("#retry").addEventListener("click", () => navigate(activeHash, { replace: true, create: newRule }));
    clearErrors();
    setBusy(true);
    activity("loading");
    try {
      let listError;
      const entries = await api("/rules").then(list).catch(error => { listError = error; return []; });
      if (route.kind === "rules") {
        route = { ...route, name: route.name ?? (newRule ? null : entries[0]?.name ?? null) };
        if (!listError) newRule = route.name === null;
      }
      renderNav(entries, route);
      const title = route.kind === "rules" ? route.name ?? t("newRule") : t(route.kind);
      find("#page-heading").textContent = title;
      document.title = title + " | " + t("appName");
      setBusy(true);
      if (listError && route.kind === "rules") throw listError;
      const loaders = { rules: loadRules, stats: loadStats, settings: loadSettings, yaml: loadYAML };
      await loaders[route.kind](route, entries);
      page.loaded = true;
      if (listError) throw listError;
      if (pendingWarning) {
        showError(pendingWarning, true);
        activity("savedWithErrors");
        pendingWarning = null;
      } else activity(notify, Boolean(notify));
    } catch (error) {
      activity("");
      showError(error, true);
      find("#retry").hidden = false;
    } finally {
      setBusy(false);
      startStats();
      if (focusPage) {
        const target = find("[aria-selected=true]", ui.nav);
        (target || find("#page-heading")).focus();
      }
    }
  }

  function renderNav(entries, route) {
    all("[data-rule], #new-rule-tab", ui.nav).forEach(tab => tab.remove());
    const tabs = entries.map((rule, index) => {
      const tab = clone("rule-tab");
      tab.id = "rule-tab-" + index;
      tab.dataset.rule = rule.name;
      tab.href = ruleRoute(rule.name);
      find(".tab-name", tab).textContent = rule.name;
      return tab;
    });
    ui.nav.prepend(...tabs, clone("new-tab"));
    all("[role=tab]", ui.nav).forEach(tab => {
      const selected = route.kind === "rules"
        ? tab.id === "new-rule-tab" ? route.name === null : tab.dataset.rule === route.name
        : tab.dataset.page === route.kind;
      tab.setAttribute("aria-selected", String(selected));
      tab.tabIndex = selected ? 0 : -1;
      if (selected) tab.setAttribute("aria-current", "page");
      else tab.removeAttribute("aria-current");
    });
    if (!find("[aria-selected=true]", ui.nav)) find("[role=tab]", ui.nav).tabIndex = 0;
  }

  async function loadRules(route, entries) {
    const originalName = route.name;
    find("#page-content").append(clone("rules"));
    setBusy(true);
    const rule = originalName === null ? { name: "", listen: { host: "127.0.0.1", port: 0 }, forward: { way: [] } }
      : await api(rulePath(originalName));
    find("#rule-name").value = rule.name;
    find("#rule-enabled").checked = !rule.disabled;
    for (const field of ["host", "port", "username", "password"]) {
      find("#listen-" + field).value = text(rule.listen[field]);
    }
    const forward = rule.forward || {};
    find("#mode-" + (forward.port ? "forward" : "proxy")).checked = true;
    for (const field of ["host", "port"]) find("#target-" + field).value = text(forward[field]);
    find("#no-rules").hidden = entries.length !== 0;
    find("#delete-rule").hidden = newRule;
    const editors = {
      listen: createHopEditor({ container: find("#listen-editor"), model: normalizeWay(rule.listen.way),
        idPrefix: "listen", roles: { first: "hopBinds", last: "hopDialed" } }),
      forward: createHopEditor({ container: find("#exit-editor"), model: normalizeWay(forward.way),
        idPrefix: "exit", roles: { first: "hopExit", last: "hopDialed" }, target: forwardTarget })
    };
    Object.values(editors).forEach(editor => editor.render());
    if (originalName !== null) {
      const strip = clone("rule-stats");
      find("#rule-panel").before(strip);
      find("#connections-slot").append(clone("connections"));
      page.updateStats = () => Object.values(editors).forEach(editor => editor.applyStats());
      page.renderStats = snapshot => {
        const current = list(snapshot.rules).find(entry => entry.name === originalName);
        renderStatFields(strip, current?.stats);
        editors.listen.applyStats(list(current?.listen));
        editors.forward.applyStats(list(current?.forward));
        renderConnections(current);
      };
      page.renderStats({ rules: [] });
    }
    const updateMode = () => {
      const forwarding = find("#mode-forward").checked;
      find("#target-fields").hidden = find("#target-fields").disabled = !forwarding;
      for (const field of ["username", "password"]) find("#listen-" + field).disabled = forwarding;
      find("#credentials-hint").textContent = t(forwarding ? "credentialsProxyOnly" : "credentialsHint");
      editors.forward.updateSummary();
    };
    find("#rule-mode").addEventListener("change", updateMode);
    find("#target-fields").addEventListener("input", editors.forward.updateSummary);
    updateMode();
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

  function readPort(input, minimum = 0) {
    const port = Number(input.value);
    if (!input.value.trim() || !Number.isInteger(port) || port < minimum || port > 65535) {
      throw new Error(t(minimum === 1 ? "invalidForwardPort" : "invalidPort"));
    }
    return port;
  }

  function readRule(editors) {
    const forwarding = find("#mode-forward").checked;
    const listen = { host: find("#listen-host").value.trim(), port: readPort(find("#listen-port")) };
    for (const field of ["username", "password"]) {
      const value = forwarding ? "" : find("#listen-" + field).value;
      if (value || forwarding) listen[field] = value;
    }
    const way = editors.listen.read();
    if (way.length) listen.way = way;
    const forward = { way: editors.forward.read() };
    if (forwarding) {
      forward.port = readPort(find("#target-port"), 1);
      const host = find("#target-host").value.trim();
      if (host) forward.host = host;
    }
    const rule = { name: find("#rule-name").value.trim(), listen, forward };
    if (!find("#rule-enabled").checked) rule.disabled = true;
    return rule;
  }

  function forwardTarget() {
    const port = find("#target-port").value;
    if (!find("#mode-forward").checked || !port) return "";
    let host = find("#target-host").value.trim() || "127.0.0.1";
    if (host.includes(":") && !host.startsWith("[")) host = "[" + host + "]";
    return host + ":" + port;
  }

  async function loadStats(route, entries) {
    const panel = clone("stats");
    find("#page-content").append(panel);
    const rows = new Map(entries.map(rule => {
      const row = clone("stats-row");
      const link = find(".stats-rule", row);
      link.textContent = rule.name;
      link.href = ruleRoute(rule.name);
      find("#stats-rows", panel).append(row);
      return [rule.name, row];
    }));
    find("#stats-empty", panel).hidden = Boolean(rows.size);
    find("#stats-table", panel).hidden = !rows.size;
    page.renderStats = snapshot => {
      const timestamp = Date.parse(snapshot.since);
      find("#stats-since", panel).textContent = t("statsSince", {
        time: Number.isFinite(timestamp) ? new Date(timestamp).toLocaleString(language) : "\u2014"
      });
      const rules = new Map(list(snapshot.rules).map(rule => [rule.name, rule]));
      rows.forEach((row, name) => {
        renderStatFields(row, rules.get(name)?.stats);
      });
    };
    page.renderStatus = status => {
      const rules = new Map(list(status?.rules).map(rule => [rule.name, rule]));
      rows.forEach((row, name) => {
        const rule = rules.get(name);
        const state = rule ? rule.running ? "running" : rule.attempt > 0 ? "retrying" : "stopped" : "unknown";
        const chip = find(".rule-chip", row);
        chip.className = "rule-chip " + state;
        chip.title = text(rule?.error);
        find(".chip-state", chip).textContent = t(state === "unknown" ? "checking" : state, { attempt: rule?.attempt });
        find(".stats-address", row).textContent = rule?.address || "\u2014";
      });
    };
    page.renderStats({ rules: [] });
    page.renderStatus(statusSnapshot);
    refreshStatus().catch(() => {});
    find("#reset-stats", panel).addEventListener("click", async () => {
      if (busy || !confirm(t("resetStatsConfirm"))) return;
      setBusy(true);
      clearErrors();
      try {
        if (statsRequest) await statsRequest;
        await api("", { method: "DELETE", base: "/apis/stats" });
        activity("statsReset", true);
      } catch (error) {
        showError(error, true);
      } finally {
        setBusy(false);
        await refreshStats();
      }
    });
  }

  async function loadSettings() {
    const [address, bypass] = await Promise.all([api("/web-ui"), api("/no-proxy")]);
    const form = clone("settings"), fields = { list: "#no-proxy-list", from_env: "#no-proxy-env", from_file: "#no-proxy-files" };
    find("#page-content").append(form);
    find("#web-ui-host").value = text(address.host);
    find("#web-ui-port").value = address.port ?? 0;
    Object.entries(fields).forEach(([key, selector]) => { find(selector).value = list(bypass[key]).join("\n"); });
    const readBypass = () => Object.fromEntries(Object.entries(fields).map(([key, selector]) => [key, cleanLines(find(selector).value.split(/\r?\n/))]));
    let savedBypass = JSON.stringify(readBypass()), savedAddress = JSON.stringify({ host: text(address.host).trim(), port: address.port ?? 0 });
    bindEditor(form, async () => {
      const body = { host: find("#web-ui-host").value.trim(), port: readPort(find("#web-ui-port")) };
      const bypassBody = readBypass(), nextAddress = JSON.stringify(body);
      const nextBypass = JSON.stringify(bypassBody), addressChanged = nextAddress !== savedAddress;
      let result = true;
      if (nextBypass !== savedBypass) {
        result = await write("/no-proxy", { method: "PUT", body: bypassBody }, { keepDirty: addressChanged });
        if (!result) return;
        savedBypass = nextBypass;
      }
      if (addressChanged) {
        result = await write("/web-ui", { method: "PUT", body }, { mayMove: true, address: submittedAddress(body), warning: result === true ? null : result });
        if (!result) return;
        savedAddress = nextAddress;
      }
      setDirty(false);
      if (result === true) activity("saved", true);
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

  async function write(resource, request, { toast = "saved", after, mayMove = false, address = "", keepDirty = false, warning = null } = {}) {
    setBusy(true);
    clearErrors();
    activity("saving");
    try {
      if (statusRequest) await statusRequest.catch(() => {});
      await api(resource, request);
    } catch (error) {
      // The service prefixes reload failures: the config is on disk, only applying it failed.
      if (!(error.message || "").startsWith(SAVED_PREFIX)) {
        activity(warning ? "savedWithErrors" : "");
        showError(warning ? new Error(warning.message + "\n" + error.message) : error, true);
        setBusy(false);
        return false;
      }
      warning = warning ? new Error(warning.message + "\n" + error.message) : error;
    }
    setDirty(keepDirty);
    if (mayMove) movedAddress = address;
    if (warning) {
      pendingWarning = warning;
      activity("savedWithErrors");
      showError(warning, true);
    } else activity(toast, true);
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
      pendingWarning = null;
      setBusy(false);
    }
    return warning || true;
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
    statusSnapshot = status;
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
      find(".chip-label", chip).textContent = rule.name + " \u00b7 " + rule.address + (rule.target ? " \u2192 " + rule.target : "");
      find(".chip-state", chip).textContent = t(state, { attempt: rule.attempt });
      const markers = [rule.target ? "forward" : "", rule.remote ? "remote" : ""].filter(Boolean);
      find(".rule-marker", chip).textContent = markers.map(key => t(key)).join(" \u00b7 ");
      find(".rule-marker", chip).hidden = !markers.length;
      if (busy) chip.setAttribute("aria-disabled", "true");
      return chip;
    }));
    page?.renderStatus?.(status);
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

  function renderStatFields(root, stats) {
    const values = stats && {
      rate_up: formatRate(stats.rate_up), rate_down: formatRate(stats.rate_down),
      up: formatBytes(stats.up), down: formatBytes(stats.down),
      connections: formatCount(stats.active) + " / " + formatCount(stats.total),
      latency: formatLatency(stats) + " / " + formatLatency(stats, "avg_latency_ms"),
      latency_ms: formatLatency(stats), avg_latency_ms: formatLatency(stats, "avg_latency_ms"),
      dial_failures: formatCount(stats.dial_failures), dials: formatCount(stats.dials),
      last_active: formatLastActive(stats.last_active),
      last_active_time: Number.isFinite(Date.parse(stats.last_active))
        ? new Date(stats.last_active).toLocaleString(language) : "\u2014"
    };
    all("[data-stat]", root).forEach(element => {
      element.textContent = values?.[element.dataset.stat] ?? "\u2014";
    });
  }

  function renderConnections(rule) {
    const connections = list(rule?.connections);
    const body = find("#connection-rows");
    const rows = new Map(all("tr[data-id]", body).map(row => [row.dataset.id, row]));
    connections.slice(0, 200).forEach(connection => {
      const id = text(connection.id);
      let row = rows.get(id);
      if (row) rows.delete(id);
      else {
        row = clone("connection-row");
        row.dataset.id = id;
        const button = find(".disconnect-connection", row);
        button.addEventListener("click", () => disconnectConnection(id, button));
        body.append(row);
      }
      find(".connection-client", row).textContent = connection.client || "\u2014";
      find(".connection-target", row).textContent = connection.target;
      find(".connection-via", row).textContent = connection.via || t("direct");
      find(".connection-duration", row).textContent = formatDuration(connection.started);
      renderStatFields(row, connection);
    });
    rows.forEach(row => row.remove());
    find("#connections-table").hidden = !connections.length;
    find("#no-connections").hidden = Boolean(connections.length);
    const note = find("#connections-note");
    note.textContent = connections.length > 200
      ? t("showingConnections", { shown: formatCount(200), total: formatCount(connections.length) }) : "";
    note.hidden = !note.textContent;
  }

  async function disconnectConnection(id, button) {
    if (busy || button.disabled) return;
    button.disabled = true;
    clearErrors();
    activity("");
    try {
      await api("/connections/" + id, { method: "DELETE", base: "/apis/stats" });
      activity("disconnected", true);
    } catch (error) {
      button.disabled = false;
      showError(error);
    } finally {
      if (statsRequest) await statsRequest;
      await refreshStats();
    }
  }

  function hasStatsView() {
    return page?.loaded && page.renderStats && (page.kind === "stats" || page.kind === "rules" && !newRule);
  }

  function refreshStats() {
    if (busy || !hasStatsView() || document.visibilityState !== "visible") return Promise.resolve();
    if (statsRequest) return statsRequest;
    const view = page;
    statsRequest = api("", { base: "/apis/stats" }).then(snapshot => {
      if (page === view && hasStatsView() && document.visibilityState === "visible") view.renderStats(snapshot);
      return snapshot;
    }).catch(() => {}).finally(() => { statsRequest = null; });
    return statsRequest;
  }

  function startStats() {
    clearInterval(statsTimer);
    statsTimer = null;
    if (!hasStatsView() || document.visibilityState !== "visible") return;
    const view = page;
    if (statsRequest) statsRequest.then(() => { if (page === view) refreshStats(); });
    else refreshStats();
    statsTimer = setInterval(refreshStats, 1000);
  }

  function chainSummary(way, target = "") {
    if (!way.length && !target) return t("direct");
    const hops = way.map((node, index) => t("hop", { number: index + 1 }));
    return [t("chainLocal"), ...hops.reverse(), target || t("chainTarget")].join(" \u2192 ");
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

  function createHopEditor({ container, model, idPrefix, roles: { first, last }, target = () => "" }) {
    const hops = find(".hops", container);
    const add = find(".add-hop", container);
    const summary = find(".chain-summary", container);
    let statsHops = null;
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
      updateSummary();
      applyStats();
    }

    function applyStats(snapshot = statsHops) {
      statsHops = snapshot;
      const apply = (element, stats) => {
        element.hidden = statsHops === null;
        renderStats(element, dirty ? null : stats);
        if (dirty) element.textContent = "\u2014 \u00b7 " + t("saveStats");
      };
      all(".hop", hops).forEach((hop, hopIndex) => {
        const current = statsHops?.[hopIndex];
        const aggregate = find(".hop-stats", hop);
        apply(aggregate, current?.stats);
        if (!dirty && current) aggregate.textContent += " \u00b7 " + t("viaHop", {
          parent: current.parent_index === -1 ? t("chainLocal") : t("hop", { number: current.parent_index + 1 })
        });
        all(".url-stats", hop).forEach((element, urlIndex) => apply(element, current?.urls?.[urlIndex]?.stats));
      });
    }

    function updateSummary() {
      if (summary) summary.textContent = chainSummary(model, target());
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
    return { render, read, updateSummary, applyStats };
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
      const create = link.id === "new-rule-tab";
      if (link.getAttribute("aria-current") !== "page" && (hash !== activeHash || create !== newRule)) {
        navigate(hash, { create, replace: create && activeHash === "#/rules" });
      }
    });
    bindRuleTabs(ui.nav);
    window.addEventListener("hashchange", hashChanged);
    document.addEventListener("visibilitychange", startStats);
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