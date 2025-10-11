import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

// Translation resources
const resources = {
  en: {
    translation: {
      // Page title
      "pageTitle": "Jump Way Configuration",
      
      // Sections
      "currentContext": "Current Context",
      "proxySettings": "Proxy Settings",
      "noProxySettings": "No Proxy Settings",
      "contexts": "Contexts",
      
      // Labels
      "currentContextName": "Current Context Name",
      "selectContext": "-- Select a context --",
      "host": "Host",
      "port": "Port",
      "list": "List",
      "fromEnvironment": "From Environment",
      "fromFile": "From File",
      "name": "Name",
      "wayNodes": "Way Nodes",
      "probe": "Probe",
      "loadBalancerEntries": "Load Balancer Entries",
      
      // Buttons
      "edit": "Edit Configuration",
      "save": "Save",
      "saving": "Saving...",
      "cancel": "Cancel",
      "delete": "Delete",
      "addListEntry": "Add List Entry",
      "addFromEnvEntry": "Add FromEnv Entry",
      "addFromFileEntry": "Add FromFile Entry",
      "addContext": "Add Context",
      "addWayNode": "Add Way Node",
      "addLbEntry": "Add LB Entry",
      "deleteWayNode": "Delete Way Node",
      "deleteContext": "Delete Context",
      
      // Context info
      "context": "Context {{number}}",
      
      // Messages
      "loading": "Loading configuration...",
      "error": "Error: {{message}}",
      "retry": "Retry",
      "noConfiguration": "No configuration available",
      "savedSuccessfully": "Configuration saved successfully!",
      "failedToFetch": "Failed to fetch configuration",
      "failedToSave": "Failed to save configuration",
      "unknownError": "Unknown error"
    }
  },
  zh: {
    translation: {
      // Page title
      "pageTitle": "Jump Way 配置",
      
      // Sections
      "currentContext": "当前上下文",
      "proxySettings": "代理设置",
      "noProxySettings": "不代理设置",
      "contexts": "上下文",
      
      // Labels
      "currentContextName": "当前上下文名称",
      "selectContext": "-- 选择上下文 --",
      "host": "主机",
      "port": "端口",
      "list": "列表",
      "fromEnvironment": "从环境变量",
      "fromFile": "从文件",
      "name": "名称",
      "wayNodes": "路由节点",
      "probe": "探测",
      "loadBalancerEntries": "负载均衡条目",
      
      // Buttons
      "edit": "编辑配置",
      "save": "保存",
      "saving": "保存中...",
      "cancel": "取消",
      "delete": "删除",
      "addListEntry": "添加列表条目",
      "addFromEnvEntry": "添加环境变量条目",
      "addFromFileEntry": "添加文件条目",
      "addContext": "添加上下文",
      "addWayNode": "添加路由节点",
      "addLbEntry": "添加负载均衡条目",
      "deleteWayNode": "删除路由节点",
      "deleteContext": "删除上下文",
      
      // Context info
      "context": "上下文 {{number}}",
      
      // Messages
      "loading": "加载配置中...",
      "error": "错误：{{message}}",
      "retry": "重试",
      "noConfiguration": "无可用配置",
      "savedSuccessfully": "配置保存成功！",
      "failedToFetch": "获取配置失败",
      "failedToSave": "保存配置失败",
      "unknownError": "未知错误"
    }
  }
};

i18n
  .use(initReactI18next)
  .init({
    resources,
    lng: navigator.language.startsWith('zh') ? 'zh' : 'en',
    fallbackLng: 'en',
    interpolation: {
      escapeValue: false
    }
  });

export default i18n;
