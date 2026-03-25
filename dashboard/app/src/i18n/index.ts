import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import zhCN from "./locales/zh-CN.json";
import en from "./locales/en.json";

const STORAGE_KEY = "memorix-dashboard-locale";

export type Locale = "en" | "zh-CN";

function detectLocale(): Locale {
  // Check localStorage first
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved === "zh-CN" || saved === "en") return saved;

  // Fall back to browser language
  const browserLang = navigator.language;
  if (browserLang.startsWith("zh")) return "zh-CN";

  return "en";
}

i18n.use(initReactI18next).init({
  resources: {
    "zh-CN": { translation: zhCN },
    en: { translation: en },
  },
  lng: detectLocale(),
  fallbackLng: "en",
  interpolation: {
    escapeValue: false,
  },
});

// Persist language preference on change
i18n.on("languageChanged", (lng) => {
  localStorage.setItem(STORAGE_KEY, lng);
});

export default i18n;
