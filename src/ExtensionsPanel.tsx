import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent as ReactKeyboardEvent, type MouseEvent, type RefObject } from "react";
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
import { RemovalDialog } from "./extensions/RemovalDialog";

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
    if (!active) return;
    const previouslyFocused = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const focusInitial = window.setTimeout(() => {
      const dialog = dialogRef.current;
      const initial = dialog?.querySelector<HTMLElement>("[data-dialog-initial]");
      (initial ?? dialog?.querySelector<HTMLElement>(FOCUSABLE_SELECTOR) ?? dialog)?.focus();
    }, 0);
    const handleKeyDown = (event: KeyboardEvent) => {
      const dialog = dialogRef.current;
      if (!dialog || !interactiveRef.current) return;
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
        .filter((element) => !element.hidden && element.getAttribute("aria-hidden") !== "true");
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
      window.setTimeout(() => previouslyFocused?.focus(), 0);
    };
  }, [active, dialogRef]);
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
  t: Translate;
  locale: "en" | "zh";
  onOpenCommand: (plan: ExtensionExecutionPlan, label: string) => void | Promise<void>;
  /** Whether extension commands currently appear in launcher search results. */
  showCommandsInSearch: boolean;
  /** Flip the launcher command-discovery setting and persist it. */
  onToggleCommandsInSearch: () => void;
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

export function ExtensionsPanel({ t, locale, onOpenCommand, showCommandsInSearch, onToggleCommandsInSearch }: ExtensionsPanelProps) {
  const [extensions, setExtensions] = useState<Extension[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<ExtensionOperation>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [provider, setProvider] = useState<ProviderResponse | null>(null);
  const [diagnose, setDiagnose] = useState<DiagnoseResult | null>(null);
  const [healthReport, setHealthReport] = useState<HealthReport | null>(null);
  const [healthLoading, setHealthLoading] = useState(false);
  const [reprobingCommands, setReprobingCommands] = useState(false);
  // Bumped to force the drawer's details effect (provider/diagnose/config)
  // to reload without a lock-entry change, e.g. after a command re-probe.
  const [detailReloadTick, setDetailReloadTick] = useState(0);
  const [configuration, setConfiguration] = useState<ExtensionConfiguration | null>(null);
  const [configValues, setConfigValues] = useState<Record<string, JsonValue>>({});
  const [savedConfigValues, setSavedConfigValues] = useState<Record<string, JsonValue>>({});
  const [configOperation, setConfigOperation] = useState<ConfigOperation>(null);
  const [configNotice, setConfigNotice] = useState<string | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [pendingLocal, setPendingLocal] = useState<{ review: PermissionReview; request: InstallRequest; name: string; runtime: string; platforms: string[]; source: string } | null>(null);
  const [pendingToolSelection, setPendingToolSelection] = useState<PendingToolSelection>(null);
  const [syncOperation, setSyncOperation] = useState<SyncOperation | null>(null);
  const [exportResult, setExportResult] = useState<ExtensionsExportResult | null>(null);
  const [importReport, setImportReport] = useState<ExtensionsImportReport | null>(null);
  const [showCustomIntegration, setShowCustomIntegration] = useState(false);
  const [editingCustomId, setEditingCustomId] = useState<string | null>(null);
  const [customIntegrationLoading, setCustomIntegrationLoading] = useState(false);
  const [customIntegrationError, setCustomIntegrationError] = useState<string | null>(null);
  const [customContentOperation, setCustomContentOperation] = useState<CustomContentOperation>(null);
  const [customIntegration, setCustomIntegration] = useState<CustomIntegrationForm>(DEFAULT_CUSTOM_INTEGRATION);
  const [toolResults, setToolResults] = useState<ExecutableToolCandidate[]>([]);
  const [toolSearching, setToolSearching] = useState(false);
  const [toolSearchFailed, setToolSearchFailed] = useState(false);
  const [toolHighlight, setToolHighlight] = useState(0);
  const [customDirty, setCustomDirty] = useState(false);
  const customSavedRef = useRef<CustomIntegrationForm>(DEFAULT_CUSTOM_INTEGRATION);
  const toolSearchNeedsRefresh = useRef(true);
  const suppressToolSearch = useRef(false);
  const [removalTarget, setRemovalTarget] = useState<RemovalTarget>(null);
  const [toasts, setToasts] = useState<PanelToast[]>([]);
  const toastIdRef = useRef(0);
  const detailGeneration = useRef(0);
  const customCreateButtonRef = useRef<HTMLButtonElement | null>(null);
  const localDialogRef = useRef<HTMLElement | null>(null);
  const toolSelectionDialogRef = useRef<HTMLElement | null>(null);
  const removalDialogRef = useRef<HTMLElement | null>(null);
  const customDialogRef = useRef<HTMLElement | null>(null);
  const toolResultsRef = useRef<HTMLDivElement | null>(null);
  const drawerRef = useRef<HTMLElement | null>(null);
  const refreshGeneration = useRef(0);
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
  const extensionActions = useExtensionActions({ refresh: () => refreshRef.current(), onError: (nextError) => showError(errorMessage(nextError)), onComplete: () => showSuccess(t("settings.extensions.operationComplete")) });
  useEffect(() => {
    setBusy(extensionActions.busy as ExtensionOperation);
  }, [extensionActions.busy]);

  const updateCustomIntegration = (update: (current: CustomIntegrationForm) => CustomIntegrationForm) => {
    setCustomIntegrationError(null);
    setCustomIntegration((current) => {
      const next = update(current);
      setCustomDirty(JSON.stringify(next) !== JSON.stringify(customSavedRef.current));
      return next;
    });
  };

  const resetCustomIntegration = () => {
    setEditingCustomId(null);
    const fresh = { ...DEFAULT_CUSTOM_INTEGRATION, argsPrefix: [], versionArgs: [], permissions: [...DEFAULT_CUSTOM_INTEGRATION.permissions], platforms: [CURRENT_PLATFORM] as Array<"darwin" | "linux" | "windows"> };
    setCustomIntegration(fresh);
    setCustomIntegrationError(null);
    setToolResults([]);
    setToolSearchFailed(false);
    setToolHighlight(0);
    toolSearchNeedsRefresh.current = true;
    setCustomDirty(false);
    customSavedRef.current = fresh;
  };

  const closeCustomIntegration = () => {
    if (busy || customIntegrationLoading) return;
    if (customDirty && !window.confirm(t("settings.extensions.customDiscardConfirm"))) return;
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

  const refresh = async () => {
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
    if (!selectedId || healthLoading) return;
    const generation = detailGeneration.current;
    setHealthLoading(true);
    setDetailError(null);
    try {
      const report = await invoke<HealthReport>("extensions_reprobe", { id: selectedId });
      if (generation === detailGeneration.current) setHealthReport(report);
    } catch (nextError) {
      if (generation === detailGeneration.current) setDetailError(errorMessage(nextError));
    } finally {
      if (generation === detailGeneration.current) setHealthLoading(false);
    }
  };

  const handleReprobeCommands = async () => {
    if (!selected || reprobingCommands || busy) return;
    setReprobingCommands(true);
    try {
      const report = await invoke<{ rootArguments: number; subcommands: number }>(
        "extensions_reprobe_commands",
        { id: selected.id },
      );
      await refresh();
      setDetailReloadTick((tick) => tick + 1);
      showSuccess(t("settings.extensions.reprobedCommands", { subcommands: report.subcommands, arguments: report.rootArguments }));
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setReprobingCommands(false);
    }
  };

  useDialogFocus(Boolean(pendingLocal), localDialogRef, () => {
    if (!busy) setPendingLocal(null);
  });
  useDialogFocus(Boolean(pendingToolSelection), toolSelectionDialogRef, () => {
    if (!busy) setPendingToolSelection(null);
  });
  useDialogFocus(Boolean(removalTarget), removalDialogRef, () => {
    if (!busy) setRemovalTarget(null);
  });
  useDialogFocus(showCustomIntegration, customDialogRef, closeCustomIntegration);
  useDialogFocus(Boolean(selectedId), drawerRef, () => {
    if (!configDirty || window.confirm(t("settings.extensions.configDiscardConfirm"))) {
      setSelectedId(null);
    }
  }, !showCustomIntegration && !removalTarget && !pendingLocal && !pendingToolSelection);

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
    if (syncOperation) return;
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
    if (syncOperation) return;
    setSyncOperation("import");
    setExportResult(null);
    setImportReport(null);
    try {
      const report = await invoke<ExtensionsImportReport | null>("extensions_import", { locale });
      if (report) {
        setImportReport(report);
        await refresh();
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
    if (extensionActions.busy) return;
    extensionActions.setBusy({ id: extension.id, kind: "repair" });
    try {
      const report = await invoke<{ repaired: boolean }>("extensions_repair", { id: extension.id });
      await refreshRef.current();
      showSuccess(t(report.repaired ? "settings.extensions.repairedNotice" : "settings.extensions.recheckedNotice"));
    } catch (error) {
      showError(errorMessage(error));
    } finally {
      extensionActions.setBusy(null);
    }
  };

  const uninstallExtension = (extension: Extension) => setRemovalTarget(extension);

  const confirmRemoval = async () => {
    if (!removalTarget || busy) return;
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
    if (busy || !extension.runtimeAvailable) return;
    setBusy({ id: extension.id, kind: "install" });
    try {
      const review = await invoke<PermissionReview>("extensions_recommended_permissions", { id: extension.id, locale });
      const confirmKey = extension.manifestSuggestion ? "settings.extensions.confirmConnectManifest" : "settings.extensions.confirmConnectRecommended";
      if (review.permissions.length && !window.confirm(t(confirmKey, {
        name: extension.name,
        path: executablePath ?? extension.executablePath,
        permissions: review.permissions.map((permission) => permission.title).join(", "),
      }))) return;
      await invoke("extensions_connect_recommended", {
        id: extension.id,
        executablePath,
        approvedPermissions: review.permissions.map(({ permission }) => permission),
      });
      await refresh();
      showSuccess(t("settings.extensions.connectedNotice", { name: extension.name }));
      setPendingToolSelection(null);
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  const connectRecommended = (extension: Extension) => {
    if (extension.toolCandidates.length > 1) {
      setPendingToolSelection({ extension, action: "connect" });
      return;
    }
    void connectRecommendedAt(extension, extension.toolCandidates[0]?.locator.path ?? null);
  };

  const connectLocal = async () => {
    if (busy) return;
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
    if (!pendingLocal || busy) return;
    const pending = pendingLocal;
    setBusy({ id: "local", kind: "install" });
    try {
      await invoke("extensions_install", { request: { ...pending.request, approvedPermissions: pending.review.permissions.map(({ permission }) => permission) } });
      await refresh();
      setPendingLocal(null);
      showSuccess(t("settings.extensions.connectedNotice", { name: pending.name }));
    } catch (nextError) { showError(errorMessage(nextError)); }
    finally { setBusy(null); }
  };

  const openCreateCustomIntegration = () => {
    resetCustomIntegration();
    setShowCustomIntegration(true);
  };

  const editCustomIntegration = async (extension: Extension) => {
    if (!extension.generatedCustom || busy || customIntegrationLoading) return;
    setCustomIntegrationLoading(true);
    setEditingCustomId(extension.id);
    setCustomIntegrationError(null);
    setShowCustomIntegration(true);
    try {
      const definition = await invoke<CustomIntegrationForm>("extensions_custom_get", { id: extension.id });
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
      setCustomIntegrationError(errorMessage(nextError));
    } finally {
      setCustomIntegrationLoading(false);
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
    if (customIntegration.executablePath.trim()) return candidates;
    return [
      ...suggestedExtensions.map((extension): ToolSuggestion => ({ kind: "recommendation", extension })),
      ...candidates,
    ];
  }, [toolResults, suggestedExtensions, customIntegration.executablePath]);

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
    if (busy) return;
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
      customSavedRef.current = customIntegration;
      setCustomDirty(false);
      const notice = editingCustomId
        ? t("settings.extensions.customUpdated", { name: customIntegration.name })
        : t("settings.extensions.customCreated", { name: customIntegration.name });
      setShowCustomIntegration(false);
      resetCustomIntegration();
      showSuccess(notice);
      await refresh();
    } catch (nextError) {
      setCustomIntegrationError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  const copyCustomContent = async (content: string, notice: string) => {
    if (customContentOperation) return;
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
    if (customContentOperation) return;
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
    if (busy || !selected || !configuration || configuration.descriptor.owner !== "host") return;
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
    if (!selected || !configuration || configuration.descriptor.owner !== "host" || configOperation) return;
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
    if (!selected || !configuration || configuration.descriptor.owner !== "host" || configOperation) return;
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
    if (configDirty && !window.confirm(t("settings.extensions.configDiscardConfirm"))) return;
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
          aria-label={t("settings.extensions.showInSearch")}
          onMouseDown={(event) => event.preventDefault()}
          onClick={onToggleCommandsInSearch}
        >
          <span className="settings-switch__thumb" />
        </button>
      </div>

      <div className="extensions-installed">
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

        <section className="extensions-section">
          <h3 className="extensions-section-title">{t("settings.extensions.section.connected")}</h3>
          <div className="extensions-list extensions-list--installed">
            {loading ? (
              <EmptyState icon={<LoaderCircle className="extensions-spinner" size={20} strokeWidth={2} />} text={t("settings.extensions.loading")} />
            ) : connectedExtensions.length === 0 ? (
              <EmptyState icon={<Package size={20} strokeWidth={2} />} text={t("settings.extensions.emptyInstalled")} />
            ) : connectedExtensions.map((extension) => (
              <ExtensionRowComponent
                key={extension.id}
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
            ))}
          </div>
        </section>
      </div>
      {pendingLocal && <LocalInstallDialog pending={pendingLocal} busy={Boolean(busy)} t={t} dialogRef={localDialogRef} stopPropagation={stopRowClick} onCancel={() => setPendingLocal(null)} onConfirm={() => void confirmLocal()} />}
      {pendingToolSelection && (
        <div className="extension-permission-backdrop" role="presentation" onMouseDown={() => { if (!busy) setPendingToolSelection(null); }}>
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
              <button type="button" className="extensions-action-button" disabled={Boolean(busy)} onClick={() => setPendingToolSelection(null)}>{t("settings.extensions.cancel")}</button>
            </footer>
          </section>
        </div>
      )}

      {removalTarget && <RemovalDialog extension={removalTarget} busy={Boolean(busy)} t={t} dialogRef={removalDialogRef} stopPropagation={stopRowClick} textKey={removalTextKey} onCancel={() => setRemovalTarget(null)} onConfirm={() => void confirmRemoval()} />}
      <CustomIntegrationDrawer open={showCustomIntegration} editingId={editingCustomId} loading={customIntegrationLoading} error={customIntegrationError} integration={customIntegration} busy={Boolean(busy)} contentOperation={customContentOperation} toolSuggestions={toolSuggestions} toolSearching={toolSearching} toolSearchFailed={toolSearchFailed} toolHighlight={toolHighlight} toolResultsRef={toolResultsRef} dialogRef={customDialogRef} t={t} onClose={closeCustomIntegration} onSubmit={(event) => void createCustomIntegration(event)} onUpdate={updateCustomIntegration} onToolKeyDown={handleToolSearchKeyDown} onToolHighlight={setToolHighlight} onChooseTool={chooseToolSuggestion} onCopy={copyCustomContent} onCopyPlan={() => void copyExecutionPlan()} onExportScript={() => void exportCustomScript()} scriptTemplate={scriptTemplate} />

      {selected && (
        <div className="extension-drawer-backdrop" role="presentation" onMouseDown={closeDetails}>
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
              </div>
              <button type="button" className="extensions-icon-button" aria-label={t("settings.extensions.closeDetails")} data-dialog-initial onClick={closeDetails}>
                <X size={17} strokeWidth={2} aria-hidden="true" />
              </button>
            </header>
            <div className="extension-drawer__body">
              {detailLoading && <div className="extension-drawer__loading"><LoaderCircle className="extensions-spinner" size={17} strokeWidth={2} />{t("settings.extensions.loadingDetails")}</div>}
              {detailError && <div className="extensions-notice extensions-notice--error"><AlertCircle size={15} strokeWidth={2} /><span>{detailError}</span></div>}
              <section className="extension-detail-block">
                <h4>{t("settings.extensions.info")}</h4>
                <dl className="extension-metadata">
                  <div><dt>{t("settings.extensions.integrationKind")}</dt><dd>{t(integrationKindKey(selected))}</dd></div>
                  <div><dt>{t("settings.extensions.author")}</dt><dd>{selected.publisherName}</dd></div>
                  <div><dt>{t("settings.extensions.source")}</dt><dd>{t(`settings.extensions.runtimeSource.${selected.runtimeSource}`)}</dd></div>
                  <div><dt>{t("settings.extensions.integrationVersion")}</dt><dd>{selected.packageVersion}</dd></div>
                  <div><dt>{t("settings.extensions.toolVersion")}</dt><dd>{selected.toolVersion ?? t("settings.extensions.unavailable")}</dd></div>
                  <div><dt>{t("settings.extensions.availability")}</dt><dd>{t(selected.runtimeAvailable ? "settings.extensions.runtimeAvailable" : "settings.extensions.runtimeUnavailable")}</dd></div>
                  <div><dt>{t("settings.extensions.status")}</dt><dd>{t(`settings.extensions.status.${selected.state}`)}</dd></div>
                  {selected.state === "broken" && (selected.lastErrorCode || selected.brokenReason) && (
                    <div><dt>{t("settings.extensions.brokenDetail")}</dt><dd>{selected.lastErrorCode ? <code>{selected.lastErrorCode}</code> : null}{selected.brokenReason ? ` · ${selected.brokenReason}` : ""}</dd></div>
                  )}
                  {selected.approvedAt ? (
                    <div><dt>{t("settings.extensions.approvedAt")}</dt><dd>{new Date(selected.approvedAt * 1000).toLocaleString(locale)}</dd></div>
                  ) : null}
                  <div><dt>{t("settings.extensions.signature")}</dt><dd>{t(selected.signatureVerified ? "settings.extensions.signatureVerified" : "settings.extensions.signatureMissing")}</dd></div>
                  <div><dt>{t("settings.extensions.trust")}</dt><dd>{t(selected.officialVerified ? "settings.extensions.trustOfficial" : "settings.extensions.trustCommunity")}</dd></div>
                  <div><dt>{t("settings.extensions.homepage")}</dt><dd>{selected.homepage ?? t("settings.extensions.unavailable")}</dd></div>
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
                          field={field}
                          value={configValues[field.key] ?? field.default}
                          t={t}
                          onChange={(value) => {
                            setConfigNotice(null);
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
            <footer className="extension-drawer__footer">
              {selected.generatedCustom && <button type="button" className="extensions-action-button" disabled={Boolean(busy)} onClick={() => void invoke("open_path", { path: selected.manifestPath.replace(/[\\/]floter\.extension\.json$/, "") })}>
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
        <div className="extensions-toasts">
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
  field: ConfigField;
  value: JsonValue | undefined;
  t: Translate;
  onChange: (value: JsonValue) => void;
};

function ConfigFieldControl({ field, value, t, onChange }: ConfigFieldControlProps) {
  const id = `extension-config-${field.key}`;
  const label = field.label || field.key;
  let control: React.ReactNode;
  if (field.type === "boolean") {
    control = (
      <button
        id={id}
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
