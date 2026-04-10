import i18n from "i18next";
import { initReactI18next } from "react-i18next";

const resources = {
  "zh-CN": {
    translation: {
      title: "Go 模板",
      login: "登录",
      setup: "安装向导",
      profile: "个人中心",
      admin: "管理后台",
      admin_console: "管理后台",
      admin_console_description: "后台模块入口与当前系统概览。",
      admin_overview: "后台概览",
      admin_users: "用户管理",
      admin_users_description: "查看、筛选并维护系统中的用户账号。",
      admin_settings_description: "运行期系统设置与后台配置。",
      settings: "系统设置",
      settings_loading: "加载系统设置中...",
      settings_empty_title: "暂未生成运行期设置",
      settings_empty_description: "安装向导会先生成最小可运行配置，后续可继续扩展更多模块设置。",
      settings_empty_hint: "完成安装向导后，这里会展示数据库、缓存、监听与日志等核心配置。",
      settings_group_database: "数据库",
      settings_group_database_description: "数据库驱动、地址与连接信息。",
      settings_group_cache: "缓存",
      settings_group_cache_description: "内存缓存或 Redis 连接相关设置。",
      settings_group_server: "服务监听",
      settings_group_server_description: "服务监听地址、端口与基础网络参数。",
      settings_group_log: "日志",
      settings_group_log_description: "启动日志级别与输出行为。",
      settings_group_default: "未分组",
      settings_group_default_description: "未归类但运行期仍会读取的核心配置。",
      settings_group_custom_description: "自定义配置分组。",
      submit: "提交",
      logout: "退出登录",
      theme: "主题",
      language: "语言",
      accent: "主题色",
      home_intro: "一个可安装、可嵌入发布的 Go 全栈模板",
      login_identifier: "用户名或邮箱",
      password: "密码",
      admin_username: "管理员用户名",
      admin_email: "管理员邮箱",
      admin_password: "管理员密码",
      install_now: "初始化系统"
    }
  },
  "en-US": {
    translation: {
      title: "Go Template",
      login: "Login",
      setup: "Setup Wizard",
      profile: "Profile",
      admin: "Admin",
      admin_console: "Admin Console",
      admin_console_description: "Overview of admin modules and current system status.",
      admin_overview: "Overview",
      admin_users: "Users",
      admin_users_description: "View, filter, and maintain user accounts.",
      admin_settings_description: "Runtime settings and backend configuration.",
      settings: "Settings",
      settings_loading: "Loading system settings...",
      settings_empty_title: "No runtime settings yet",
      settings_empty_description: "The setup wizard creates the minimum runnable configuration first, and you can extend more modules later.",
      settings_empty_hint: "After the setup wizard finishes, this page shows core database, cache, server, and log settings.",
      settings_group_database: "Database",
      settings_group_database_description: "Driver, address, and connection settings for the database.",
      settings_group_cache: "Cache",
      settings_group_cache_description: "Memory cache or Redis connection settings.",
      settings_group_server: "Server",
      settings_group_server_description: "Listen address, port, and basic network parameters.",
      settings_group_log: "Logs",
      settings_group_log_description: "Startup log level and output behavior.",
      settings_group_default: "Uncategorized",
      settings_group_default_description: "Core settings without an explicit runtime group.",
      settings_group_custom_description: "Custom configuration group.",
      submit: "Submit",
      logout: "Logout",
      theme: "Theme",
      language: "Language",
      accent: "Accent",
      home_intro: "An installable Go full-stack starter with embedded web assets",
      login_identifier: "Username or email",
      password: "Password",
      admin_username: "Admin username",
      admin_email: "Admin email",
      admin_password: "Admin password",
      install_now: "Install"
    }
  }
} as const;

const savedLanguage = localStorage.getItem("app.language");
const browserLanguage = navigator.language.startsWith("en") ? "en-US" : "zh-CN";

void i18n.use(initReactI18next).init({
  resources,
  lng: savedLanguage || browserLanguage,
  fallbackLng: "zh-CN",
  interpolation: {
    escapeValue: false
  }
});

export default i18n;
