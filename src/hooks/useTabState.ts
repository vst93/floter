import { useCallback, useState, type KeyboardEvent } from "react";

export type ExtensionTab = "installed" | "discover" | "updates";

type UseTabStateOptions = {
  initial?: ExtensionTab;
  onChange?: (tab: ExtensionTab) => void;
};

export function useTabState({ initial = "installed", onChange }: UseTabStateOptions = {}) {
  const [tab, setTabState] = useState<ExtensionTab>(initial);
  const setTab = useCallback((next: ExtensionTab) => {
    setTabState(next);
    onChange?.(next);
  }, [onChange]);

  const onTabKeyDown = useCallback((event: KeyboardEvent<HTMLElement>) => {
    const tabs: ExtensionTab[] = ["installed", "discover", "updates"];
    const current = tabs.indexOf(tab);
    if (event.key === "ArrowRight" || event.key === "ArrowDown") {
      event.preventDefault();
      setTab(tabs[(current + 1) % tabs.length]);
    } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
      event.preventDefault();
      setTab(tabs[(current - 1 + tabs.length) % tabs.length]);
    } else if (event.key === "Home") {
      event.preventDefault();
      setTab(tabs[0]);
    } else if (event.key === "End") {
      event.preventDefault();
      setTab(tabs[tabs.length - 1]);
    }
  }, [setTab, tab]);

  return { tab, setTab, onTabKeyDown };
}
