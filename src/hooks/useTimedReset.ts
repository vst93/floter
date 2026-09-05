import { useEffect, useRef } from "react";

export function useTimedReset(value: unknown, reset: () => void, timeout = 3000) {
  const resetRef = useRef(reset);
  resetRef.current = reset;
  useEffect(() => {
    if (!value) return;
    const timer = window.setTimeout(() => resetRef.current(), timeout);
    return () => window.clearTimeout(timer);
  }, [value, timeout]);
}
