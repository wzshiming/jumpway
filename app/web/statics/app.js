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
      loading: "Loading configuration...",
      saving: "Saving and applying...",
      saved: "Saved and applied.",
      reloaded: "Reloaded from disk.",
      movedTo: "The web UI has moved to",
      movedUnknown: "Saved and applied. The listen address changed. Open the address shown in the tray status item.",
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
      loading: "正在加载配置...",
      saving: "正在保存并应用...",
      saved: "已保存并应用。",
      reloaded: "已从磁盘重新加载。",
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
    movedMessage: find("#moved-message"),
    statusDot: find("#status-dot"), statusLabel: find("#status-label"),
    statusAddress: find("#status-address"), statusError: find("#status-error"),
    builder: find("#url-builder"), builderForm: find("#builder-form"),
    builderProtocol: find("#builder-protocol"), builderFields: find("#builder-fields"),
    builderPreview: find("#builder-preview"), builderHint: find("#builder-hint"),
    builderUse: find("#builder-use"), builderCancel: find("#builder-cancel")
  };
  let state = null;
  let currentIndex = -1;
  let activeTab = "form";
  let dirty = false;
  let busy = false;
  let loaded = false;
  let statusRequest = null;
  let requestUnreachable = false;
  let builderLayouts = [];
  let builderRequest = null;
  let builderTarget = null;
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
      ui.unreachable.hidden = !ui.moved.hidden && ui.movedLink.hidden;
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
    const listener = addressURL(address);
    const page = addressURL(location.host);
    if (!listener || !page) return false;
    const host = url => url.hostname === "localhost" ? "127.0.0.1" : url.hostname;
    return (listener.port || "80") === (page.port || "80")
      && (host(listener) === "127.0.0.1" || host(listener) === host(page));
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

  function renderURL(value, contextIndex, hopIndex, urlIndex) {
    const row = clone("url");
    const input = find("input", row);
    input.id = "url-" + contextIndex + "-" + hopIndex + "-" + urlIndex;
    input.value = value;
    find("label", row).htmlFor = input.id;
    row.dataset.context = contextIndex;
    row.dataset.hop = hopIndex;
    row.dataset.url = urlIndex;
    find(".build-url", row).addEventListener("click", () => openURLBuilder(input));
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
    if (busy || !loaded || ui.builder.open) return;
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
      const address = submittedAddress(payload);
      if (!showMoved(address) && !address) {
        ui.movedMessage.textContent = t("movedUnknown");
        ui.movedLink.hidden = true;
        ui.moved.hidden = false;
      } else {
        showError(error, true);
      }
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