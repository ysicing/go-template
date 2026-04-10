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
      settings: "系统设置",
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
      settings: "Settings",
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

