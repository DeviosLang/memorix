import { useState, useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ThemeToggle } from "@/components/theme-toggle";
import { setServerUrl, setDashboardToken, verifyCredentials, clearSession } from "@/api/client";
import { initTheme } from "@/lib/theme";
import { Server, Key, AlertCircle, Loader2 } from "lucide-react";

const DEFAULT_SERVER_URL = "http://localhost:8080";
const VERSION = "0.1.0"; // From package.json
const DOCS_URL = "https://github.com/DeviosLang/memorix#readme";

type ErrorType = "invalid_token" | "server_unreachable" | "network_error" | null;

export function LoginPage() {
  const [serverUrl, setServerUrlInput] = useState(DEFAULT_SERVER_URL);
  const [token, setToken] = useState("");
  const [error, setError] = useState<ErrorType>(null);
  const [isLoading, setIsLoading] = useState(false);
  const navigate = useNavigate();

  // Initialize theme on mount
  useEffect(() => {
    initTheme();
    // Clear any existing session on login page
    clearSession();
  }, []);

  const getErrorMessage = (errorType: ErrorType): string => {
    switch (errorType) {
      case "invalid_token":
        return "Invalid dashboard token. Please check your token and try again.";
      case "server_unreachable":
        return "Unable to connect to server. Please verify the server URL is correct and the server is running.";
      case "network_error":
        return "Network error. Please check your connection and try again.";
      default:
        return "";
    }
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsLoading(true);

    if (!token.trim()) {
      setError("invalid_token");
      setIsLoading(false);
      return;
    }

    // Normalize server URL (remove trailing slash)
    const normalizedUrl = serverUrl.trim().replace(/\/+$/, "");

    try {
      // Verify credentials by calling the overview endpoint
      await verifyCredentials(normalizedUrl, token.trim());

      // Store credentials in sessionStorage
      setServerUrl(normalizedUrl);
      setDashboardToken(token.trim());

      // Navigate to dashboard
      navigate({ to: "/dashboard" });
    } catch (err) {
      if (err instanceof Error) {
        if (err.message.includes("Invalid dashboard token") || err.message.includes("401")) {
          setError("invalid_token");
        } else if (err.message.includes("Failed to fetch") || err.message.includes("NetworkError")) {
          setError("server_unreachable");
        } else {
          setError("network_error");
        }
      } else {
        setError("network_error");
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      {/* Theme toggle in top-right corner */}
      <div className="fixed right-4 top-4">
        <ThemeToggle />
      </div>

      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-xl bg-primary">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-8 w-8 text-primary-foreground"
            >
              <path d="M12 2L2 7l10 5 10-5-10-5z" />
              <path d="M2 17l10 5 10-5" />
              <path d="M2 12l10 5 10-5" />
            </svg>
          </div>
          <CardTitle className="text-2xl">Memorix Dashboard</CardTitle>
          <CardDescription>
            Enter your server URL and dashboard token to connect
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleLogin} className="space-y-4">
            {/* Server URL Field */}
            <div className="space-y-2">
              <label
                htmlFor="server-url"
                className="flex items-center gap-2 text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
              >
                <Server className="h-4 w-4" />
                Server URL
              </label>
              <input
                id="server-url"
                type="url"
                value={serverUrl}
                onChange={(e) => setServerUrlInput(e.target.value)}
                placeholder={DEFAULT_SERVER_URL}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>

            {/* Token Field */}
            <div className="space-y-2">
              <label
                htmlFor="token"
                className="flex items-center gap-2 text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
              >
                <Key className="h-4 w-4" />
                Dashboard Token
              </label>
              <input
                id="token"
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="Enter your dashboard token"
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>

            {/* Error Message */}
            {error && (
              <div className="flex items-center gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                <AlertCircle className="h-4 w-4 flex-shrink-0" />
                <span>{getErrorMessage(error)}</span>
              </div>
            )}

            {/* Submit Button */}
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Connecting...
                </>
              ) : (
                "Connect"
              )}
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* Footer */}
      <footer className="mt-6 text-center text-sm text-muted-foreground">
        <p>
          Memorix Dashboard v{VERSION} &middot;{" "}
          <a
            href={DOCS_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="underline hover:text-foreground"
          >
            Documentation
          </a>
        </p>
      </footer>
    </div>
  );
}
