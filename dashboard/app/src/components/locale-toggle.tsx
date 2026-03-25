import { useTranslation } from "react-i18next";
import { Globe } from "lucide-react";
import type { Locale } from "@/i18n";

const localeLabels: Record<Locale, string> = {
  en: "EN",
  "zh-CN": "中文",
};

export function LocaleToggle() {
  const { i18n } = useTranslation();

  const currentLocale = i18n.language as Locale;
  const nextLocale: Locale = currentLocale === "en" ? "zh-CN" : "en";

  const handleToggle = () => {
    i18n.changeLanguage(nextLocale);
  };

  return (
    <button
      type="button"
      onClick={handleToggle}
      className="inline-flex items-center gap-1.5 rounded-md border border-border bg-card px-2.5 py-1.5 text-sm transition-colors hover:bg-accent hover:text-accent-foreground"
      title={currentLocale === "en" ? "Switch to Chinese" : "切换到英文"}
    >
      <Globe className="h-4 w-4" />
      <span>{localeLabels[currentLocale]}</span>
    </button>
  );
}
