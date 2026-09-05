import { useCallback, useRef, useState, type SetStateAction } from "react";

/** Async guards must observe writes before React commits the next render. */
export function useImmediateState<T>(initial: T) {
  const [value, render] = useState(initial);
  const current = useRef(value);
  const setValue = useCallback((next: SetStateAction<T>) => {
    current.current = typeof next === "function"
      ? (next as (previous: T) => T)(current.current)
      : next;
    render(current.current);
  }, []);
  return [value, setValue, current] as const;
}
