import type { TFunction } from "i18next";

/**
 * Format a relative time string based on the current locale
 */
export function formatRelativeTime(t: TFunction, isoDate?: string): string {
  if (!isoDate) return t("time.never");

  const diff = Date.now() - new Date(isoDate).getTime();
  const minutes = Math.floor(diff / 60_000);
  const hours = Math.floor(diff / 3_600_000);
  const days = Math.floor(diff / 86_400_000);

  if (minutes < 1) return t("time.justNow");
  if (minutes < 60) return t("time.minutesAgo", { n: minutes });
  if (hours < 24) return t("time.hoursAgo", { n: hours });
  if (days < 2) return t("time.yesterday");

  return t("time.daysAgo", { n: days });
}

/**
 * Format a date with locale-aware formatting
 */
export function formatDateTime(isoDate?: string, locale: string = "en"): string {
  if (!isoDate) return "--";

  const date = new Date(isoDate);
  const localeStr = locale === "zh-CN" ? "zh-CN" : "en-US";

  return date.toLocaleDateString(localeStr) + " " + date.toLocaleTimeString(localeStr);
}

/**
 * Format bytes with localized unit suffixes
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";

  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}
