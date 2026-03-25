import { useState, useEffect } from "react";
import { Sun, Moon, Monitor } from "lucide-react";
import { type Theme, getTheme, setTheme } from "@/lib/theme";

const themeIcons: Record<Theme, React.ReactNode> = {
  light: <Sun className="h-4 w-4" />,
  dark: <Moon className="h-4 w-4" />,
  system: <Monitor className="h-4 w-4" />,
};

const themeLabels: Record<Theme, string> = {
  light: "Light",
  dark: "Dark",
  system: "System",
};

export function ThemeToggle() {
  const [currentTheme, setCurrentTheme] = useState<Theme>("system");

  useEffect(() => {
    setCurrentTheme(getTheme());
  }, []);

  const handleThemeChange = (theme: Theme) => {
    setCurrentTheme(theme);
    setTheme(theme);
  };

  return (
    <div className="flex items-center gap-1 rounded-lg border border-border bg-card p-1">
      {(["light", "dark", "system"] as Theme[]).map((theme) => (
        <button
          key={theme}
          type="button"
          onClick={() => handleThemeChange(theme)}
          className={`inline-flex items-center justify-center rounded-md px-2 py-1.5 text-sm transition-colors ${
            currentTheme === theme
              ? "bg-primary text-primary-foreground"
              : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
          }`}
          title={themeLabels[theme]}
        >
          {themeIcons[theme]}
        </button>
      ))}
    </div>
  );
}
