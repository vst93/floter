import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent as ReactKeyboardEvent, type MouseEvent, type RefObject } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  AlertCircle,
  Check,
  Copy,
  ExternalLink,
  FileDown,
  FileUp,
  Link2,
  LoaderCircle,
  Package,
  Plus,
  RefreshCw,
  RotateCcw,
  Trash2,
  Unplug,
  Wrench,
  X,
} from "lucide-react";
import type { Translate } from "./i18n";
import { useExtensionActions } from "./hooks/useExtensionActions";
import { ExtensionRow as ExtensionRowComponent } from "./extensions/ExtensionRow";
import { CustomIntegrationDrawer } from "./extensions/CustomIntegrationDrawer";
import { LocalInstallDialog } from "./extensions/LocalInstallDialog";
import { RemovalConfirmation } from "./extensions/RemovalConfirmation";
import { useImmediateState } from "./hooks/useImmediateState";
import { useTimedReset } from "./hooks/useTimedReset";

type ExtensionDistributionSource = "npm" | "local" | "built-in";
type ExtensionRuntimeOwnership = "bundled" | "system";
type ExtensionStateKind =
  | "enabled"
  | "disabled"
  | "broken";

export type Extension = {
  id: string;
  name: string;
  publisherId: string;
  publisherName: string;
  distributionSource: ExtensionDistributionSource;
  runtimeOwnership: ExtensionRuntimeOwnership;
  providerKind: "executable" | "static-descriptor" | "bundled-static";
  connected: boolean;
  runtimeSource: "managed" | "system" | "bundled";
  runtimeAvailable: boolean;
  reconnectAvailable: boolean;
  homepage: string | null;
  state: ExtensionStateKind;
  enabled: boolean;
  packageName: string | null;
  packageVersion: string;
  toolVersion: string | null;
  integrity: string | null;
  signatureVerified: boolean;
  officialVerified: boolean;
  currentVersion: string;
  previousVersion: string | null;
  manifestPath: string;
  executablePath: string;
  runtimeRoot: string | null;
  installedAt: number;
  updatedAt: number;
  pinned: boolean;
  channel: string;
  generatedCustom: boolean;
  recommended: boolean;
  /** Suggested from a convention-location manifest (~/.config/floter/tools). */
  manifestSuggestion?: boolean;
  toolLockState: "connected" | "reconnect-required" | "reverify-required" | null;
  toolCandidates: ExecutableToolCandidate[];
  /** Permission set recorded at approval time (audit trail). */
  approvedPermissions?: string[] | null;
  approvedAt?: number | null;
  approvedManifestDigest?: string | null;
  lastErrorCode?: string | null;
  lastErrorDetail?: string | null;
  lastErrorAt?: number | null;
  /** Why the extension is broken, persisted until repair succeeds. */
  brokenReason?: string | null;
};

type CommandDescriptor = {
  id: string;
  name: string;
  description: string;
  aliases: string[];
};

type ProviderDescription = {
  protocolVersion: string;
  provider: {
    id: string;
    name: string;
    version: string;
    description: string;
  };
  commands: CommandDescriptor[];
};

type ProviderResponse = {
  description: ProviderDescription;
  runtimeAvailable: boolean;
  cached: boolean;
  stderr: string | null;
};

type DiagnoseResult = {
  status: string;
  checks: Array<{ id: string; status: string; message: string }>;
};

type HealthReport = {
  schemaVersion: number;
  status: "healthy" | "degraded" | "unhealthy" | "unknown";
  checkedAt: string;
  capabilities: {
    version: string;
    supportedFeatures: string[];
    limitations: string[];
  };
  probes: Array<{
    probeId: string;
    passed: boolean;
    durationMs: number;
    exitCode: number | null;
    stderr: string;
  }>;
  failures: Array<{
    probe: string;
    exitCode: number | null;
    stderr: string;
    retryable: boolean;
  }>;
};

type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };
type ConfigFieldType = "text" | "password" | "path" | "select" | "multi-select" | "boolean" | "number";

type ConfigField = {
  key: string;
  type: ConfigFieldType;
  label: string;
  description: string;
  required: boolean;
  default: JsonValue | null;
  options: JsonValue[];
  minimum: number | null;
  maximum: number | null;
  minLength: number | null;
  maxLength: number | null;
  envVar: string | null;
  argument: string | null;
};

export type ExtensionExecutionPlan = {
  program: string;
  args: string[];
  mode: "pty" | "external";
  cwd: string | null;
  environment: Record<string, string>;
  inheritEnvironment: boolean;
  planToken?: string;
  argumentOverride?: string[];
};

type ExtensionConfiguration = {
  descriptor: {
    configVersion: number;
    owner: "host" | "tool";
    openCommand: string[];
    schema: ConfigField[];
    environmentMapping: Record<string, string>;
  };
  values: Record<string, JsonValue>;
  openPlan: ExtensionExecutionPlan | null;
};

type MutationKind = "enable" | "disable" | "install" | "repair" | "uninstall" | "save";
export type ExtensionOperation = { id: string; kind: MutationKind } | null;
type SyncOperation = "export" | "import";
type ConfigOperation = "copy" | "export" | null;
type CustomContentOperation = "copy" | "export" | null;
type RemovalTarget = Extension | null;
type PendingToolSelection = { extension: Extension; action: "connect" | "reconnect" } | null;
type PendingPermissionReview = {
  extension: Extension;
  executablePath: string | null;
  review: PermissionReview;
} | null;

type DiscoverySource =
  | "path"
  | "desktop"
  | "dpkg"
  | "rpm"
  | "pacman"
  | "flatpak"
  | "snap"
  | "nix"
  | "brew"
  | "launch-services"
  | "registry"
  | "scoop"
  | "chocolatey"
  | "win-get"
  | "wsl";

type ToolLocator =
  | { kind: "executable"; path: string }
  | { kind: "dockerImage"; reference: string; digest: string | null }
  | { kind: "flatpak"; appId: string }
  | { kind: "snap"; name: string };

type ToolCandidate = {
  id: string;
  name: string;
  locator: ToolLocator;
  version: string | null;
  sources: DiscoverySource[];
  quality: "official-adapter" | "native-support" | "auto-detected" | "user-defined" | "inferred";
  available: boolean;
  fingerprint: string | null;
};

export type ExecutableToolCandidate = ToolCandidate & { locator: Extract<ToolLocator, { kind: "executable" }> };

// Suggestion rows for the create-custom-integration drawer's executable
// picker: authored recommendations (shipped v-tools and convention-location
// manifests) and raw PATH discoveries. Both kinds end up as identical
// ToolBindings, but each keeps its own connect flow.
export type ToolSuggestion =
  | { kind: "recommendation"; extension: Extension }
  | { kind: "candidate"; candidate: ExecutableToolCandidate };

export type CustomIntegrationForm = {
  mode: "executable" | "script";
  id: string;
  name: string;
  command: string;
  version: string;
  executablePath: string;
  scriptLanguage: "js" | "shell" | "powershell";
  scriptContent: string;
  argsPrefix: string[];
  versionArgs: string[];
  permissions: PermissionName[];
  platforms: Array<"darwin" | "linux" | "windows">;
};

const CURRENT_PLATFORM: "darwin" | "linux" | "windows" = navigator.userAgent.includes("Mac") ? "darwin" : navigator.userAgent.includes("Win") ? "windows" : "linux";

const DEFAULT_CUSTOM_INTEGRATION: CustomIntegrationForm = {
  mode: "executable",
  id: "local.custom-tool",
  name: "Custom Tool",
  command: "custom-tool",
  version: "1.0.0",
  executablePath: "",
  scriptLanguage: "js",
  scriptContent: "",
  argsPrefix: [],
  versionArgs: [],
  permissions: ["environment"],
  platforms: [CURRENT_PLATFORM],
};

const FOCUSABLE_SELECTOR = [
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "a[href]",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

function useDialogFocus(
  active: boolean,
  dialogRef: RefObject<HTMLElement | null>,
  onEscape: () => void,
  interactive = true,
) {
  const escapeHandlerRef = useRef(onEscape);
  escapeHandlerRef.current = onEscape;
  const interactiveRef = useRef(interactive);
  interactiveRef.current = interactive;

  useEffect(() => {
    if (!active || !interactive) return;
    const previouslyFocused = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const focusInitial = window.setTimeout(() => {
      const dialog = dialogRef.current;
      const initial = dialog?.querySelector<HTMLElement>("[data-dialog-initial]");
      (initial ?? dialog?.querySelector<HTMLElement>(FOCUSABLE_SELECTOR) ?? dialog)?.focus();
    }, 0);
    const inertElements = new Map<HTMLElement, boolean>();
    let branch: HTMLElement | null = dialogRef.current?.parentElement ?? null;
    while (branch?.parentElement) {
      for (const sibling of Array.from(branch.parentElement.children)) {
        if (sibling !== branch && sibling instanceof HTMLElement && !sibling.classList.contains("extensions-toasts")) {
          inertElements.set(sibling, sibling.inert);
          sibling.inert = true;
        }
      }
      if (branch.parentElement.classList.contains("settings-card")) break;
      branch = branch.parentElement;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      const dialog = dialogRef.current;
      if (!dialog || !interactiveRef.current) return;
      if (event.isComposing || event.keyCode === 229) return;
      if (event.key === "Enter" && (event.target as HTMLElement)?.closest("[data-destructive-confirm]")) {
        event.preventDefault();
        event.stopPropagation();
        return;
      }
      // Cmd+W (macOS) / Ctrl+W (other platforms) dismisses the surface — a
      // convention every overlay in floter follows, alongside Escape below.
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w") {
        event.preventDefault();
        event.stopPropagation();
        escapeHandlerRef.current();
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        escapeHandlerRef.current();
        return;
      }
      if (event.key !== "Tab") return;
      const focusable = Array.from(dialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))
        .filter((element) => element.getClientRects().length > 0 && !element.closest("[inert]"));
      if (!focusable.length) {
        event.preventDefault();
        dialog.focus();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && (document.activeElement === first || !dialog.contains(document.activeElement))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && (document.activeElement === last || !dialog.contains(document.activeElement))) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", handleKeyDown, true);
    return () => {
      window.clearTimeout(focusInitial);
      document.removeEventListener("keydown", handleKeyDown, true);
      for (const [element, wasInert] of inertElements) element.inert = wasInert;
      window.setTimeout(() => {
        if (previouslyFocused?.isConnected && !previouslyFocused.closest("[inert]")) {
          previouslyFocused.focus({ preventScroll: true });
        }
      }, 0);
    };
  }, [active, dialogRef, interactive]);
}

type ExtensionSourceKey =
  | "settings.extensions.integrationKind.custom"
  | "settings.extensions.integrationKind.npm"
  | "settings.extensions.integrationKind.system"
  | "settings.extensions.integrationKind.package";

const integrationKindKey = (extension: Extension): ExtensionSourceKey => {
  if (extension.generatedCustom) return "settings.extensions.integrationKind.custom";
  if (extension.distributionSource === "npm") return "settings.extensions.integrationKind.npm";
  if (extension.distributionSource === "built-in" && extension.runtimeOwnership === "system") {
    return "settings.extensions.integrationKind.system";
  }
  return "settings.extensions.integrationKind.package";
};

const removalKind = (extension: Extension): "custom" | "npm" | "system" | "package" => {
  if (extension.generatedCustom) return "custom";
  if (extension.distributionSource === "npm") return "npm";
  if (extension.distributionSource === "built-in" && extension.runtimeOwnership === "system") return "system";
  return "package";
};

type ExtensionsExportResult = {
  path: string;
  extensionCount: number;
};

type ExtensionsImportItem = {
  id: string;
  message: string;
};

type ExtensionsImportReport = {
  path: string;
  succeeded: ExtensionsImportItem[];
  failed: ExtensionsImportItem[];
  skipped: ExtensionsImportItem[];
};

type ExtensionsPanelProps = {
  settingsBusy: boolean;
  t: Translate;
  locale: "en" | "zh";
  onOpenCommand: (plan: ExtensionExecutionPlan, label: string) => void | Promise<void>;
  /** Whether extension commands currently appear in launcher search results. */
  showCommandsInSearch: boolean;
  /** Flip the launcher command-discovery setting and persist it. */
  onToggleCommandsInSearch: () => void;
  /** Registered base plugins — built-in functionality that ships with floter
   * and is switched on/off here (the ONE obvious place). */
  basePlugins: BasePluginRow[];
  /** Enable/disable a base plugin; tears its runtime down when disabled. */
  onToggleBasePlugin: (id: string, enabled: boolean) => void;
};

export type BasePluginRow = {
  id: string;
  titleKey: Parameters<Translate>[0];
  descriptionKey: Parameters<Translate>[0];
  enabled: boolean;
};

type PermissionName =
  | "filesystem-read"
  | "filesystem-write"
  | "network-fetch"
  | "process-spawn"
  | "clipboard-read"
  | "clipboard-write"
  | "environment";

export type PermissionReview = {
  extensionId: string;
  extensionName: string;
  permissions: Array<{ permission: PermissionName; title: string; description: string }>;
  publisherSigned: boolean;
  officialVerified: boolean;
  deprecation: string | null;
};

export type InstallRequest = {
  source: "npm" | "linked";
  package: string | null;
  version: string | null;
  manifestPath: string | null;
  executablePath: string | null;
  approvedPermissions?: PermissionName[];
};

const errorMessage = (error: unknown): string =>
  error instanceof Error ? error.message : String(error);

const localErrorMessage = (error: unknown, t: Translate): string => {
  const message = errorMessage(error);
  if (message.startsWith("manifest_missing:")) return `${t("settings.extensions.localErrorMissing")}: ${t("settings.extensions.localFixMissing")}`;
  if (message.startsWith("manifest_invalid:")) return `${t("settings.extensions.localErrorInvalid")}: ${message.slice("manifest_invalid:".length).trim()}`;
  if (message.startsWith("manifest_incompatible:") || message.startsWith("platform_incompatible:")) return `${t("settings.extensions.localErrorPlatform")}: ${message.split(":").slice(1).join(":").trim()}`;
  if (message.startsWith("duplicate_id:") || message.includes("already installed")) return `${t("settings.extensions.localErrorDuplicate")}: ${t("settings.extensions.localFixDuplicate")}`;
  return message;
};

const displayJson = (value: JsonValue): string => {
  if (typeof value === "string") return value;
  if (value === null) return "";
  return JSON.stringify(value);
};

type PanelToast = { id: number; kind: "error" | "success"; text: string };

function ExtensionsToast({ toast, t, onDismiss }: { toast: PanelToast; t: Translate; onDismiss: (id: number) => void }) {
  useEffect(() => {
    const timer = window.setTimeout(() => onDismiss(toast.id), toast.kind === "error" ? 8000 : 4000);
    return () => window.clearTimeout(timer);
  }, [toast.id, toast.kind, onDismiss]);
  return (
    <div className={`extensions-toast extensions-toast--${toast.kind}`} role={toast.kind === "error" ? "alert" : "status"}>
      {toast.kind === "error"
        ? <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
        : <Check size={15} strokeWidth={2} aria-hidden="true" />}
      <span>{toast.text}</span>
      <button type="button" className="extensions-icon-button" aria-label={t("settings.extensions.dismissNotice")} onClick={() => onDismiss(toast.id)}><X size={13} strokeWidth={2} /></button>
    </div>
  );
}

export function ExtensionsPanel({ settingsBusy, t, locale, onOpenCommand, showCommandsInSearch, onToggleCommandsInSearch, basePlugins, onToggleBasePlugin }: ExtensionsPanelProps) {
  const [extensions, setExtensions] = useState<Extension[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [provider, setProvider] = useState<ProviderResponse | null>(null);
  const [diagnose, setDiagnose] = useState<DiagnoseResult | null>(null);
  const [healthReport, setHealthReport] = useState<HealthReport | null>(null);
  const [healthLoading, setHealthLoading, healthLoadingRef] = useImmediateState(false);
  const [reprobingCommands, setReprobingCommands, reprobingRef] = useImmediateState(false);
  // Bumped to force the drawer's details effect (provider/diagnose/config)
  // to reload without a lock-entry change, e.g. after a command re-probe.
  const [detailReloadTick, setDetailReloadTick] = useState(0);
  const [configuration, setConfiguration] = useState<ExtensionConfiguration | null>(null);
  const [configValues, setConfigValues] = useState<Record<string, JsonValue>>({});
  const [savedConfigValues, setSavedConfigValues] = useState<Record<string, JsonValue>>({});
  const [configOperation, setConfigOperation, configOperationRef] = useImmediateState<ConfigOperation>(null);
  const [configNotice, setConfigNotice] = useState<string | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [pendingLocal, setPendingLocal] = useState<{ review: PermissionReview; request: InstallRequest; name: string; runtime: string; platforms: string[]; source: string } | null>(null);
  const [pendingToolSelection, setPendingToolSelection, toolSelectionRef] = useImmediateState<PendingToolSelection>(null);
  const [pendingPermissionReview, setPendingPermissionReview] = useState<PendingPermissionReview>(null);
  const [syncOperation, setSyncOperation, syncOperationRef] = useImmediateState<SyncOperation | null>(null);
  const [exportResult, setExportResult] = useState<ExtensionsExportResult | null>(null);
  const [importReport, setImportReport] = useState<ExtensionsImportReport | null>(null);
  const [showCustomIntegration, setShowCustomIntegration] = useState(false);
  const [editingCustomId, setEditingCustomId] = useState<string | null>(null);
  const [customIntegrationLoading, setCustomIntegrationLoading, customLoadingRef] = useImmediateState(false);
  const [customIntegrationError, setCustomIntegrationError] = useState<string | null>(null);
  const [customContentOperation, setCustomContentOperation, customContentRef] = useImmediateState<CustomContentOperation>(null);
  const [customIntegration, setCustomIntegration] = useState<CustomIntegrationForm>(DEFAULT_CUSTOM_INTEGRATION);
  const [toolResults, setToolResults] = useState<ExecutableToolCandidate[]>([]);
  const [toolSearching, setToolSearching] = useState(false);
  const [toolSearchFailed, setToolSearchFailed] = useState(false);
  const [toolHighlight, setToolHighlight] = useState(0);
  const [customDirty, setCustomDirty] = useState(false);
  // Inline discard confirmations (replacing window.confirm): armed while the
  // bar above a drawer footer asks "discard unsaved changes?". Cleared as
  // soon as the user edits again, saves, or dismisses the bar.
  const [detailsDiscardArmed, setDetailsDiscardArmed] = useState(false);
  const [customDiscardArmed, setCustomDiscardArmed] = useState(false);
  const customSavedRef = useRef<CustomIntegrationForm>(DEFAULT_CUSTOM_INTEGRATION);
  const customGeneration = useRef(0);
  const toolSearchNeedsRefresh = useRef(true);
  const suppressToolSearch = useRef(false);
  const [removalTarget, setRemovalTarget] = useState<RemovalTarget>(null);
  const [toasts, setToasts] = useState<PanelToast[]>([]);
  const toastIdRef = useRef(0);
  const detailGeneration = useRef(0);
  const customCreateButtonRef = useRef<HTMLButtonElement | null>(null);
  const localDialogRef = useRef<HTMLElement | null>(null);
  const toolSelectionDialogRef = useRef<HTMLElement | null>(null);
  const permissionReviewDialogRef = useRef<HTMLElement | null>(null);
  const customDialogRef = useRef<HTMLElement | null>(null);
  const toolResultsRef = useRef<HTMLDivElement | null>(null);
  const drawerRef = useRef<HTMLElement | null>(null);
  const refreshGeneration = useRef(0);
  const refreshPending = useRef<Promise<void> | null>(null);
  const refreshRef = useRef<() => Promise<void>>(async () => {});
  const dismissToast = useCallback((id: number) => {
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }, []);
  const pushToast = useCallback((kind: PanelToast["kind"], text: string) => {
    const id = ++toastIdRef.current;
    setToasts((current) => [...current, { id, kind, text }].slice(-3));
  }, []);
  const showError = useCallback((text: string) => pushToast("error", text), [pushToast]);
  const showSuccess = useCallback((text: string) => pushToast("success", text), [pushToast]);
  const refreshAfterMutation = async () => {
    await refreshPending.current;
    await refreshRef.current();
  };
  const extensionActions = useExtensionActions({ refresh: refreshAfterMutation, onError: (nextError) => showError(errorMessage(nextError)), onComplete: () => showSuccess(t("settings.extensions.operationComplete")) });
  // One owner for busy state. A local mirror used to shadow it, so operations
  // that set it directly were invisible to runMutation's guard (and vice
  // versa) — two mutations could run at once.
  const busy = extensionActions.busy as ExtensionOperation;
  const setBusy = extensionActions.setBusy;
  const busyRef = extensionActions.busyRef;
  useTimedReset(removalTarget, () => setRemovalTarget(null));
  useTimedReset(detailsDiscardArmed, () => setDetailsDiscardArmed(false));
  useTimedReset(customDiscardArmed, () => setCustomDiscardArmed(false));

  const updateCustomIntegration = (update: (current: CustomIntegrationForm) => CustomIntegrationForm) => {
    setCustomIntegrationError(null);
    setCustomDiscardArmed(false);
    setCustomIntegration((current) => {
      const next = update(current);
      setCustomDirty(JSON.stringify(next) !== JSON.stringify(customSavedRef.current));
      return next;
    });
  };

  const resetCustomIntegration = () => {
    customGeneration.current += 1;
    setCustomIntegrationLoading(false);
    setEditingCustomId(null);
    const fresh = { ...DEFAULT_CUSTOM_INTEGRATION, argsPrefix: [], versionArgs: [], permissions: [...DEFAULT_CUSTOM_INTEGRATION.permissions], platforms: [CURRENT_PLATFORM] as Array<"darwin" | "linux" | "windows"> };
    setCustomIntegration(fresh);
    setCustomIntegrationError(null);
    setToolResults([]);
    setToolSearchFailed(false);
    setToolHighlight(0);
    toolSearchNeedsRefresh.current = true;
    setCustomDirty(false);
    setCustomDiscardArmed(false);
    customSavedRef.current = fresh;
  };

  const closeCustomIntegration = () => {
    if (customDiscardArmed) {
      setCustomDiscardArmed(false);
      return;
    }
    if (customDirty) {
      setCustomDiscardArmed(true);
      return;
    }
    setShowCustomIntegration(false);
    resetCustomIntegration();
  };

  const discardCustomIntegration = () => {
    setCustomDiscardArmed(false);
    setShowCustomIntegration(false);
    resetCustomIntegration();
  };

  const scriptTemplate = (language: CustomIntegrationForm["scriptLanguage"]) => ({
    js: "#!/usr/bin/env node\n\n// Floter provider script\n",
    shell: "#!/bin/sh\n\n# Floter provider script\n",
    powershell: "#!/usr/bin/env pwsh\n\n# Floter provider script\n",
  }[language]);

  const connectedExtensions = useMemo(() => extensions.filter((extension) => extension.connected), [extensions]);
  const suggestedExtensions = useMemo(() => extensions.filter((extension) => !extension.connected), [extensions]);
  const connectedPaths = useMemo(
    () => new Set(connectedExtensions.map((extension) => extension.executablePath)),
    [connectedExtensions],
  );
  const selected = extensions.find((extension) => extension.id === selectedId) ?? null;
  const configDirty = configuration?.descriptor.owner === "host"
    && JSON.stringify(configValues) !== JSON.stringify(savedConfigValues);

  const refreshOfficialStatus = async (generation: number) => {
    const statuses = await invoke<Record<string, boolean>>("extensions_refresh_official_status");
    if (generation !== refreshGeneration.current) return;
    setExtensions((current) => current.map((extension) => ({
      ...extension,
      officialVerified: statuses[extension.id] ?? false,
    })));
  };

  const refreshData = async () => {
    const generation = ++refreshGeneration.current;
    setLoading(true);
    try {
      const entries = await invoke<Extension[]>("extensions_list");
      if (generation !== refreshGeneration.current) return;
      setExtensions(entries);
      void refreshOfficialStatus(generation).catch(() => {});
    } catch (nextError) {
      if (generation === refreshGeneration.current) showError(localErrorMessage(nextError, t));
    } finally {
      if (generation === refreshGeneration.current) setLoading(false);
    }
  };
  const refresh = () => {
    if (refreshPending.current) return refreshPending.current;
    const request = refreshData().finally(() => { refreshPending.current = null; });
    refreshPending.current = request;
    return request;
  };
  refreshRef.current = refresh;

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    if (!showCustomIntegration || customIntegration.mode !== "executable") return;
    if (suppressToolSearch.current) {
      suppressToolSearch.current = false;
      setToolSearching(false);
      setToolSearchFailed(false);
      setToolResults([]);
      return;
    }
    const query = customIntegration.executablePath.trim();
    let cancelled = false;
    const timer = window.setTimeout(() => {
      const forceRefresh = toolSearchNeedsRefresh.current;
      toolSearchNeedsRefresh.current = false;
      setToolSearching(true);
      setToolSearchFailed(false);
      void invoke<ToolCandidate[]>("extensions_search_tools", { query, limit: 12, forceRefresh, executableOnly: true })
        .then((results) => {
          if (cancelled) return;
          setToolResults(results.filter((candidate): candidate is ExecutableToolCandidate =>
            candidate.locator.kind === "executable"
              && candidate.available
              && !connectedPaths.has(candidate.locator.path),
          ));
          setToolHighlight(0);
        })
        .catch(() => {
          if (cancelled) return;
          toolSearchNeedsRefresh.current = true;
          setToolResults([]);
          setToolSearchFailed(true);
        })
        .finally(() => { if (!cancelled) setToolSearching(false); });
    }, 120);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [showCustomIntegration, customIntegration.mode, customIntegration.executablePath, connectedPaths]);

  useEffect(() => {
    toolResultsRef.current
      ?.querySelector<HTMLElement>("[aria-selected='true']")
      ?.scrollIntoView({ block: "nearest" });
  }, [toolHighlight]);

  useEffect(() => {
    if (!selectedId) {
      detailGeneration.current += 1;
      setProvider(null);
      setDiagnose(null);
      setHealthReport(null);
      setHealthLoading(false);
      setConfiguration(null);
      setConfigValues({});
      setSavedConfigValues({});
      setConfigNotice(null);
      setDetailError(null);
      return;
    }
    const generation = ++detailGeneration.current;
    setDetailLoading(true);
    setProvider(null);
    setDiagnose(null);
    setHealthReport(null);
    setHealthLoading(false);
    setConfiguration(null);
    setDetailError(null);
    void Promise.allSettled([
      invoke<ProviderResponse>("extensions_describe", { id: selectedId }),
      invoke<DiagnoseResult>("extensions_diagnose", { id: selectedId }),
      invoke<ExtensionConfiguration>("extensions_config_get", { id: selectedId }),
      invoke<HealthReport>("extensions_health", { id: selectedId }),
    ]).then(([descriptionResult, diagnoseResult, configResult, healthResult]) => {
      if (generation !== detailGeneration.current) return;
      if (descriptionResult.status === "fulfilled") setProvider(descriptionResult.value);
      if (diagnoseResult.status === "fulfilled") setDiagnose(diagnoseResult.value);
      if (healthResult.status === "fulfilled") setHealthReport(healthResult.value);
      if (configResult.status === "fulfilled") {
        setConfiguration(configResult.value);
        const defaults = Object.fromEntries(
          configResult.value.descriptor.schema
            .filter((field) => field.default !== null)
            .map((field) => [field.key, field.default]),
        );
        const values = { ...defaults, ...configResult.value.values };
        setConfigValues(values);
        setSavedConfigValues(values);
      }
      const primaryError = descriptionResult.status === "rejected"
        ? descriptionResult.reason
        : diagnoseResult.status === "rejected" && configResult.status === "rejected"
          ? diagnoseResult.reason
          : null;
      if (primaryError) setDetailError(errorMessage(primaryError));
      setDetailLoading(false);
    });
  }, [selectedId, selected?.updatedAt, detailReloadTick]);

  const handleReprobe = async () => {
    if (!selectedId || healthLoadingRef.current) return;
    const generation = detailGeneration.current;
    setHealthLoading(true);
    setDetailError(null);
    try {
      const report = await invoke<HealthReport>("extensions_reprobe", { id: selectedId });
      if (generation === detailGeneration.current) setHealthReport(report);
    } catch (nextError) {
      if (generation === detailGeneration.current) {
        setDetailError(errorMessage(nextError));
        showError(errorMessage(nextError));
      }
    } finally {
      if (generation === detailGeneration.current) setHealthLoading(false);
    }
  };

  const handleReprobeCommands = async () => {
    if (!selected || reprobingRef.current || busyRef.current) return;
    setReprobingCommands(true);
    try {
      const report = await invoke<{ rootArguments: number; subcommands: number }>(
        "extensions_reprobe_commands",
        { id: selected.id },
      );
      await refreshAfterMutation();
      setDetailReloadTick((tick) => tick + 1);
      showSuccess(t("settings.extensions.reprobedCommands", { subcommands: report.subcommands, arguments: report.rootArguments }));
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setReprobingCommands(false);
    }
  };

  useDialogFocus(Boolean(pendingLocal), localDialogRef, () => setPendingLocal(null));
  useDialogFocus(Boolean(pendingToolSelection), toolSelectionDialogRef, () => setPendingToolSelection(null));
  useDialogFocus(Boolean(pendingPermissionReview), permissionReviewDialogRef, () => setPendingPermissionReview(null));
  useDialogFocus(showCustomIntegration, customDialogRef, closeCustomIntegration);
  useDialogFocus(Boolean(selectedId), drawerRef, () => closeDetails(), !showCustomIntegration && !pendingLocal && !pendingToolSelection && !pendingPermissionReview);

  useEffect(() => {
    if (!configDirty) setDetailsDiscardArmed(false);
  }, [configDirty]);

  useEffect(() => {
    if (!showCustomIntegration || customIntegrationLoading) return;
    customDialogRef.current?.querySelector<HTMLElement>("[data-dialog-initial]")?.focus();
  }, [showCustomIntegration, customIntegrationLoading]);

  const runMutation = async (id: string, kind: MutationKind, action: () => Promise<unknown>): Promise<boolean> => {
    const result = await extensionActions.runMutation(id, kind, action);
    if (result && kind === "uninstall") setSelectedId(null);
    return result;
  };

  const exportExtensions = async () => {
    if (syncOperationRef.current) return;
    setSyncOperation("export");
    setExportResult(null);
    setImportReport(null);
    try {
      const result = await invoke<ExtensionsExportResult | null>("extensions_export");
      if (result) setExportResult(result);
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setSyncOperation(null);
    }
  };

  const importExtensions = async () => {
    if (syncOperationRef.current) return;
    setSyncOperation("import");
    setExportResult(null);
    setImportReport(null);
    try {
      const report = await invoke<ExtensionsImportReport | null>("extensions_import", { locale });
      if (report) {
        setImportReport(report);
        await refreshAfterMutation();
      }
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setSyncOperation(null);
    }
  };

  const toggleExtension = (extension: Extension) => runMutation(
    extension.id,
    extension.enabled ? "disable" : "enable",
    () => invoke(extension.enabled ? "extensions_disable" : "extensions_enable", { id: extension.id }),
  );

  // Re-check verifies the installed extension and repairs it when needed;
  // the toast reports which of the two happened.
  const repairExtension = async (extension: Extension) => {
    if (busyRef.current) return;
    extensionActions.setBusy({ id: extension.id, kind: "repair" });
    try {
      const report = await invoke<{ repaired: boolean }>("extensions_repair", { id: extension.id });
      await refreshAfterMutation();
      showSuccess(t(report.repaired ? "settings.extensions.repairedNotice" : "settings.extensions.recheckedNotice"));
    } catch (error) {
      showError(errorMessage(error));
    } finally {
      extensionActions.setBusy(null);
    }
  };

  const uninstallExtension = (extension: Extension) => setRemovalTarget(extension);

  const confirmRemoval = async () => {
    if (!removalTarget || busyRef.current) return;
    const extension = removalTarget;
    const removed = await runMutation(
      extension.id,
      "uninstall",
      () => invoke("extensions_uninstall", { id: extension.id, removeData: false }),
    );
    if (removed) {
      setRemovalTarget(null);
      showSuccess(t(
        removalKind(extension) === "custom"
          ? "settings.extensions.customDeleted"
          : removalKind(extension) === "npm"
            ? "settings.extensions.uninstalledNotice"
            : removalKind(extension) === "system"
              ? "settings.extensions.disconnectedNotice"
              : "settings.extensions.packageRemovedNotice",
        { name: extension.name },
      ));
    }
  };

  const connectRecommendedAt = async (extension: Extension, executablePath: string | null) => {
    if (busyRef.current || !extension.runtimeAvailable) return;
    const selection = toolSelectionRef.current;
    setBusy({ id: extension.id, kind: "install" });
    try {
      const review = await invoke<PermissionReview>("extensions_recommended_permissions", { id: extension.id, locale });
      if (selection && toolSelectionRef.current !== selection) return;
      if (review.permissions.length) {
        setPendingToolSelection(null);
        setPendingPermissionReview({ extension, executablePath, review });
        setBusy(null);
        return;
      }
      await invoke("extensions_connect_recommended", {
        id: extension.id,
        executablePath,
        approvedPermissions: [],
      });
      await refreshAfterMutation();
      showSuccess(t("settings.extensions.connectedNotice", { name: extension.name }));
      setPendingToolSelection(null);
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  const confirmPermissionReview = async () => {
    if (!pendingPermissionReview || busyRef.current) return;
    const { extension, executablePath, review } = pendingPermissionReview;
    setBusy({ id: extension.id, kind: "install" });
    try {
      await invoke("extensions_connect_recommended", {
        id: extension.id,
        executablePath,
        approvedPermissions: review.permissions.map(({ permission }) => permission),
      });
      await refreshAfterMutation();
      showSuccess(t("settings.extensions.connectedNotice", { name: extension.name }));
      setPendingPermissionReview(null);
      setPendingToolSelection(null);
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  const connectRecommended = (extension: Extension) => {
    if (busyRef.current) return;
    if (extension.toolCandidates.length > 1) {
      setPendingToolSelection({ extension, action: "connect" });
      return;
    }
    void connectRecommendedAt(extension, extension.toolCandidates[0]?.locator.path ?? null);
  };

  const connectLocal = async () => {
    if (busyRef.current) return;
    setBusy({ id: "local", kind: "install" });
    try {
      const manifestPath = await invoke<string | null>("extensions_pick_local_package");
      if (!manifestPath) return;
      const request: InstallRequest = {
        source: "linked",
        package: null,
        version: null,
        manifestPath,
        executablePath: null,
      };
      const details = await invoke<{ extensionName: string; runtime: string; platforms: string[]; source: string; permissions: PermissionReview }>("extensions_local_manifest_review", { manifestPath, locale });
      setPendingLocal({ review: details.permissions, request, name: details.extensionName, runtime: details.runtime, platforms: details.platforms, source: details.source });
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  const confirmLocal = async () => {
    if (!pendingLocal || busyRef.current) return;
    const pending = pendingLocal;
    setBusy({ id: "local", kind: "install" });
    try {
      await invoke("extensions_install", { request: { ...pending.request, approvedPermissions: pending.review.permissions.map(({ permission }) => permission) } });
      await refreshAfterMutation();
      setPendingLocal(null);
      showSuccess(t("settings.extensions.connectedNotice", { name: pending.name }));
    } catch (nextError) { showError(errorMessage(nextError)); }
    finally { setBusy(null); }
  };

  const openCreateCustomIntegration = () => {
    if (busyRef.current || showCustomIntegration) return;
    resetCustomIntegration();
    setShowCustomIntegration(true);
  };

  const editCustomIntegration = async (extension: Extension) => {
    if (!extension.generatedCustom || busyRef.current || customLoadingRef.current) return;
    resetCustomIntegration();
    const generation = customGeneration.current;
    setCustomIntegrationLoading(true);
    setEditingCustomId(extension.id);
    setCustomIntegrationError(null);
    setShowCustomIntegration(true);
    try {
      const definition = await invoke<CustomIntegrationForm>("extensions_custom_get", { id: extension.id });
      if (generation !== customGeneration.current) return;
      setCustomIntegration({
        ...definition,
        scriptLanguage: definition.scriptLanguage ?? "shell",
        scriptContent: definition.scriptContent ?? "",
        argsPrefix: [...definition.argsPrefix],
        versionArgs: [...definition.versionArgs],
        permissions: [...definition.permissions],
        platforms: [...definition.platforms],
      });
      customSavedRef.current = { ...definition, scriptLanguage: definition.scriptLanguage ?? "shell", scriptContent: definition.scriptContent ?? "", argsPrefix: [...definition.argsPrefix], versionArgs: [...definition.versionArgs], permissions: [...definition.permissions], platforms: [...definition.platforms] };
      setCustomDirty(false);
    } catch (nextError) {
      if (generation === customGeneration.current) setCustomIntegrationError(errorMessage(nextError));
    } finally {
      if (generation === customGeneration.current) setCustomIntegrationLoading(false);
    }
  };

  const chooseToolCandidate = (candidate: ExecutableToolCandidate) => {
    const command = candidate.name.replace(/\.(exe|cmd|bat)$/i, "");
    const slug = command.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "custom-tool";
    suppressToolSearch.current = true;
    updateCustomIntegration((current) => ({
      ...current,
      executablePath: candidate.locator.path,
      name: current.name === DEFAULT_CUSTOM_INTEGRATION.name ? command : current.name,
      command: current.command === DEFAULT_CUSTOM_INTEGRATION.command ? slug : current.command,
      id: current.id === DEFAULT_CUSTOM_INTEGRATION.id ? `local.${slug}` : current.id,
    }));
    setToolResults([]);
  };

  // Suggestions for the drawer's executable picker. While the executable path
  // is empty (idle state) authored recommendations come first, followed by a
  // device-wide PATH scan minus anything already connected; once the user
  // types, normal search results take over through the same list.
  const toolSuggestions = useMemo<ToolSuggestion[]>(() => {
    const candidates = toolResults.map((candidate): ToolSuggestion => ({ kind: "candidate", candidate }));
    if (customIntegration.executablePath.trim() || customDirty || editingCustomId) return candidates;
    return [
      ...suggestedExtensions.map((extension): ToolSuggestion => ({ kind: "recommendation", extension })),
      ...candidates,
    ];
  }, [toolResults, suggestedExtensions, customIntegration.executablePath, customDirty, editingCustomId]);

  const chooseToolSuggestion = (item: ToolSuggestion) => {
    if (item.kind === "candidate") {
      chooseToolCandidate(item.candidate);
      return;
    }
    // Recommendations carry an authored manifest, so they connect through the
    // recommendation pipeline (permission review, confirm text, multi-candidate
    // chooser) instead of the generic custom-integration form. Close the drawer
    // without a discard prompt — nothing was edited.
    setShowCustomIntegration(false);
    resetCustomIntegration();
    void connectRecommended(item.extension);
  };

  const handleToolSearchKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
    if (!toolSuggestions.length) return;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setToolHighlight((index) => Math.min(index + 1, toolSuggestions.length - 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setToolHighlight((index) => Math.max(index - 1, 0));
    } else if (event.key === "Enter") {
      event.preventDefault();
      chooseToolSuggestion(toolSuggestions[toolHighlight]);
    }
  };

  const createCustomIntegration = async (event: FormEvent) => {
    event.preventDefault();
    if (busyRef.current || customLoadingRef.current) return;
    const generation = customGeneration.current;
    setBusy({ id: customIntegration.id, kind: editingCustomId ? "save" : "install" });
    setCustomIntegrationError(null);
    try {
      const request = {
        ...customIntegration,
        executablePath: customIntegration.mode === "executable" ? customIntegration.executablePath : "",
        scriptLanguage: customIntegration.mode === "script" ? customIntegration.scriptLanguage : null,
        scriptContent: customIntegration.mode === "script" ? customIntegration.scriptContent : null,
        argsPrefix: customIntegration.argsPrefix,
        versionArgs: customIntegration.versionArgs,
      };
      await invoke(editingCustomId ? "extensions_custom_update" : "extensions_create_custom", {
        ...(editingCustomId ? { id: editingCustomId } : {}),
        request: {
          ...request,
        },
      });
      const notice = editingCustomId
        ? t("settings.extensions.customUpdated", { name: customIntegration.name })
        : t("settings.extensions.customCreated", { name: customIntegration.name });
      if (generation === customGeneration.current) {
        setShowCustomIntegration(false);
        resetCustomIntegration();
      }
      showSuccess(notice);
      await refreshAfterMutation();
    } catch (nextError) {
      const message = errorMessage(nextError);
      if (generation === customGeneration.current) setCustomIntegrationError(message);
      showError(message);
    } finally {
      setBusy(null);
    }
  };

  const copyCustomContent = async (content: string, notice: string) => {
    if (customContentRef.current) return;
    setCustomContentOperation("copy");
    setCustomIntegrationError(null);
    try {
      await navigator.clipboard.writeText(content);
      showSuccess(notice);
    } catch (nextError) {
      setCustomIntegrationError(errorMessage(nextError));
    } finally {
      setCustomContentOperation(null);
    }
  };
  const copyExecutionPlan = async () => {
    const plan = JSON.stringify({ program: customIntegration.executablePath, args: [...customIntegration.argsPrefix], mode: "pty" }, null, 2);
    await copyCustomContent(plan, t("settings.extensions.customPlanCopied"));
  };
  const exportCustomScript = async () => {
    if (customContentRef.current) return;
    setCustomContentOperation("export");
    setCustomIntegrationError(null);
    const extension = customIntegration.scriptLanguage === "shell" ? "sh" : customIntegration.scriptLanguage === "powershell" ? "ps1" : "js";
    try {
      const path = await invoke<string | null>("extensions_custom_export_script", { id: customIntegration.id, content: customIntegration.scriptContent, extension });
      if (path) showSuccess(t("settings.extensions.customScriptExported"));
    } catch (nextError) {
      setCustomIntegrationError(errorMessage(nextError));
    } finally {
      setCustomContentOperation(null);
    }
  };

  const reconnectSystemAt = async (extension: Extension, executablePath: string | null) => {
    const reconnected = await runMutation(
      extension.id,
      "repair",
      () => invoke("extensions_reconnect_system", { id: extension.id, executablePath }),
    );
    if (reconnected) setPendingToolSelection(null);
  };

  const reconnectSystem = (extension: Extension) => {
    if (busyRef.current) return;
    if (extension.toolCandidates.length > 1) {
      setPendingToolSelection({ extension, action: "reconnect" });
      return;
    }
    void reconnectSystemAt(extension, extension.toolCandidates[0]?.locator.path ?? null);
  };

  const chooseSystemTool = (candidate: ExecutableToolCandidate) => {
    if (!pendingToolSelection) return;
    const { extension, action } = pendingToolSelection;
    if (action === "connect") void connectRecommendedAt(extension, candidate.locator.path);
    else void reconnectSystemAt(extension, candidate.locator.path);
  };

  const saveConfiguration = async () => {
    if (busyRef.current || !selected || !configuration || configuration.descriptor.owner !== "host") return;
    setBusy({ id: selected.id, kind: "save" });
    setDetailError(null);
    try {
      const saved = await invoke<ExtensionConfiguration>("extensions_config_set", {
        id: selected.id,
        values: configValues,
      });
      setConfiguration(saved);
      setConfigValues(saved.values);
      setSavedConfigValues(saved.values);
      setConfigNotice(t("settings.extensions.configSaved"));
      showSuccess(t("settings.extensions.configSaved"));
    } catch (nextError) {
      const message = errorMessage(nextError);
      setDetailError(message);
      showError(message);
    } finally {
      setBusy(null);
    }
  };

  const configurationDefaults = () => Object.fromEntries(
    configuration?.descriptor.schema
      .filter((field) => field.default !== null)
      .map((field) => [field.key, field.default]) ?? [],
  );

  const copyConfiguration = async () => {
    if (!selected || !configuration || configuration.descriptor.owner !== "host" || configOperationRef.current) return;
    setConfigOperation("copy");
    setDetailError(null);
    setConfigNotice(null);
    try {
      const json = await invoke<string>("extensions_config_copy", { id: selected.id, values: configValues });
      await navigator.clipboard.writeText(json);
      setConfigNotice(t("settings.extensions.configCopied"));
      showSuccess(t("settings.extensions.configCopied"));
    } catch (nextError) {
      const message = errorMessage(nextError);
      setDetailError(message);
      showError(message);
    } finally {
      setConfigOperation(null);
    }
  };

  const exportConfiguration = async () => {
    if (!selected || !configuration || configuration.descriptor.owner !== "host" || configOperationRef.current) return;
    setConfigOperation("export");
    setDetailError(null);
    setConfigNotice(null);
    try {
      const path = await invoke<string | null>("extensions_config_export", { id: selected.id, values: configValues });
      if (path) {
        setConfigNotice(t("settings.extensions.configExported"));
        showSuccess(t("settings.extensions.configExported"));
      }
    } catch (nextError) {
      const message = errorMessage(nextError);
      setDetailError(message);
      showError(message);
    } finally {
      setConfigOperation(null);
    }
  };

  const closeDetails = () => {
    if (removalTarget) { setRemovalTarget(null); return; }
    if (detailsDiscardArmed) { setDetailsDiscardArmed(false); return; }
    if (configDirty) {
      setDetailsDiscardArmed(true);
      return;
    }
    setSelectedId(null);
  };

  const discardDetailsChanges = () => {
    setDetailsDiscardArmed(false);
    setSelectedId(null);
  };

  const removalTextKey = (extension: Extension, suffix: "" | "Title" | "Description") => {
    const kind = removalKind(extension);
    const stem = kind === "custom"
      ? "deleteCustom"
      : kind === "npm"
        ? "uninstall"
        : kind === "system"
          ? "disconnect"
          : "removePackage";
    return `settings.extensions.${stem}${suffix}` as Parameters<Translate>[0];
  };

  const stopRowClick = (event: MouseEvent) => event.stopPropagation();
  const removalConfirmation = removalTarget && <RemovalConfirmation extension={removalTarget} busy={Boolean(busy)} t={t} textKey={removalTextKey} onCancel={() => setRemovalTarget(null)} onConfirm={() => void confirmRemoval()} />;

  return (
    <section className="settings-section extensions-panel" data-no-drag>
      <div className="settings-section__heading extensions-panel__heading">
        <div>
          <h2 className="settings-section__label">{t("settings.extensions.title")}</h2>
          <p className="settings-section__hint">{t("settings.extensions.hint")}</p>
        </div>
        <button
          type="button"
          className="extensions-icon-button"
          aria-label={t("settings.extensions.refresh")}
          title={t("settings.extensions.refresh")}
          disabled={loading}
          onClick={() => void refresh()}
        >
          {loading
            ? <LoaderCircle className="extensions-spinner" size={16} strokeWidth={2} aria-hidden="true" />
            : <RefreshCw size={16} strokeWidth={2} aria-hidden="true" />}
        </button>
      </div>

      <div className="settings-option extensions-panel__search-toggle">
        <span className="settings-option__main">
          <span className="settings-option__label">
            {t("settings.extensions.showInSearch")}
          </span>
          <span className="settings-option__description">
            {t("settings.extensions.showInSearchHint")}
          </span>
        </span>
        <button
          type="button"
          className={`settings-switch${showCommandsInSearch ? " settings-switch--active" : ""}`}
          role="switch"
          aria-checked={showCommandsInSearch}
          disabled={settingsBusy}
          aria-busy={settingsBusy}
          aria-label={t("settings.extensions.showInSearch")}
          onMouseDown={(event) => event.preventDefault()}
          onClick={onToggleCommandsInSearch}
        >
          <span className="settings-switch__thumb" />
        </button>
      </div>

      <div className="extensions-installed">
        <div className="extensions-sync-cluster">
          <div className="extensions-sync-toolbar">
            <div className="extensions-sync-toolbar__group">
              <button
                type="button"
                className="extensions-action-button extensions-action-button--primary"
                disabled={Boolean(syncOperation) || Boolean(busy) || loading}
                ref={customCreateButtonRef}
                onClick={openCreateCustomIntegration}
              >
                <Plus size={14} strokeWidth={2} aria-hidden="true" />
                {t("settings.extensions.createCustom")}
              </button>
              <button
                type="button"
                className="extensions-action-button"
                disabled={Boolean(syncOperation) || Boolean(busy) || loading}
                title={t("settings.extensions.chooseManifestHint")}
                onClick={() => void connectLocal()}
              >
                <Link2 size={14} strokeWidth={2} aria-hidden="true" />
                {t("settings.extensions.chooseManifest")}
              </button>
            </div>
            <div className="extensions-sync-toolbar__group extensions-sync-toolbar__group--transfer">
              <button
                type="button"
                className="extensions-action-button"
                aria-busy={syncOperation === "export"}
                disabled={Boolean(syncOperation) || Boolean(busy) || loading}
                onClick={() => void exportExtensions()}
              >
                {syncOperation === "export" ? <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" /> : <FileDown size={14} strokeWidth={2} aria-hidden="true" />}
                {syncOperation === "export" ? t("settings.extensions.exporting") : t("settings.extensions.export")}
              </button>
              <button
                type="button"
                className="extensions-action-button"
                aria-busy={syncOperation === "import"}
                disabled={Boolean(syncOperation) || Boolean(busy) || loading}
                onClick={() => void importExtensions()}
              >
                {syncOperation === "import" ? <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" /> : <FileUp size={14} strokeWidth={2} aria-hidden="true" />}
                {syncOperation === "import" ? t("settings.extensions.importing") : t("settings.extensions.import")}
              </button>
            </div>
          </div>
          <div className="extensions-package-hints">
            <div className="extensions-package-hint" role="note">
              <FileDown size={13} strokeWidth={2} aria-hidden="true" />
              <span>{t("settings.extensions.fileTransferHint")}</span>
            </div>
            <div className="extensions-package-hint" role="note">
              <Link2 size={13} strokeWidth={2} aria-hidden="true" />
              <span>{t("settings.extensions.chooseManifestHint")}</span>
            </div>
          </div>
        </div>
        {exportResult && (
          <div className="extensions-notice extensions-notice--success" role="status" title={exportResult.path}>
            <Check size={15} strokeWidth={2} aria-hidden="true" />
            <span>{t("settings.extensions.exportComplete", { count: exportResult.extensionCount })}</span>
          </div>
        )}
        {importReport && (
          <div
            className={`extensions-notice${importReport.failed.length ? " extensions-notice--error" : " extensions-notice--success"}`}
            role="status"
            title={importReport.failed.map((item) => `${item.id}: ${item.message}`).join("\n") || importReport.path}
          >
            {importReport.failed.length
              ? <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
              : <Check size={15} strokeWidth={2} aria-hidden="true" />}
            <span>{importReport.failed.length
              ? t("settings.extensions.importRolledBack")
              : t("settings.extensions.importSummary", {
                  succeeded: importReport.succeeded.length,
                  skipped: importReport.skipped.length,
                })}</span>
          </div>
        )}

        <section className="extensions-section extensions-section--base">
          <h3 className="extensions-section-title">
            <span>{t("settings.plugins.baseSection")}</span>
            <span className="extension-status">{basePlugins.length}</span>
          </h3>
          <p className="settings-section__hint extensions-base-plugin__hint">{t("settings.plugins.baseHint")}</p>
          <div className="extensions-list extensions-list--base">
            {basePlugins.map((plugin) => (
              <div key={plugin.id} className="extensions-base-plugin">
                <span className="extensions-base-plugin__main">
                  <span className="extensions-base-plugin__name">{t(plugin.titleKey)}</span>
                  <span className="extensions-base-plugin__description">{t(plugin.descriptionKey)}</span>
                </span>
                <button
                  type="button"
                  className={`settings-switch${plugin.enabled ? " settings-switch--active" : ""}`}
                  role="switch"
                  aria-checked={plugin.enabled}
                  disabled={settingsBusy}
                  aria-busy={settingsBusy}
                  aria-label={t(plugin.titleKey)}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => onToggleBasePlugin(plugin.id, !plugin.enabled)}
                >
                  <span className="settings-switch__thumb" />
                </button>
              </div>
            ))}
            <p className="extensions-base-plugin__privacy settings-privacy-hint">
              {t("settings.clipboardPrivacy")}
            </p>
          </div>
        </section>
        <section className="extensions-section">
          <h3 className="extensions-section-title">
            <span>{t("settings.extensions.section.connected")}</span>
            <span className="extension-status">
              {loading
                ? <LoaderCircle className="extensions-spinner" size={11} strokeWidth={2} aria-hidden="true" />
                : connectedExtensions.length}
            </span>
          </h3>
          <div className="extensions-list extensions-list--installed">
            {loading ? (
              <EmptyState icon={<LoaderCircle className="extensions-spinner" size={20} strokeWidth={2} />} text={t("settings.extensions.loading")} />
            ) : connectedExtensions.length === 0 ? (
              <EmptyState icon={<Package size={20} strokeWidth={2} />} text={t("settings.extensions.emptyInstalled")} />
            ) : connectedExtensions.map((extension) => (
              <Fragment key={extension.id}>
              <ExtensionRowComponent
                extension={extension}
                operation={busy}
                t={t}
                onOpen={() => setSelectedId(extension.id)}
                onRepair={() => void repairExtension(extension)}
                onReconnect={() => void reconnectSystem(extension)}
                onToggle={() => void toggleExtension(extension)}
                onEdit={() => void editCustomIntegration(extension)}
                onUninstall={() => uninstallExtension(extension)}
              />
              {!selected && removalTarget?.id === extension.id && removalConfirmation}
              </Fragment>
            ))}
          </div>
        </section>
      </div>
      {pendingLocal && <LocalInstallDialog pending={pendingLocal} busy={Boolean(busy)} t={t} dialogRef={localDialogRef} stopPropagation={stopRowClick} onCancel={() => setPendingLocal(null)} onConfirm={() => void confirmLocal()} />}
      {pendingPermissionReview && (
        <div className="extension-permission-backdrop" role="presentation" onMouseDown={() => setPendingPermissionReview(null)}>
          <section ref={permissionReviewDialogRef} className="extension-permission-dialog extension-permission-dialog--review" role="dialog" aria-modal="true" aria-labelledby="permission-review-title" tabIndex={-1} onMouseDown={stopRowClick}>
            <header>
              <AlertCircle size={18} strokeWidth={2} aria-hidden="true" />
              <div>
                <h3 id="permission-review-title">{t(pendingPermissionReview.extension.manifestSuggestion ? "settings.extensions.confirmConnectManifest" : "settings.extensions.confirmConnectRecommended", { name: pendingPermissionReview.extension.name })}</h3>
                <p>{pendingPermissionReview.executablePath ?? pendingPermissionReview.extension.executablePath}</p>
              </div>
            </header>
            <div className="extension-permission-list">
              <span className="extension-permission-list__label">{t("settings.extensions.permissionsRequired")}</span>
              {pendingPermissionReview.review.permissions.map((perm) => (
                <div key={perm.permission} className="extension-permission-item">
                  <strong>{perm.title}</strong>
                  <span>{perm.description}</span>
                </div>
              ))}
            </div>
            <footer>
              <button type="button" className="extensions-action-button" data-dialog-initial onClick={() => setPendingPermissionReview(null)}>{t("settings.extensions.cancel")}</button>
              <button type="button" className="extensions-action-button extensions-action-button--primary" disabled={Boolean(busy)} onClick={() => void confirmPermissionReview()}>{busy ? t("settings.extensions.connecting") : t("settings.extensions.connect")}</button>
            </footer>
          </section>
        </div>
      )}
      {pendingToolSelection && (
        <div className="extension-permission-backdrop" role="presentation" onMouseDown={() => setPendingToolSelection(null)}>
          <section ref={toolSelectionDialogRef} className="extension-permission-dialog" role="dialog" aria-modal="true" aria-labelledby="extension-tool-selection-title" aria-describedby="extension-tool-selection-hint" tabIndex={-1} onMouseDown={stopRowClick}>
            <header>
              <Link2 size={18} strokeWidth={2} aria-hidden="true" />
              <div>
                <h3 id="extension-tool-selection-title">{t("settings.extensions.chooseSystemTool", { name: pendingToolSelection.extension.name })}</h3>
                <p id="extension-tool-selection-hint">{t("settings.extensions.chooseSystemToolHint")}</p>
              </div>
            </header>
            <div className="extension-tool-choice-list" role="listbox" aria-label={t("settings.extensions.chooseSystemTool", { name: pendingToolSelection.extension.name })}>
              {pendingToolSelection.extension.toolCandidates.map((candidate, index) => (
                <button key={candidate.id} type="button" role="option" aria-selected="false" data-dialog-initial={index === 0 || undefined} disabled={Boolean(busy)} onClick={() => chooseSystemTool(candidate)}>
                  <strong>{candidate.name}</strong>
                  <span>{candidate.locator.path}</span>
                  <small>{candidate.sources.join(" · ")}</small>
                </button>
              ))}
            </div>
            <footer>
              <button type="button" className="extensions-action-button" onClick={() => setPendingToolSelection(null)}>{t("settings.extensions.cancel")}</button>
            </footer>
          </section>
        </div>
      )}

      <CustomIntegrationDrawer open={showCustomIntegration} editingId={editingCustomId} loading={customIntegrationLoading} error={customIntegrationError} integration={customIntegration} busy={Boolean(busy)} contentOperation={customContentOperation} discardArmed={customDiscardArmed} onDismissDiscard={() => setCustomDiscardArmed(false)} onDiscard={discardCustomIntegration} toolSuggestions={toolSuggestions} toolSearching={toolSearching} toolSearchFailed={toolSearchFailed} toolHighlight={toolHighlight} toolResultsRef={toolResultsRef} dialogRef={customDialogRef} t={t} onClose={closeCustomIntegration} onSubmit={(event) => void createCustomIntegration(event)} onUpdate={updateCustomIntegration} onToolKeyDown={handleToolSearchKeyDown} onToolHighlight={setToolHighlight} onChooseTool={chooseToolSuggestion} onCopy={copyCustomContent} onCopyPlan={() => void copyExecutionPlan()} onExportScript={() => void exportCustomScript()} scriptTemplate={scriptTemplate} />

      {selected && (
        <div className="extension-drawer-backdrop" role="presentation" style={showCustomIntegration || pendingLocal || pendingToolSelection || pendingPermissionReview ? { display: "none" } : undefined} onMouseDown={closeDetails}>
          <aside
            ref={drawerRef}
            className="extension-drawer"
            role="dialog"
            aria-modal="true"
            aria-labelledby="extension-drawer-title"
            tabIndex={-1}
            onMouseDown={stopRowClick}
          >
            <header className="extension-drawer__header">
              <div>
                <h3 id="extension-drawer-title">{selected.name}</h3>
                <span>v{selected.currentVersion}</span>
                <span className={`extension-status extension-status--${selected.state}`}>{t(`settings.extensions.status.${selected.state}`)}</span>
              </div>
              <button type="button" className="extensions-icon-button" aria-label={t("settings.extensions.closeDetails")} data-dialog-initial onClick={closeDetails}>
                <X size={17} strokeWidth={2} aria-hidden="true" />
              </button>
            </header>
            <div className="extension-drawer__body">
              {detailLoading && <div className="extension-drawer__loading"><LoaderCircle className="extensions-spinner" size={17} strokeWidth={2} />{t("settings.extensions.loadingDetails")}</div>}
              {detailError && <div className="extensions-notice extensions-notice--error"><AlertCircle size={15} strokeWidth={2} /><span>{detailError}</span></div>}
              {selected.state === "broken" && (selected.brokenReason || selected.lastErrorCode) && (
                <div className="extensions-notice extensions-notice--error"><AlertCircle size={15} strokeWidth={2} /><span>{t("settings.extensions.brokenDetail")}: {selected.lastErrorCode ? <code>{selected.lastErrorCode}</code> : null}{selected.brokenReason ? ` ${selected.brokenReason}` : ""}</span></div>
              )}
              <section className="extension-detail-block">
                <h4>{t("settings.extensions.info")}</h4>
                <dl className="extension-metadata">
                  <div><dt>{t("settings.extensions.integrationKind")}</dt><dd title={t(integrationKindKey(selected))}>{t(integrationKindKey(selected))}</dd></div>
                  <div><dt>{t("settings.extensions.author")}</dt><dd title={selected.publisherName}>{selected.publisherName}</dd></div>
                  <div><dt>{t("settings.extensions.source")}</dt><dd title={t(`settings.extensions.runtimeSource.${selected.runtimeSource}`)}>{t(`settings.extensions.runtimeSource.${selected.runtimeSource}`)}</dd></div>
                  <div><dt>{t("settings.extensions.integrationVersion")}</dt><dd title={selected.packageVersion}>{selected.packageVersion}</dd></div>
                  <div><dt>{t("settings.extensions.toolVersion")}</dt><dd title={selected.toolVersion ?? t("settings.extensions.unavailable")}>{selected.toolVersion ?? t("settings.extensions.unavailable")}</dd></div>
                  <div><dt>{t("settings.extensions.availability")}</dt><dd title={t(selected.runtimeAvailable ? "settings.extensions.runtimeAvailable" : "settings.extensions.runtimeUnavailable")}>{t(selected.runtimeAvailable ? "settings.extensions.runtimeAvailable" : "settings.extensions.runtimeUnavailable")}</dd></div>
                  <div><dt>{t("settings.extensions.status")}</dt><dd title={t(`settings.extensions.status.${selected.state}`)}>{t(`settings.extensions.status.${selected.state}`)}</dd></div>
                  {selected.state === "broken" && (selected.lastErrorCode || selected.brokenReason) && (
                    <div><dt>{t("settings.extensions.brokenDetail")}</dt><dd className="extension-metadata__dd--wrap" title={[selected.lastErrorCode, selected.brokenReason].filter(Boolean).join(" · ")}>{selected.lastErrorCode ? <code>{selected.lastErrorCode}</code> : null}{selected.brokenReason ? ` · ${selected.brokenReason}` : ""}</dd></div>
                  )}
                  {selected.approvedAt ? (
                    <div><dt>{t("settings.extensions.approvedAt")}</dt><dd title={new Date(selected.approvedAt * 1000).toLocaleString(locale)}>{new Date(selected.approvedAt * 1000).toLocaleString(locale)}</dd></div>
                  ) : null}
                  <div><dt>{t("settings.extensions.signature")}</dt><dd title={t(selected.signatureVerified ? "settings.extensions.signatureVerified" : "settings.extensions.signatureMissing")}>{t(selected.signatureVerified ? "settings.extensions.signatureVerified" : "settings.extensions.signatureMissing")}</dd></div>
                  <div><dt>{t("settings.extensions.trust")}</dt><dd title={t(selected.officialVerified ? "settings.extensions.trustOfficial" : "settings.extensions.trustCommunity")}>{t(selected.officialVerified ? "settings.extensions.trustOfficial" : "settings.extensions.trustCommunity")}</dd></div>
                  <div><dt>{t("settings.extensions.homepage")}</dt><dd className="extension-metadata__dd--wrap" title={selected.homepage ?? t("settings.extensions.unavailable")}>{selected.homepage ?? t("settings.extensions.unavailable")}</dd></div>
                </dl>
                <p className="extension-detail-description">{provider?.description.provider.description || t("settings.extensions.noDescription")}</p>
              </section>

              <section className="extension-detail-block">
                <h4>{t("settings.extensions.commands")}</h4>
                {provider?.description.commands.length ? (
                  <div className="extension-command-list">
                    {provider.description.commands.map((command) => (
                      <div key={command.id}><code>{command.name}</code><span>{command.description || t("settings.extensions.noDescription")}</span></div>
                    ))}
                  </div>
                ) : !detailLoading && <p className="extension-detail-empty">{t("settings.extensions.noCommands")}</p>}
                {selected.generatedCustom && (
                  <button
                    type="button"
                    className="extensions-action-button"
                    disabled={Boolean(busy) || reprobingCommands}
                    aria-busy={reprobingCommands}
                    onClick={() => void handleReprobeCommands()}
                  >
                    {reprobingCommands
                      ? <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" />
                      : <RefreshCw size={14} strokeWidth={2} aria-hidden="true" />}
                    {reprobingCommands ? t("settings.extensions.reprobing") : t("settings.extensions.reprobeCommands")}
                  </button>
                )}
              </section>

              <section className="extension-detail-block">
                <h4>{t("settings.extensions.diagnostics")}</h4>
                {diagnose ? (
                  <div className="extension-diagnostics">
                    <span className={`extension-diagnostics__summary extension-diagnostics__summary--${diagnose.status}`}>{diagnose.status}</span>
                    {diagnose.checks.map((check) => (
                      <div key={check.id}><span className={`extension-check extension-check--${check.status}`}>{check.status}</span><p>{check.message}</p></div>
                    ))}
                  </div>
                ) : !detailLoading && <p className="extension-detail-empty">{t("settings.extensions.diagnosticsUnavailable")}</p>}
              </section>

              <section className="extension-detail-block">
                <h4 className="extension-detail-heading">
                  {t("settings.extensions.health")}
                  {healthReport && (
                    <button
                      type="button"
                      className="extensions-icon-button"
                      title={t("settings.extensions.healthReprobe")}
                      aria-label={t("settings.extensions.healthReprobe")}
                      disabled={healthLoading}
                      onClick={() => void handleReprobe()}
                    >
                      <RefreshCw size={14} strokeWidth={2} aria-hidden="true" />
                    </button>
                  )}
                </h4>
                {healthReport ? (
                  <div className="extension-health">
                    <div className="extension-health__header">
                      <span className={`extension-health__status extension-health__status--${healthReport.status}`}>{t(`settings.extensions.healthStatus.${healthReport.status}`)}</span>
                      {healthReport.capabilities.version && <span className="extension-health__version">v{healthReport.capabilities.version}</span>}
                    </div>
                    {healthReport.capabilities.supportedFeatures.length > 0 && (
                      <div className="extension-health__features">
                        <span className="extension-health__label">{t("settings.extensions.healthFeatures")}</span>
                        {healthReport.capabilities.supportedFeatures.map((f) => (
                          <span key={f} className="extension-health__tag">{f}</span>
                        ))}
                      </div>
                    )}
                    {healthReport.capabilities.limitations.length > 0 && (
                      <div className="extension-health__limitations">
                        <span className="extension-health__label">{t("settings.extensions.healthLimitations")}</span>
                        <ul>
                          {healthReport.capabilities.limitations.map((l, i) => (
                            <li key={i}>{l}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                    <div className="extension-health__probes">
                      <span className="extension-health__label">{t("settings.extensions.healthProbes")}</span>
                      {healthReport.probes.map((probe) => (
                        <div key={probe.probeId} className="extension-health__probe">
                          <span className={`extension-health__probe-status extension-health__probe-status--${probe.passed ? "passed" : "failed"}`} aria-hidden="true">
                            {probe.passed ? <Check size={11} strokeWidth={2.4} /> : <X size={11} strokeWidth={2.4} />}
                          </span>
                          <span className="extension-health__probe-id">{probe.probeId}</span>
                          <span className="extension-health__probe-duration">{probe.durationMs}ms</span>
                        </div>
                      ))}
                    </div>
                    {healthReport.failures.length > 0 && (
                      <div className="extension-health__failures">
                        <span className="extension-health__label">{t("settings.extensions.healthFailures")}</span>
                        {healthReport.failures.map((failure, i) => (
                          <div key={i} className="extension-health__failure">
                            <strong>{failure.probe}</strong>
                            <span className="extension-health__failure-code">({t("settings.extensions.healthExitCode", { code: failure.exitCode ?? "-" })})</span>
                            {failure.stderr && <p className="extension-health__failure-stderr">{failure.stderr}</p>}
                            {failure.retryable && <span className="extension-health__retryable">{t("settings.extensions.healthRetryable")}</span>}
                          </div>
                        ))}
                      </div>
                    )}
                    <p className="extension-health__timestamp">{t("settings.extensions.healthCheckedAt", { time: new Date(healthReport.checkedAt).toLocaleString(locale) })}</p>
                  </div>
                ) : !detailLoading && !healthLoading && <p className="extension-detail-empty">{t("settings.extensions.healthUnavailable")}</p>}
              </section>

              {configuration && (
                <section className="extension-detail-block">
                  <h4>{t("settings.extensions.configuration")}</h4>
                  {configuration.descriptor.owner === "tool" ? (
                    <button
                      type="button"
                      className="extensions-action-button"
                      disabled={!configuration.openPlan}
                      onClick={() => configuration.openPlan && void onOpenCommand(configuration.openPlan, `${selected.name} ${t("settings.extensions.configuration")}`)}
                    >
                      <ExternalLink size={14} strokeWidth={2} aria-hidden="true" />
                      {t("settings.extensions.openConfiguration")}
                    </button>
                  ) : (
                    <form className="extension-config-form" onSubmit={(event) => { event.preventDefault(); void saveConfiguration(); }}>
                      <div className="extension-config-toolbar">
                        <span className={configDirty ? "extension-config-status extension-config-status--dirty" : "extension-config-status"}>
                          {t(configDirty ? "settings.extensions.configUnsaved" : "settings.extensions.configSavedState")}
                        </span>
                        <div className="extension-config-toolbar__actions">
                          <button type="button" className="extensions-icon-button" title={t("settings.extensions.copyConfiguration")} aria-label={t("settings.extensions.copyConfiguration")} disabled={Boolean(configOperation)} onClick={() => void copyConfiguration()}>
                            <Copy size={14} strokeWidth={2} aria-hidden="true" />
                          </button>
                          <button type="button" className="extensions-icon-button" title={t("settings.extensions.exportConfiguration")} aria-label={t("settings.extensions.exportConfiguration")} disabled={Boolean(configOperation)} onClick={() => void exportConfiguration()}>
                            <FileDown size={14} strokeWidth={2} aria-hidden="true" />
                          </button>
                        </div>
                      </div>
                      {configNotice && <div className="extensions-notice extensions-notice--success" role="status"><Check size={14} strokeWidth={2} aria-hidden="true" /><span>{configNotice}</span></div>}
                      {configuration.descriptor.schema.map((field) => (
                        <ConfigFieldControl
                          key={field.key}
                          disabled={Boolean(busy)}
                          field={field}
                          value={configValues[field.key] ?? field.default}
                          t={t}
                          onChange={(value) => {
                            setConfigNotice(null);
                            setDetailsDiscardArmed(false);
                            setConfigValues((current) => ({ ...current, [field.key]: value }));
                          }}
                        />
                      ))}
                      <div className="extension-config-form__actions">
                        <button type="button" className="extensions-action-button" disabled={Boolean(busy) || Boolean(configOperation)} onClick={() => { setConfigNotice(null); setConfigValues(configurationDefaults()); }}>
                          <RotateCcw size={14} strokeWidth={2} aria-hidden="true" />
                          {t("settings.extensions.configDefaults")}
                        </button>
                        <button type="submit" className="extensions-action-button extensions-action-button--primary" disabled={Boolean(busy) || !configDirty}>
                          {busy?.kind === "save" ? t("settings.extensions.saving") : t("settings.extensions.save")}
                        </button>
                      </div>
                    </form>
                  )}
                </section>
              )}
            </div>
            {detailsDiscardArmed && configDirty && (
              <div className="extensions-notice extensions-notice--error extension-discard-bar" role="alert">
                <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
                <span>{t("settings.extensions.configDiscardConfirm")}</span>
                <button type="button" className="extensions-action-button" onClick={() => setDetailsDiscardArmed(false)}>{t("settings.extensions.cancel")}</button>
                <button type="button" data-destructive-confirm className="extensions-action-button extensions-action-button--danger" onClick={discardDetailsChanges}>{t("settings.extensions.discard")}</button>
              </div>
            )}
            {removalConfirmation}
            <footer className="extension-drawer__footer">
              {selected.generatedCustom && <button type="button" className="extensions-action-button" disabled={Boolean(busy)} onClick={() => void runMutation(selected.id, "repair", () => invoke("open_path", { path: selected.manifestPath.replace(/[\\/]floter\.extension\.json$/, "") }))}>
                <ExternalLink size={14} strokeWidth={2} />{t("settings.extensions.openGeneratedLocation")}
              </button>}
              {selected.generatedCustom && <button type="button" className="extensions-action-button" disabled={Boolean(busy)} onClick={() => void editCustomIntegration(selected)}>
                {t("settings.extensions.editCustom")}
              </button>}
              {selected.distributionSource === "local" && (
                <button
                  type="button"
                  className="extensions-action-button extensions-action-button--primary"
                  aria-busy={busy?.id === selected.id && busy.kind === "repair"}
                  disabled={Boolean(busy)}
                  onClick={() => void repairExtension(selected)}
                >
                  {busy?.id === selected.id && busy.kind === "repair"
                    ? <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" />
                    : <Wrench size={14} strokeWidth={2} aria-hidden="true" />}
                  {t("settings.extensions.recheck")}
                </button>
              )}
              <button type="button" className="extensions-action-button extensions-action-button--danger" disabled={Boolean(busy)} onClick={() => uninstallExtension(selected)}>
                {removalKind(selected) === "system" ? <Unplug size={14} strokeWidth={2} /> : <Trash2 size={14} strokeWidth={2} />}
                {busy?.id === selected.id && busy.kind === "uninstall" ? t("settings.extensions.removing") : t(removalTextKey(selected, ""))}
              </button>
            </footer>
          </aside>
        </div>
      )}

      {toasts.length > 0 && (
        <div className={`extensions-toasts${selected || showCustomIntegration ? " extensions-toasts--offset" : ""}`}>
          {toasts.map((toast) => <ExtensionsToast key={toast.id} toast={toast} t={t} onDismiss={dismissToast} />)}
        </div>
      )}
    </section>
  );
}

function EmptyState({ icon, text, query }: { icon: React.ReactNode; text: string; query?: string }) {
  return <div className="extensions-empty">{icon}<span>{text}{query && <strong> "{query}"</strong>}</span></div>;
}

type ConfigFieldControlProps = {
  disabled: boolean;
  field: ConfigField;
  value: JsonValue | undefined;
  t: Translate;
  onChange: (value: JsonValue) => void;
};

function ConfigFieldControl({ disabled, field, value, t, onChange }: ConfigFieldControlProps) {
  const id = `extension-config-${field.key}`;
  const label = field.label || field.key;
  let control: React.ReactNode;
  if (field.type === "boolean") {
    control = (
      <button
        id={id}
        disabled={disabled}
        type="button"
        role="switch"
        aria-checked={value === true}
        className={`settings-switch${value === true ? " settings-switch--active" : ""}`}
        onClick={() => onChange(value !== true)}
      ><span className="settings-switch__thumb" /></button>
    );
  } else if (field.type === "select") {
    control = (
      <select
        id={id}
        disabled={disabled}
        required={field.required}
        value={displayJson(value ?? "")}
        onChange={(event) => {
          const option = field.options.find((candidate) => displayJson(candidate) === event.target.value);
          onChange(option ?? (event.target.value === "" ? null : event.target.value));
        }}
      >
        {!field.required && <option value="">{t("settings.extensions.configNone")}</option>}
        {field.options.map((option) => <option key={displayJson(option)} value={displayJson(option)}>{displayJson(option)}</option>)}
      </select>
    );
  } else if (field.type === "multi-select") {
    const selectedValues = Array.isArray(value) ? value.map(displayJson) : [];
    control = (
      <select
        id={id}
        disabled={disabled}
        multiple
        required={field.required}
        value={selectedValues}
        onChange={(event) => onChange(Array.from(event.target.selectedOptions, (option) =>
          field.options.find((candidate) => displayJson(candidate) === option.value) ?? option.value,
        ))}
      >
        {field.options.map((option) => <option key={displayJson(option)} value={displayJson(option)}>{displayJson(option)}</option>)}
      </select>
    );
  } else {
    control = (
      <input
        id={id}
        disabled={disabled}
        type={field.type === "password" ? "password" : field.type === "number" ? "number" : "text"}
        required={field.required}
        min={field.minimum ?? undefined}
        max={field.maximum ?? undefined}
        minLength={field.type === "text" ? field.minLength ?? undefined : undefined}
        maxLength={field.type === "text" ? field.maxLength ?? undefined : undefined}
        value={displayJson(value ?? "")}
        onChange={(event) => onChange(field.type === "number" ? (event.target.value === "" ? null : event.target.valueAsNumber) : event.target.value)}
      />
    );
  }
  return (
    <label className="extension-config-field" htmlFor={id}>
      <span>{label}{field.required && <b aria-label={t("settings.extensions.required")}>*</b>}</span>
      {control}
      {field.description && <small>{field.description}</small>}
    </label>
  );
}
