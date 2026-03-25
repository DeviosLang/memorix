import { useEffect, useRef, useCallback } from "react";
import { useNavigate } from "@tanstack/react-router";
import { clearSession, getDashboardToken } from "@/api/client";

const INACTIVITY_TIMEOUT_MS = 30 * 60 * 1000; // 30 minutes
const ACTIVITY_EVENTS = [
  "mousedown",
  "mousemove",
  "keydown",
  "scroll",
  "touchstart",
  "click",
];

/**
 * Hook to manage session timeout due to inactivity.
 * Automatically logs out the user after 30 minutes of no activity.
 */
export function useSessionTimeout() {
  const navigate = useNavigate();
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastActivityRef = useRef<number>(Date.now());

  const logout = useCallback(() => {
    clearSession();
    navigate({ to: "/" });
  }, [navigate]);

  const resetTimer = useCallback(() => {
    lastActivityRef.current = Date.now();

    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }

    timeoutRef.current = setTimeout(() => {
      logout();
    }, INACTIVITY_TIMEOUT_MS);
  }, [logout]);

  useEffect(() => {
    // Only set up the timer if user is logged in
    const token = getDashboardToken();
    if (!token) {
      return;
    }

    // Start the initial timer
    resetTimer();

    // Add event listeners for user activity
    const handleActivity = () => {
      resetTimer();
    };

    ACTIVITY_EVENTS.forEach((event) => {
      window.addEventListener(event, handleActivity, { passive: true });
    });

    return () => {
      // Cleanup on unmount
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
      ACTIVITY_EVENTS.forEach((event) => {
        window.removeEventListener(event, handleActivity);
      });
    };
  }, [resetTimer]);

  return {
    lastActivity: lastActivityRef.current,
    resetTimer,
  };
}

/**
 * Check if user has a valid session (token exists).
 */
export function hasSession(): boolean {
  return getDashboardToken() !== null;
}
