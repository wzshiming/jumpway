"use strict";

(() => {
  const STR = {
    en: {
      appName: "JumpWay",
      configuration: "Configuration",
      checking: "Status unknown",
      running: "Running",
      stopped: "Stopped",
      reload: "Reload from disk",
      save: "Save & Apply",
      unsaved: "Unsaved changes",
      discardChanges: "Discard unsaved changes and load the configuration from disk?",
      editorTabs: "Configuration editor",
      form: "Form",
      yaml: "Advanced YAML",
      listen: "Listen address",
      host: "Host",
      port: "Port",
      listenHint: "The web UI moves with the listen address. An empty host uses 127.0.0.1; port 0 chooses an available port.",
      contexts: "Contexts",
      contextName: "Context name",
      contextNumber: "Context {number}",
      current: "Current",
      deleteContext: "Delete context",
      addContext: "+ Context",
      noContexts: "No contexts: connections go direct.",
      noHops: "No hops: this context connects directly.",
      hop: "Hop {number}",
      up: "Up",
      down: "Down",
      deleteHop: "Delete hop",
      addHop: "+ Hop",
      proxyURL: "Proxy URL",
      remove: "Remove",
      addURL: "+ URL",
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
      loading: "Loading configuration...",
      saving: "Saving and applying...",
      saved: "Saved and applied.",
      reloaded: "Reloaded from disk.",
      movedTo: "The web UI has moved to",
      unreachable: "Cannot reach JumpWay.",
      recoveryHint: "The proxy may have stopped. Use the tray's Edit Config to check ~/.jumpway/config.yaml, then Reload Config to restart it.",
      requestFailed: "The request could not be completed.",
      invalidResponse: "JumpWay returned an unreadable response. Try reloading from disk."
    },
    zh: {
      appName: "JumpWay",
      configuration: "配置",
      checking: "状态未知",
      running: "运行中",
      stopped: "已停止",
      reload: "从磁盘重新加载",
      save: "保存并应用",
      unsaved: "未保存的修改",
      discardChanges: "放弃未保存的修改，并从磁盘重新加载配置吗？",
      editorTabs: "配置编辑器",
      form: "表单",
      yaml: "高级 YAML",
      listen: "监听地址",
      host: "主机",
      port: "端口",
      listenHint: "网页配置的地址会随监听地址改变。主机留空时使用 127.0.0.1；端口为 0 时自动分配可用端口。",
      contexts: "上下文",
      contextName: "上下文名称",
      contextNumber: "上下文 {number}",
      current: "当前",
      deleteContext: "删除上下文",
      addContext: "+ 上下文",
      noContexts: "没有上下文：直接连接，不走代理。",
      noHops: "没有跳板节点：此上下文使用直接连接。",
      hop: "跳板节点 {number}",
      up: "上移",
      down: "下移",
      deleteHop: "删除节点",
      addHop: "+ 跳板节点",
      proxyURL: "代理 URL",
      remove: "移除",
      addURL: "+ URL",
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
      loading: "正在加载配置...",
      saving: "正在保存并应用...",
      saved: "已保存并应用。",
      reloaded: "已从磁盘重新加载。",
      movedTo: "网页配置已移至",
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

  async function api(route = "", payload) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 15000);
    try {
      const response = await fetch("/apis/configs" + route, {
        method: payload === undefined ? "GET" : "PUT",
        headers: payload === undefined ? { Accept: "application/json" }
          : { Accept: "application/json", "Content-Type": "application/json" },
        body: payload === undefined ? undefined : JSON.stringify(payload),
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

  function normalize(config) {
    return {
      current_context: text(config.current_context),
      contexts: list(config.contexts).map(context => ({
        name: text(context.name),
        way: list(context.way).map(node => ({
          lb: (typeof node === "string" ? node.split("|")
            : Array.isArray(node) ? node : list(node?.lb)).map(text)
        }))
      })),
      proxy: { host: text(config.proxy?.host), port: config.proxy?.port ?? 0 },
      no_proxy: {
        list: list(config.no_proxy?.list).map(text),
        from_env: list(config.no_proxy?.from_env).map(text),
        from_file: list(config.no_proxy?.from_file).map(text)
      }
    };
  }

  function serialize() {
    readForm();
    return {
      current_context: text(state.contexts[currentIndex]?.name).trim(),
      contexts: state.contexts.map(context => ({
        name: context.name.trim(),
        way: context.way.map(node => ({ lb: cleanLines(node.lb) }))
      })),
      proxy: { host: state.proxy.host.trim(), port: Number(state.proxy.port) },
      no_proxy: {
        list: cleanLines(state.no_proxy.list),
        from_env: cleanLines(state.no_proxy.from_env),
        from_file: cleanLines(state.no_proxy.from_file)
      }
    };
  }

  const find = (selector, root = document) => root.querySelector(selector);
  const all = (selector, root = document) => Array.from(root.querySelectorAll(selector));
  const ui = {
    form: find("#config-form"), fields: find("#form-fields"),
    host: find("#proxy-host"), port: find("#proxy-port"),
    contexts: find("#contexts"), noContexts: find("#no-contexts"),
    bypass: find("#no-proxy-list"), env: find("#no-proxy-env"), files: find("#no-proxy-files"),
    yaml: find("#yaml-source"), save: find("#save"), reload: find("#reload"),
    tabs: all("[role=tab]"), dirty: find("#dirty-badge"), activity: find("#activity"),
    error: find("#request-error"), errorText: find("#request-error-text"),
    unreachable: find("#unreachable-banner"), moved: find("#moved-banner"), movedLink: find("#moved-link"),
    statusDot: find("#status-dot"), statusLabel: find("#status-label"),
    statusAddress: find("#status-address"), statusError: find("#status-error")
  };
  let state = null;
  let currentIndex = -1;
  let activeTab = "form";
  let dirty = false;
  let busy = false;
  let loaded = false;
  let statusRequest = null;
  let requestUnreachable = false;
  let toastTimer;

  function localize(root) {
    all("[data-i18n]", root).forEach(element => { element.textContent = t(element.dataset.i18n); });
    all("[data-i18n-aria]", root).forEach(element => {
      element.setAttribute("aria-label", t(element.dataset.i18nAria));
    });
  }

  function clone(name) {
    const fragment = find("#" + name + "-template").content.cloneNode(true);
    localize(fragment);
    return fragment.firstElementChild;
  }

  function setDirty(value) {
    dirty = value;
    ui.dirty.hidden = !value;
  }

  function activity(key, success = false) {
    clearTimeout(toastTimer);
    ui.activity.textContent = key ? t(key) : "";
    ui.activity.classList.toggle("success", success);
    if (success) toastTimer = setTimeout(() => { ui.activity.textContent = ""; }, 6000);
  }

  function setBusy(value, message) {
    busy = value;
    ui.save.disabled = value || !loaded;
    ui.reload.disabled = value;
    ui.fields.disabled = value || !loaded;
    ui.yaml.disabled = value || !loaded;
    ui.tabs.forEach(tab => { tab.disabled = value; });
    ui.tabs.forEach(tab => {
      const panel = find("#" + tab.getAttribute("aria-controls"));
      panel.setAttribute("aria-busy", String(value && tab.id === activeTab + "-tab"));
    });
    if (message) activity(message);
  }

  function clearErrors() {
    ui.error.hidden = true;
    ui.errorText.textContent = "";
    ui.unreachable.hidden = true;
    requestUnreachable = false;
  }

  function showError(error, persistent = false) {
    if (error instanceof TypeError || error.name === "AbortError") {
      requestUnreachable = requestUnreachable || persistent;
      ui.unreachable.hidden = false;
      ui.statusDot.className = "status-dot unknown";
      ui.statusLabel.textContent = t("checking");
    } else {
      ui.errorText.textContent = error.message || t("requestFailed");
      ui.error.hidden = false;
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
    const canonical = value => {
      const url = addressURL(value || "127.0.0.1");
      if (!url) return "";
      const host = url.hostname === "localhost" ? "127.0.0.1" : url.hostname;
      return host + ":" + (url.port || "80");
    };
    return canonical(address) === canonical(location.host);
  }

  function showMoved(address) {
    const url = addressURL(address);
    if (!url || sameAddress(address)) return;
    if (["en", "zh"].includes(override)) url.searchParams.set("lang", override);
    ui.movedLink.href = url.href;
    ui.movedLink.textContent = address;
    ui.moved.hidden = false;
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

  function readForm() {
    if (!state) return;
    state.proxy.host = ui.host.value;
    state.proxy.port = ui.port.value;
    state.no_proxy.list = ui.bypass.value.split(/\r?\n/);
    state.no_proxy.from_env = ui.env.value.split(/\r?\n/);
    state.no_proxy.from_file = ui.files.value.split(/\r?\n/);
    currentIndex = -1;
    all(".context-card", ui.contexts).forEach((card, contextIndex) => {
      const context = state.contexts[contextIndex];
      context.name = find(".context-name", card).value;
      const current = find("input[name=current]", card).checked;
      if (current) currentIndex = contextIndex;
      card.classList.toggle("is-current", current);
      all(".hop", card).forEach((hop, hopIndex) => {
        context.way[hopIndex].lb = all(".proxy-url", hop).map(input => input.value);
      });
    });
  }

  function changeForm(change) {
    if (busy) return;
    readForm();
    const focus = change();
    setDirty(true);
    renderForm();
    if (focus) find(focus)?.focus();
  }

  function renderURL(value, contextIndex, hopIndex, urlIndex) {
    const row = clone("url");
    const input = find("input", row);
    input.id = "url-" + contextIndex + "-" + hopIndex + "-" + urlIndex;
    input.value = value;
    find("label", row).htmlFor = input.id;
    row.dataset.context = contextIndex;
    row.dataset.hop = hopIndex;
    row.dataset.url = urlIndex;
    find(".remove-url", row).addEventListener("click", () => changeForm(() => {
      const urls = state.contexts[contextIndex].way[hopIndex].lb;
      urls.splice(urlIndex, 1);
      return urls.length ? "#url-" + contextIndex + "-" + hopIndex + "-" + Math.min(urlIndex, urls.length - 1)
        : "#hop-" + contextIndex + "-" + hopIndex + " .add-url";
    }));
    return row;
  }

  function renderHop(node, contextIndex, hopIndex) {
    const hop = clone("hop");
    const way = state.contexts[contextIndex].way;
    hop.id = "hop-" + contextIndex + "-" + hopIndex;
    const title = find(".hop-title", hop);
    title.id = hop.id + "-title";
    title.textContent = t("hop", { number: hopIndex + 1 });
    hop.setAttribute("aria-labelledby", title.id);
    const rows = find(".url-rows", hop);
    node.lb.forEach((url, urlIndex) => rows.append(renderURL(url, contextIndex, hopIndex, urlIndex)));
    const move = offset => changeForm(() => {
      const target = hopIndex + offset;
      [way[hopIndex], way[target]] = [way[target], way[hopIndex]];
      return "#hop-" + contextIndex + "-" + target + " .add-url";
    });
    find(".hop-up", hop).disabled = hopIndex === 0;
    find(".hop-down", hop).disabled = hopIndex === way.length - 1;
    find(".hop-up", hop).addEventListener("click", () => move(-1));
    find(".hop-down", hop).addEventListener("click", () => move(1));
    find(".delete-hop", hop).addEventListener("click", () => changeForm(() => {
      way.splice(hopIndex, 1);
      return "#context-" + contextIndex + " .add-hop";
    }));
    find(".add-url", hop).addEventListener("click", () => changeForm(() => {
      node.lb.push("");
      return "#url-" + contextIndex + "-" + hopIndex + "-" + (node.lb.length - 1);
    }));
    return hop;
  }

  function renderContext(context, contextIndex) {
    const card = clone("context");
    card.id = "context-" + contextIndex;
    card.setAttribute("aria-label", t("contextNumber", { number: contextIndex + 1 }));
    card.classList.toggle("is-current", contextIndex === currentIndex);
    const name = find(".context-name", card);
    name.id = card.id + "-name";
    name.value = context.name;
    find(".context-name-label", card).htmlFor = name.id;
    const radio = find("input[name=current]", card);
    radio.value = contextIndex;
    radio.checked = contextIndex === currentIndex;
    const hops = find(".hops", card);
    context.way.forEach((node, hopIndex) => hops.append(renderHop(node, contextIndex, hopIndex)));
    if (!context.way.length) {
      const empty = document.createElement("p");
      empty.className = "hint";
      empty.textContent = t("noHops");
      hops.append(empty);
    }
    find(".add-hop", card).addEventListener("click", () => changeForm(() => {
      context.way.push({ lb: [""] });
      return "#url-" + contextIndex + "-" + (context.way.length - 1) + "-0";
    }));
    find(".delete-context", card).addEventListener("click", () => changeForm(() => {
      state.contexts.splice(contextIndex, 1);
      if (currentIndex === contextIndex) currentIndex = Math.min(contextIndex, state.contexts.length - 1);
      else if (currentIndex > contextIndex) currentIndex--;
      return state.contexts.length ? "#context-" + Math.min(contextIndex, state.contexts.length - 1) + "-name"
        : "#add-context";
    }));
    return card;
  }

  function renderForm() {
    ui.host.value = state.proxy.host;
    ui.port.value = state.proxy.port;
    ui.bypass.value = state.no_proxy.list.join("\n");
    ui.env.value = state.no_proxy.from_env.join("\n");
    ui.files.value = state.no_proxy.from_file.join("\n");
    ui.contexts.replaceChildren(...state.contexts.map(renderContext));
    ui.noContexts.hidden = state.contexts.length !== 0;
    find("#context-count").textContent = state.contexts.length.toLocaleString(language);
  }

  function acceptConfig(config) {
    state = normalize(config);
    currentIndex = state.contexts.findIndex(context => context.name === state.current_context);
    if (activeTab === "form") renderForm();
  }

  function selectTab(tab) {
    activeTab = tab;
    ui.tabs.forEach(button => {
      const selected = button.id === tab + "-tab";
      button.setAttribute("aria-selected", String(selected));
      button.tabIndex = selected ? 0 : -1;
      find("#" + button.getAttribute("aria-controls")).hidden = !selected;
    });
  }

  async function loadTab(tab, notify = false) {
    if (busy || (dirty && !confirm(t("discardChanges")))) return;
    setBusy(true, "loading");
    clearErrors();
    try {
      const data = await api(tab === "yaml" ? "/raw" : "");
      selectTab(tab);
      if (tab === "form") acceptConfig(data);
      else ui.yaml.value = data.yaml;
      loaded = true;
      setDirty(false);
      activity(notify ? "reloaded" : "", notify);
      await refreshStatus();
    } catch (error) {
      activity("");
      showError(error, true);
    } finally {
      setBusy(false);
    }
  }

  function submittedAddress(payload) {
    if (!payload?.proxy?.port) return "";
    let host = payload.proxy.host || "127.0.0.1";
    if (["0.0.0.0", "::", "[::]"].includes(host)) host = "127.0.0.1";
    if (host.includes(":") && !host.startsWith("[")) host = "[" + host + "]";
    return host + ":" + payload.proxy.port;
  }

  async function save() {
    if (busy || !loaded) return;
    const payload = activeTab === "yaml" ? { yaml: ui.yaml.value } : serialize();
    setBusy(true, "saving");
    clearErrors();
    try {
      if (statusRequest) await statusRequest.catch(() => {});
      await api(activeTab === "yaml" ? "/raw" : "", payload);
    } catch (error) {
      activity("");
      showError(error, true);
      await refreshStatus().catch(showError);
      setBusy(false);
      return;
    }
    setDirty(false);
    activity("saved", true);
    try {
      await refreshStatus();
      acceptConfig(await api());
      if (activeTab === "yaml") ui.yaml.value = (await api("/raw")).yaml;
    } catch (error) {
      showMoved(submittedAddress(payload));
      showError(error, true);
    } finally {
      setBusy(false);
    }
  }

  function bindEvents() {
    ["input", "change"].forEach(event => {
      ui.form.addEventListener(event, () => {
        if (busy || !loaded) return;
        readForm();
        setDirty(true);
      });
      ui.yaml.addEventListener(event, () => {
        if (!busy && loaded) setDirty(true);
      });
    });
    find("#add-context").addEventListener("click", () => changeForm(() => {
      let number = 1;
      while (state.contexts.some(context => context.name.trim() === "context-" + number)) number++;
      state.contexts.push({ name: "context-" + number, way: [{ lb: [""] }] });
      if (state.contexts.length === 1) currentIndex = 0;
      return "#context-" + (state.contexts.length - 1) + "-name";
    }));
    ui.save.addEventListener("click", save);
    ui.reload.addEventListener("click", () => loadTab(activeTab, true));
    ui.form.addEventListener("submit", event => { event.preventDefault(); save(); });
    ui.tabs.forEach(tab => tab.addEventListener("click", () => {
      const target = tab.id === "form-tab" ? "form" : "yaml";
      if (target !== activeTab) loadTab(target);
    }));
    find("[role=tablist]").addEventListener("keydown", event => {
      if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key) || busy) return;
      event.preventDefault();
      const position = ui.tabs.indexOf(document.activeElement);
      const next = event.key === "Home" ? 0 : event.key === "End" ? ui.tabs.length - 1
        : (position + (event.key === "ArrowRight" ? 1 : -1) + ui.tabs.length) % ui.tabs.length;
      ui.tabs[next].focus();
      ui.tabs[next].click();
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
  loadTab("form");
  setInterval(() => {
    if (!busy) refreshStatus().catch(showError);
  }, 10000);
})();