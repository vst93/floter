import { useEffect, useMemo, useRef, useState, type FormEvent, type MouseEvent, type RefObject } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  AlertCircle,
  Check,
  ChevronRight,
  Copy,
  Download,
  ExternalLink,
  FileDown,
  FileUp,
  Link2,
  MoreHorizontal,
  Package,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  ShieldCheck,
  Trash2,
  Unplug,
  X,
} from "lucide-react";
import type { Translate } from "./i18n";

type ExtensionDistributionSource = "npm" | "local" | "built-in";
type ExtensionRuntimeOwnership = "bundled" | "system";
type ExtensionStateKind =
  | "enabled"
  | "disabled"
  | "broken";

type Extension = {
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
};

type SearchResult = {
  package: string;
  version: string;
  description: string;
  publisher: string | null;
  homepage: string | null;
  verified: boolean;
  downloads: number;
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

type Tab = "installed" | "discover" | "updates";
type MutationKind = "enable" | "disable" | "install" | "update" | "rollback" | "reinstall" | "uninstall" | "save";
type SyncOperation = "export" | "import";
type ConfigOperation = "copy" | "export" | null;
type CustomContentOperation = "copy" | "export" | null;
type RemovalTarget = Extension | null;

type PathExecutable = {
  name: string;
  path: string;
};

type CustomIntegrationForm = {
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

const CUSTOM_PLATFORMS = ["darwin", "linux", "windows"] as const;

const CUSTOM_ENFORCED_PERMISSION_NAMES: PermissionName[] = ["environment", "process-spawn"];

const CUSTOM_DECLARED_PERMISSION_NAMES: PermissionName[] = [
  "filesystem-read",
  "filesystem-write",
  "network-fetch",
  "clipboard-read",
  "clipboard-write",
];

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

const integrationKindKey = (extension: Extension) => {
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
};

type PermissionName =
  | "filesystem-read"
  | "filesystem-write"
  | "network-fetch"
  | "process-spawn"
  | "clipboard-read"
  | "clipboard-write"
  | "environment";

type PermissionReview = {
  extensionId: string;
  extensionName: string;
  permissions: Array<{ permission: PermissionName; title: string; description: string }>;
  publisherSigned: boolean;
  officialVerified: boolean;
};

type InstallRequest = {
  source: "npm" | "linked";
  package: string | null;
  version: string | null;
  manifestPath: string | null;
  executablePath: string | null;
  approvedPermissions?: PermissionName[];
};

const compareVersions = (left: string, right: string): number => {
  const parse = (value: string) => {
    const [core, prerelease = ""] = value.replace(/^v/, "").split("-", 2);
    const numbers = core.split(".").map((part) => Number.parseInt(part, 10) || 0);
    return { numbers, prerelease };
  };
  const a = parse(left);
  const b = parse(right);
  for (let index = 0; index < Math.max(a.numbers.length, b.numbers.length); index += 1) {
    const difference = (a.numbers[index] ?? 0) - (b.numbers[index] ?? 0);
    if (difference !== 0) return difference;
  }
  if (a.prerelease === b.prerelease) return 0;
  if (!a.prerelease) return 1;
  if (!b.prerelease) return -1;
  return a.prerelease.localeCompare(b.prerelease, undefined, { numeric: true });
};

const formatDownloads = (value: number): string =>
  new Intl.NumberFormat(undefined, { notation: value >= 10_000 ? "compact" : "standard", maximumFractionDigits: 1 }).format(value);

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

function ArgumentListEditor({ values, label, addLabel, removeLabel, emptyLabel, onChange }: {
  values: string[];
  label: string;
  addLabel: string;
  removeLabel: string;
  emptyLabel: string;
  onChange: (values: string[]) => void;
}) {
  return (
    <div className="extension-argument-editor">
      <div className="extension-argument-editor__heading">
        <span>{label}</span>
        <button type="button" className="extensions-icon-button extension-argument-editor__add" aria-label={addLabel} title={addLabel} onClick={() => onChange([...values, ""])}>
          <Plus size={13} strokeWidth={2} aria-hidden="true" />
        </button>
      </div>
      {values.length === 0 ? <span className="extension-argument-editor__empty">{emptyLabel}</span> : values.map((value, index) => (
        <div className="extension-argument-editor__row" key={index}>
          <input aria-label={`${label} ${index + 1}`} value={value} onChange={(event) => onChange(values.map((item, itemIndex) => itemIndex === index ? event.target.value : item))} />
          <button type="button" className="extensions-icon-button" aria-label={removeLabel} title={removeLabel} onClick={() => onChange(values.filter((_, itemIndex) => itemIndex !== index))}>
            <X size={13} strokeWidth={2} aria-hidden="true" />
          </button>
        </div>
      ))}
    </div>
  );
}

export function ExtensionsPanel({ t, locale, onOpenCommand }: ExtensionsPanelProps) {
  const [tab, setTab] = useState<Tab>("installed");
  const [extensions, setExtensions] = useState<Extension[]>([]);
  const [latestById, setLatestById] = useState<Record<string, SearchResult>>({});
  const [loading, setLoading] = useState(true);
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<{ id: string; kind: MutationKind } | null>(null);
  const [query, setQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [hasSearched, setHasSearched] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [provider, setProvider] = useState<ProviderResponse | null>(null);
  const [diagnose, setDiagnose] = useState<DiagnoseResult | null>(null);
  const [configuration, setConfiguration] = useState<ExtensionConfiguration | null>(null);
  const [configValues, setConfigValues] = useState<Record<string, JsonValue>>({});
  const [savedConfigValues, setSavedConfigValues] = useState<Record<string, JsonValue>>({});
  const [configOperation, setConfigOperation] = useState<ConfigOperation>(null);
  const [configNotice, setConfigNotice] = useState<string | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [reviewingPackage, setReviewingPackage] = useState<string | null>(null);
  const [permissionReviews, setPermissionReviews] = useState<Record<string, PermissionReview>>({});
  const [pendingInstall, setPendingInstall] = useState<{ result: SearchResult; request: InstallRequest; review: PermissionReview } | null>(null);
  const [pendingLocal, setPendingLocal] = useState<{ review: PermissionReview; request: InstallRequest; name: string; runtime: string; platforms: string[]; source: string } | null>(null);
  const [syncOperation, setSyncOperation] = useState<SyncOperation | null>(null);
  const [exportResult, setExportResult] = useState<ExtensionsExportResult | null>(null);
  const [importReport, setImportReport] = useState<ExtensionsImportReport | null>(null);
  const [showCustomIntegration, setShowCustomIntegration] = useState(false);
  const [editingCustomId, setEditingCustomId] = useState<string | null>(null);
  const [customIntegrationLoading, setCustomIntegrationLoading] = useState(false);
  const [customIntegrationError, setCustomIntegrationError] = useState<string | null>(null);
  const [customContentOperation, setCustomContentOperation] = useState<CustomContentOperation>(null);
  const [customIntegration, setCustomIntegration] = useState<CustomIntegrationForm>(DEFAULT_CUSTOM_INTEGRATION);
  const [pathResults, setPathResults] = useState<PathExecutable[]>([]);
  const [pathSearching, setPathSearching] = useState(false);
  const [pathHighlight, setPathHighlight] = useState(0);
  const [customDirty, setCustomDirty] = useState(false);
  const customSavedRef = useRef<CustomIntegrationForm>(DEFAULT_CUSTOM_INTEGRATION);
  const [removalTarget, setRemovalTarget] = useState<RemovalTarget>(null);
  const [operationNotice, setOperationNotice] = useState<string | null>(null);
  const detailGeneration = useRef(0);
  const customCreateButtonRef = useRef<HTMLButtonElement | null>(null);
  const permissionDialogRef = useRef<HTMLElement | null>(null);
  const localDialogRef = useRef<HTMLElement | null>(null);
  const removalDialogRef = useRef<HTMLElement | null>(null);
  const customDialogRef = useRef<HTMLElement | null>(null);
  const drawerRef = useRef<HTMLElement | null>(null);

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
    setPathResults([]);
    setPathHighlight(0);
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

  const updates = useMemo(
    () => extensions.filter((extension) => {
      const latest = latestById[extension.id];
      return latest && compareVersions(latest.version, extension.currentVersion) > 0;
    }),
    [extensions, latestById],
  );
  const installedPackages = useMemo(
    () => new Set(extensions.map((extension) => extension.packageName).filter(Boolean)),
    [extensions],
  );
  const selected = extensions.find((extension) => extension.id === selectedId) ?? null;
  const configDirty = configuration?.descriptor.owner === "host"
    && JSON.stringify(configValues) !== JSON.stringify(savedConfigValues);

  const checkForUpdates = async (entries: Extension[]) => {
    const managed = entries.filter((entry) => entry.distributionSource === "npm" && entry.packageName && !entry.pinned);
    if (!managed.length) {
      setLatestById({});
      return;
    }
    setCheckingUpdates(true);
    const settled = await Promise.allSettled(
      managed.map(async (entry) => {
        const results = await invoke<SearchResult[]>("extensions_search", { query: entry.packageName, limit: 10 });
        return [entry.id, results.find((result) => result.package === entry.packageName) ?? null] as const;
      }),
    );
    const next: Record<string, SearchResult> = {};
    settled.forEach((result) => {
      if (result.status === "fulfilled" && result.value[1]) next[result.value[0]] = result.value[1];
    });
    setLatestById(next);
    setCheckingUpdates(false);
  };

  const refresh = async () => {
    setLoading(true);
    setError(null);
    try {
      const entries = await invoke<Extension[]>("extensions_list");
      setExtensions(entries);
      await checkForUpdates(entries);
    } catch (nextError) {
      setError(localErrorMessage(nextError, t));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    if (!showCustomIntegration || customIntegration.mode !== "executable") return;
    const query = customIntegration.executablePath.trim();
    const timer = window.setTimeout(() => {
      setPathSearching(true);
      void invoke<PathExecutable[]>("extensions_search_path", { query, limit: 12 })
        .then((results) => { setPathResults(results); setPathHighlight(0); })
        .catch(() => setPathResults([]))
        .finally(() => setPathSearching(false));
    }, 120);
    return () => window.clearTimeout(timer);
  }, [showCustomIntegration, customIntegration.mode, customIntegration.executablePath]);

  useEffect(() => {
    if (!showCustomIntegration) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (customIntegration.mode === "executable" && pathResults.length) {
        if (event.key === "ArrowDown") { event.preventDefault(); setPathHighlight((index) => Math.min(index + 1, pathResults.length - 1)); }
        if (event.key === "ArrowUp") { event.preventDefault(); setPathHighlight((index) => Math.max(index - 1, 0)); }
        if (event.key === "Enter" && document.activeElement?.getAttribute("data-path-search") === "true") { event.preventDefault(); choosePathExecutable(pathResults[pathHighlight]); }
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [showCustomIntegration, customIntegration.mode, pathResults, pathHighlight]);

  useEffect(() => {
    if (!selectedId) {
      detailGeneration.current += 1;
      setProvider(null);
      setDiagnose(null);
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
    setConfiguration(null);
    setDetailError(null);
    void Promise.allSettled([
      invoke<ProviderResponse>("extensions_describe", { id: selectedId }),
      invoke<DiagnoseResult>("extensions_diagnose", { id: selectedId }),
      invoke<ExtensionConfiguration>("extensions_config_get", { id: selectedId }),
    ]).then(([descriptionResult, diagnoseResult, configResult]) => {
      if (generation !== detailGeneration.current) return;
      if (descriptionResult.status === "fulfilled") setProvider(descriptionResult.value);
      if (diagnoseResult.status === "fulfilled") setDiagnose(diagnoseResult.value);
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
  }, [selectedId, selected?.updatedAt]);

  useDialogFocus(Boolean(pendingInstall), permissionDialogRef, () => {
    if (!busy) setPendingInstall(null);
  });
  useDialogFocus(Boolean(pendingLocal), localDialogRef, () => {
    if (!busy) setPendingLocal(null);
  });
  useDialogFocus(Boolean(removalTarget), removalDialogRef, () => {
    if (!busy) setRemovalTarget(null);
  });
  useDialogFocus(showCustomIntegration, customDialogRef, closeCustomIntegration);
  useDialogFocus(Boolean(selectedId), drawerRef, () => {
    if (!configDirty || window.confirm(t("settings.extensions.configDiscardConfirm"))) {
      setSelectedId(null);
    }
  }, !showCustomIntegration && !removalTarget && !pendingInstall && !pendingLocal);

  useEffect(() => {
    if (!showCustomIntegration || customIntegrationLoading) return;
    customDialogRef.current?.querySelector<HTMLElement>("[data-dialog-initial]")?.focus();
  }, [showCustomIntegration, customIntegrationLoading]);

  const runMutation = async (id: string, kind: MutationKind, action: () => Promise<unknown>): Promise<boolean> => {
    if (busy) return false;
    setBusy({ id, kind });
    setError(null);
    try {
      const result = await action();
      if (result === false) return false;
      await refresh();
      if (kind === "uninstall") setSelectedId(null);
      setOperationNotice(t("settings.extensions.operationComplete"));
      return true;
    } catch (nextError) {
      await refresh();
      setError(errorMessage(nextError));
      return false;
    } finally {
      setBusy(null);
    }
  };

  const exportExtensions = async () => {
    if (syncOperation) return;
    setSyncOperation("export");
    setError(null);
    setExportResult(null);
    setImportReport(null);
    try {
      const result = await invoke<ExtensionsExportResult | null>("extensions_export");
      if (result) setExportResult(result);
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      setSyncOperation(null);
    }
  };

  const importExtensions = async () => {
    if (syncOperation) return;
    setSyncOperation("import");
    setError(null);
    setExportResult(null);
    setImportReport(null);
    try {
      const report = await invoke<ExtensionsImportReport | null>("extensions_import", { locale });
      if (report) {
        setImportReport(report);
        await refresh();
      }
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      setSyncOperation(null);
    }
  };

  const toggleExtension = (extension: Extension) => runMutation(
    extension.id,
    extension.enabled ? "disable" : "enable",
    () => invoke(extension.enabled ? "extensions_disable" : "extensions_enable", { id: extension.id }),
  );

  const reviewPermissions = async (request: InstallRequest): Promise<PermissionReview> =>
    invoke<PermissionReview>("extensions_permissions_summary", { request, locale });

  const prefetchPermissions = async (result: SearchResult) => {
    if (busy || reviewingPackage || permissionReviews[result.package]) return;
    const request: InstallRequest = {
      source: "npm",
      package: result.package,
      version: result.version,
      manifestPath: null,
      executablePath: null,
    };
    setReviewingPackage(result.package);
    try {
      const review = await reviewPermissions(request);
      setPermissionReviews((current) => ({ ...current, [result.package]: review }));
    } catch {
      // Installation reports preflight failures explicitly when the user clicks.
    } finally {
      setReviewingPackage(null);
    }
  };

  const updateExtension = (extension: Extension) => {
    if (!extension.packageName) return Promise.resolve();
    const version = latestById[extension.id]?.version ?? extension.channel;
    const request: InstallRequest = {
      source: "npm",
      package: extension.packageName,
      version,
      manifestPath: null,
      executablePath: null,
    };
    return runMutation(extension.id, "update", async () => {
      const review = await reviewPermissions(request);
      if (review.permissions.length && !window.confirm(t("settings.extensions.confirmPermissionsInline", {
        permissions: review.permissions.map((permission) => permission.title).join(", "),
      }))) return false;
      await invoke("extensions_update", {
        id: extension.id,
        version,
        approvedPermissions: review.permissions.map(({ permission }) => permission),
      });
    });
  };

  const rollbackExtension = (extension: Extension) => {
    if (!window.confirm(t("settings.extensions.confirmRollback", { name: extension.name }))) return Promise.resolve();
    return runMutation(
      extension.id,
      "rollback",
      () => invoke("extensions_rollback", { id: extension.id }),
    );
  };

  const reinstallExtension = (extension: Extension) => {
    if (!window.confirm(t("settings.extensions.confirmReinstall", { name: extension.name }))) return Promise.resolve();
    return runMutation(
      extension.id,
      "reinstall",
      async () => {
        if (!extension.packageName) throw new Error(t("settings.extensions.reinstallUnavailable"));
        const request: InstallRequest = {
          source: "npm",
          package: extension.packageName,
          version: extension.currentVersion,
          manifestPath: null,
          executablePath: null,
        };
        const review = await reviewPermissions(request);
        if (review.permissions.length && !window.confirm(t("settings.extensions.confirmPermissionsInline", {
          permissions: review.permissions.map((permission) => permission.title).join(", "),
        }))) return false;
        await invoke<Extension>("extensions_reinstall", {
          id: extension.id,
          approvedPermissions: review.permissions.map(({ permission }) => permission),
        });
        if (!extension.enabled) await invoke("extensions_disable", { id: extension.id });
      },
    );
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
      setOperationNotice(t(
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

  const searchExtensions = async (event: FormEvent) => {
    event.preventDefault();
    const searchQuery = query.trim();
    if (!searchQuery || searching) return;
    setSearching(true);
    setHasSearched(true);
    setError(null);
    try {
      setSearchResults(await invoke<SearchResult[]>("extensions_search", { query: searchQuery, limit: 20 }));
    } catch (nextError) {
      setSearchResults([]);
      setError(errorMessage(nextError));
    } finally {
      setSearching(false);
    }
  };

  const performInstall = async (result: SearchResult, request: InstallRequest, review: PermissionReview) => {
    const installed = await runMutation(
      result.package,
      "install",
      () => invoke("extensions_install", {
        request: { ...request, approvedPermissions: review.permissions.map(({ permission }) => permission) },
      }),
    );
    if (installed) setOperationNotice(t("settings.extensions.installedNotice", { name: result.package }));
    return installed;
  };

  const installExtension = async (result: SearchResult) => {
    if (busy || reviewingPackage) return;
    const request: InstallRequest = {
      source: "npm",
      package: result.package,
      version: result.version,
      manifestPath: null,
      executablePath: null,
    };
    setReviewingPackage(result.package);
    setError(null);
    try {
      const review = permissionReviews[result.package] ?? await reviewPermissions(request);
      if (!permissionReviews[result.package]) {
        setPermissionReviews((current) => ({ ...current, [result.package]: review }));
      }
      setPendingInstall({ result, request, review });
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      setReviewingPackage(null);
    }
  };

  const connectBundled = async (extension: Extension) => {
    if (busy || !extension.runtimeAvailable) return;
    setBusy({ id: extension.id, kind: "install" });
    setError(null);
    try {
      const review = await invoke<PermissionReview>("extensions_bundled_permissions", { id: extension.id, locale });
      if (review.permissions.length && !window.confirm(t("settings.extensions.confirmPermissionsInline", {
        permissions: review.permissions.map((permission) => permission.title).join(", "),
      }))) return;
      await invoke("extensions_connect_bundled", {
        id: extension.id,
        executablePath: null,
        approvedPermissions: review.permissions.map(({ permission }) => permission),
      });
      await refresh();
      setOperationNotice(t("settings.extensions.connectedNotice", { name: extension.name }));
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  const connectLocal = async () => {
    if (busy) return;
    setBusy({ id: "local", kind: "install" });
    setError(null);
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
      setError(errorMessage(nextError));
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
      setOperationNotice(t("settings.extensions.connectedNotice", { name: pending.name }));
    } catch (nextError) { setError(errorMessage(nextError)); }
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
      const message = errorMessage(nextError);
      setCustomIntegrationError(message);
      setError(message);
    } finally {
      setCustomIntegrationLoading(false);
    }
  };

  const choosePathExecutable = (candidate: PathExecutable) => {
    const command = candidate.name.replace(/\.(exe|cmd|bat)$/i, "");
    const slug = command.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "custom-tool";
    updateCustomIntegration((current) => ({
      ...current,
      executablePath: candidate.path,
      name: current.name === DEFAULT_CUSTOM_INTEGRATION.name ? command : current.name,
      command: current.command === DEFAULT_CUSTOM_INTEGRATION.command ? slug : current.command,
      id: current.id === DEFAULT_CUSTOM_INTEGRATION.id ? `local.${slug}` : current.id,
    }));
    setPathResults([]);
  };

  const createCustomIntegration = async (event: FormEvent) => {
    event.preventDefault();
    if (busy) return;
    setBusy({ id: customIntegration.id, kind: editingCustomId ? "save" : "install" });
    setError(null);
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
      setOperationNotice(notice);
      await refresh();
    } catch (nextError) {
      const message = errorMessage(nextError);
      setCustomIntegrationError(message);
      setError(message);
    } finally {
      setBusy(null);
    }
  };

  const copyCustomContent = async (content: string, notice: string) => {
    if (customContentOperation) return;
    setCustomContentOperation("copy");
    setCustomIntegrationError(null);
    setError(null);
    try {
      await navigator.clipboard.writeText(content);
      setOperationNotice(notice);
    } catch (nextError) {
      const message = errorMessage(nextError);
      setCustomIntegrationError(message);
      setError(message);
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
    setError(null);
    const extension = customIntegration.scriptLanguage === "shell" ? "sh" : customIntegration.scriptLanguage === "powershell" ? "ps1" : "js";
    try {
      const path = await invoke<string | null>("extensions_custom_export_script", { id: customIntegration.id, content: customIntegration.scriptContent, extension });
      if (path) setOperationNotice(t("settings.extensions.customScriptExported"));
    } catch (nextError) {
      const message = errorMessage(nextError);
      setCustomIntegrationError(message);
      setError(message);
    } finally {
      setCustomContentOperation(null);
    }
  };

  const reconnectSystem = (extension: Extension) => runMutation(
    extension.id,
    "reinstall",
    () => invoke("extensions_reconnect_system", { id: extension.id }),
  );

  const confirmPendingInstall = async () => {
    if (!pendingInstall || busy) return;
    const pending = pendingInstall;
    if (await performInstall(pending.result, pending.request, pending.review)) {
      setPendingInstall(null);
    }
  };

  const updateAll = async () => {
    if (busy || !updates.length) return;
    setBusy({ id: "*", kind: "update" });
    setError(null);
    try {
      for (const extension of updates) {
        if (!extension.packageName) continue;
        const version = latestById[extension.id]?.version ?? extension.channel;
        const request: InstallRequest = {
          source: "npm",
          package: extension.packageName,
          version,
          manifestPath: null,
          executablePath: null,
        };
        const review = await reviewPermissions(request);
        if (review.permissions.length && !window.confirm(t("settings.extensions.confirmPermissionsInline", {
          permissions: review.permissions.map((permission) => permission.title).join(", "),
        }))) break;
        await invoke("extensions_update", {
          id: extension.id,
          version,
          approvedPermissions: review.permissions.map(({ permission }) => permission),
        });
      }
      await refresh();
    } catch (nextError) {
      setError(errorMessage(nextError));
      await refresh();
    } finally {
      setBusy(null);
    }
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
      setOperationNotice(t("settings.extensions.configSaved"));
    } catch (nextError) {
      const message = errorMessage(nextError);
      setDetailError(message);
      setError(message);
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
      setOperationNotice(t("settings.extensions.configCopied"));
    } catch (nextError) {
      const message = errorMessage(nextError);
      setDetailError(message);
      setError(message);
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
        setOperationNotice(t("settings.extensions.configExported"));
      }
    } catch (nextError) {
      const message = errorMessage(nextError);
      setDetailError(message);
      setError(message);
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
          disabled={loading || checkingUpdates}
          onClick={() => void refresh()}
        >
          <RefreshCw size={16} strokeWidth={2} aria-hidden="true" />
        </button>
      </div>

      <div className="extensions-tabs" role="tablist" aria-label={t("settings.extensions.title")}>
        {(["installed", "discover", "updates"] as Tab[]).map((item) => (
          <button
            key={item}
            type="button"
            role="tab"
            aria-selected={tab === item}
            className={`extensions-tabs__item${tab === item ? " extensions-tabs__item--active" : ""}`}
            onClick={() => setTab(item)}
          >
            {t(`settings.extensions.tab.${item}`)}
            {item === "installed" && extensions.length > 0 && <span>{extensions.length}</span>}
            {item === "updates" && updates.length > 0 && <span>{updates.length}</span>}
          </button>
        ))}
      </div>

      {error && (
        <div className="extensions-notice extensions-notice--error" role="alert">
          <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
          <span>{error}</span>
        </div>
      )}

      {operationNotice && (
        <div className="extensions-notice extensions-notice--success" role="status">
          <Check size={15} strokeWidth={2} aria-hidden="true" />
          <span>{operationNotice}</span>
          <button type="button" className="extensions-icon-button" aria-label={t("settings.extensions.dismissNotice")} onClick={() => setOperationNotice(null)}><X size={13} strokeWidth={2} /></button>
        </div>
      )}

      {tab === "installed" && (
        <div className="extensions-installed" role="tabpanel">
          <div className="extensions-sync-toolbar">
            <button
              type="button"
              className="extensions-action-button"
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
            <button
              type="button"
              className="extensions-action-button"
              disabled={Boolean(syncOperation) || Boolean(busy) || loading}
              onClick={() => void exportExtensions()}
            >
              <FileDown size={14} strokeWidth={2} aria-hidden="true" />
              {syncOperation === "export" ? t("settings.extensions.exporting") : t("settings.extensions.export")}
            </button>
            <button
              type="button"
              className="extensions-action-button"
              disabled={Boolean(syncOperation) || Boolean(busy) || loading}
              onClick={() => void importExtensions()}
            >
              <FileUp size={14} strokeWidth={2} aria-hidden="true" />
              {syncOperation === "import" ? t("settings.extensions.importing") : t("settings.extensions.import")}
            </button>
          </div>
          <div className="extensions-package-hint" role="note">
            <Link2 size={13} strokeWidth={2} aria-hidden="true" />
            <span>{t("settings.extensions.chooseManifestHint")}</span>
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
              <span>{t("settings.extensions.importSummary", {
                succeeded: importReport.succeeded.length,
                skipped: importReport.skipped.length,
                failed: importReport.failed.length,
              })}</span>
            </div>
          )}
          <div className="extensions-list">
            {loading ? (
              <EmptyState icon={<RefreshCw size={20} strokeWidth={2} />} text={t("settings.extensions.loading")} />
            ) : extensions.length === 0 ? (
              <EmptyState icon={<Package size={20} strokeWidth={2} />} text={t("settings.extensions.emptyInstalled")} />
            ) : extensions.map((extension) => (
              <ExtensionRow
                key={extension.id}
                extension={extension}
                latest={latestById[extension.id]}
                busy={Boolean(busy)}
                t={t}
                onOpen={() => setSelectedId(extension.id)}
                onConnect={() => void connectBundled(extension)}
                onRepair={() => extension.homepage && void invoke("open_url", { url: extension.homepage })}
                onReconnect={() => void reconnectSystem(extension)}
                onToggle={() => void toggleExtension(extension)}
                onUpdate={() => void updateExtension(extension)}
                onRollback={() => void rollbackExtension(extension)}
                onReinstall={() => void reinstallExtension(extension)}
                onEdit={() => void editCustomIntegration(extension)}
                onUninstall={() => uninstallExtension(extension)}
              />
            ))}
          </div>
        </div>
      )}

      {tab === "discover" && (
        <div className="extensions-discover" role="tabpanel">
          <form className="extensions-search" onSubmit={searchExtensions}>
            <Search size={16} strokeWidth={2} aria-hidden="true" />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t("settings.extensions.searchPlaceholder")}
              aria-label={t("settings.extensions.searchPlaceholder")}
            />
            <button type="submit" disabled={!query.trim() || searching}>
              {searching ? t("settings.extensions.searching") : t("settings.extensions.search")}
            </button>
          </form>
          <div className="extensions-source" aria-label={t("settings.extensions.source")}>
            <span className="extensions-source__active"><Check size={12} strokeWidth={2} />{t("settings.extensions.sourceNpm")}</span>
          </div>
          <div className="extensions-list">
            {searching ? (
              <EmptyState icon={<RefreshCw size={20} strokeWidth={2} />} text={t("settings.extensions.searching")} />
            ) : searchResults.length ? searchResults.map((result) => {
              const installed = installedPackages.has(result.package);
              return (
                <div className="extension-search-row" key={result.package}>
                  <div className="extension-search-row__main">
                    <div className="extension-search-row__title">
                      <strong>{result.package}</strong>
                      <span>v{result.version}</span>
                      <span className={`extension-trust-badge extension-trust-badge--${result.verified ? "official" : "community"}`}>
                        {result.verified ? <ShieldCheck size={11} strokeWidth={2} aria-hidden="true" /> : <Package size={11} strokeWidth={2} aria-hidden="true" />}
                        {t(result.verified ? "settings.extensions.trustOfficial" : "settings.extensions.trustCommunity")}
                      </span>
                    </div>
                    <p>{result.description || t("settings.extensions.noDescription")}</p>
                    <span className="extension-search-row__downloads">
                      <Download size={12} strokeWidth={2} aria-hidden="true" />
                      {t("settings.extensions.downloads", { count: formatDownloads(result.downloads) })}
                    </span>
                  </div>
                  <div className="extension-search-row__actions">
                    {permissionReviews[result.package]?.permissions.length ? (
                      <span className="extension-search-row__permissions" title={permissionReviews[result.package].permissions.map(({ description }) => description).join("\n")}>
                        <ShieldCheck size={12} strokeWidth={2} aria-hidden="true" />
                        {t("settings.extensions.permissionCount", { count: permissionReviews[result.package].permissions.length })}
                      </span>
                    ) : null}
                    <button
                      type="button"
                      className="extensions-action-button extensions-action-button--primary"
                      disabled={installed || Boolean(busy) || Boolean(reviewingPackage)}
                      onMouseEnter={() => void prefetchPermissions(result)}
                      onFocus={() => void prefetchPermissions(result)}
                      onClick={() => void installExtension(result)}
                    >
                      {installed
                        ? t("settings.extensions.installed")
                        : reviewingPackage === result.package
                          ? t("settings.extensions.reviewingPermissions")
                          : busy?.id === result.package
                            ? t("settings.extensions.installing")
                            : t("settings.extensions.install")}
                    </button>
                  </div>
                </div>
              );
            }) : (
              <EmptyState
                icon={<Search size={20} strokeWidth={2} />}
                text={hasSearched ? t("settings.extensions.emptySearch") : t("settings.extensions.searchPrompt")}
              />
            )}
          </div>
        </div>
      )}

      {tab === "updates" && (
        <div className="extensions-updates" role="tabpanel">
          <div className="extensions-updates__toolbar">
            <span>{checkingUpdates ? t("settings.extensions.checkingUpdates") : t("settings.extensions.updateCount", { count: updates.length })}</span>
            <button
              type="button"
              className="extensions-action-button extensions-action-button--primary"
              disabled={!updates.length || Boolean(busy)}
              onClick={() => void updateAll()}
            >
              <RefreshCw size={14} strokeWidth={2} aria-hidden="true" />
              {busy?.id === "*" ? t("settings.extensions.updating") : t("settings.extensions.updateAll")}
            </button>
          </div>
          <div className="extensions-list">
            {!loading && !checkingUpdates && updates.length === 0 ? (
              <EmptyState icon={<Check size={20} strokeWidth={2} />} text={t("settings.extensions.emptyUpdates")} />
            ) : updates.map((extension) => (
              <div className="extension-update-row" key={extension.id}>
                <button type="button" className="extension-update-row__main" onClick={() => setSelectedId(extension.id)}>
                  <strong>{extension.name}</strong>
                  <span>v{extension.currentVersion} → v{latestById[extension.id].version}</span>
                </button>
                <button
                  type="button"
                  className="extensions-action-button extensions-action-button--primary"
                  disabled={Boolean(busy)}
                  onClick={() => void updateExtension(extension)}
                >
                  {busy?.id === extension.id ? t("settings.extensions.updating") : t("settings.extensions.update")}
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {pendingInstall && (
        <div className="extension-permission-backdrop" role="presentation" onMouseDown={() => { if (!busy) setPendingInstall(null); }}>
          <section
            ref={permissionDialogRef}
            className="extension-permission-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="extension-permission-title"
            aria-describedby="extension-permission-hint extension-permission-boundary"
            tabIndex={-1}
            onMouseDown={stopRowClick}
          >
            <header>
              <ShieldCheck size={18} strokeWidth={2} aria-hidden="true" />
              <div>
                <h3 id="extension-permission-title">{t("settings.extensions.permissionTitle", { name: pendingInstall.review.extensionName })}</h3>
                <p id="extension-permission-hint">{t("settings.extensions.permissionHint")}</p>
              </div>
            </header>
            <div className="extension-permission-list">
              <div className="extension-install-trust">
                <strong>{t(pendingInstall.review.officialVerified ? "settings.extensions.trustOfficial" : "settings.extensions.trustCommunity")}</strong>
                <span>{t(pendingInstall.review.officialVerified
                  ? "settings.extensions.trustOfficialDescription"
                  : pendingInstall.review.publisherSigned
                    ? "settings.extensions.trustSelfSignedDescription"
                    : "settings.extensions.trustUnverifiedDescription")}</span>
              </div>
              {pendingInstall.review.permissions.map((permission) => (
                <div key={permission.permission}>
                  <strong>{permission.title}</strong>
                  <span>{permission.description}</span>
                </div>
              ))}
              <p id="extension-permission-boundary" className="extension-permission-dialog__boundary">{t("settings.extensions.permissionReviewBoundary")}</p>
            </div>
            <footer>
              <button type="button" className="extensions-action-button" disabled={Boolean(busy)} onClick={() => setPendingInstall(null)}>{t("settings.extensions.cancel")}</button>
              <button type="button" className="extensions-action-button extensions-action-button--primary" data-dialog-initial disabled={Boolean(busy)} onClick={() => void confirmPendingInstall()}>{busy ? t("settings.extensions.installing") : t("settings.extensions.approveInstall")}</button>
            </footer>
          </section>
        </div>
      )}

      {pendingLocal && (
        <div className="extension-permission-backdrop" role="presentation" onMouseDown={() => { if (!busy) setPendingLocal(null); }}>
          <section ref={localDialogRef} className="extension-permission-dialog" role="dialog" aria-modal="true" aria-labelledby="extension-local-title" aria-describedby="extension-local-hint extension-local-boundary" tabIndex={-1} onMouseDown={stopRowClick}>
            <header><ShieldCheck size={18} strokeWidth={2} aria-hidden="true" /><div><h3 id="extension-local-title">{t("settings.extensions.localConfirmTitle", { name: pendingLocal.name })}</h3><p id="extension-local-hint">{t("settings.extensions.localConfirmHint")}</p></div></header>
            <div className="extension-permission-list">
              <div><strong>{t("settings.extensions.localSource")}</strong><span>{pendingLocal.source === "local" ? t("settings.extensions.localDirectory") : pendingLocal.source}</span></div>
              <div><strong>{t("settings.extensions.localRuntime")}</strong><span>{pendingLocal.runtime}</span></div>
              <div><strong>{t("settings.extensions.customPlatforms")}</strong><span>{pendingLocal.platforms.join(", ") || t("settings.extensions.unavailable")}</span></div>
              {pendingLocal.review.permissions.map((permission) => <div key={permission.permission}><strong>{permission.title}</strong><span>{permission.description}</span></div>)}
              <p id="extension-local-boundary" className="extension-permission-dialog__boundary">{t("settings.extensions.permissionReviewBoundary")}</p>
            </div>
            <footer><button type="button" className="extensions-action-button" disabled={Boolean(busy)} onClick={() => setPendingLocal(null)}>{t("settings.extensions.cancel")}</button><button type="button" className="extensions-action-button extensions-action-button--primary" data-dialog-initial disabled={Boolean(busy)} onClick={() => void confirmLocal()}>{busy ? t("settings.extensions.installing") : t("settings.extensions.localConfirm")}</button></footer>
          </section>
        </div>
      )}

      {removalTarget && (
        <div className="extension-permission-backdrop" role="presentation" onMouseDown={() => { if (!busy) setRemovalTarget(null); }}>
          <section ref={removalDialogRef} className="extension-removal-dialog" role="alertdialog" aria-modal="true" aria-labelledby="extension-removal-title" aria-describedby="extension-removal-description" tabIndex={-1} onMouseDown={stopRowClick}>
            <header>
              <AlertCircle size={19} strokeWidth={2} aria-hidden="true" />
              <div>
                <h3 id="extension-removal-title">{t(removalTextKey(removalTarget, "Title"), { name: removalTarget.name })}</h3>
                <p id="extension-removal-description">{t(removalTextKey(removalTarget, "Description"))}</p>
              </div>
            </header>
            {removalTarget.generatedCustom && <div className="extension-removal-dialog__warning">{t("settings.extensions.deleteCustomWarning")}</div>}
            <footer>
              <button type="button" className="extensions-action-button" disabled={Boolean(busy)} onClick={() => setRemovalTarget(null)}>{t("settings.extensions.cancel")}</button>
              <button type="button" className="extensions-action-button extensions-action-button--danger" data-dialog-initial disabled={Boolean(busy)} onClick={() => void confirmRemoval()}>{busy ? t("settings.extensions.removing") : t(removalTextKey(removalTarget, ""))}</button>
            </footer>
          </section>
        </div>
      )}

      {showCustomIntegration && (
        <div className="extension-permission-backdrop" role="presentation" onMouseDown={closeCustomIntegration}>
          <section ref={customDialogRef} className="extension-custom-dialog" role="dialog" aria-modal="true" aria-labelledby="custom-integration-title" tabIndex={-1} onMouseDown={stopRowClick}>
            <header className="extension-custom-dialog__header">
              <div>
                <h3 id="custom-integration-title">{t(editingCustomId ? "settings.extensions.editCustomTitle" : "settings.extensions.createCustomTitle")}</h3>
                <p>{t(editingCustomId ? "settings.extensions.editCustomHint" : "settings.extensions.createCustomHint")}</p>
              </div>
              <button type="button" className="extensions-icon-button" aria-label={t("settings.extensions.cancel")} disabled={Boolean(busy) || customIntegrationLoading} onClick={closeCustomIntegration}><X size={16} strokeWidth={2} /></button>
            </header>
            {customIntegrationLoading && <div className="extension-custom-dialog__loading"><RefreshCw size={15} strokeWidth={2} /><span>{t("settings.extensions.loadingCustom")}</span></div>}
            {customIntegrationError && <div className="extensions-notice extensions-notice--error extension-custom-form__error" role="alert"><AlertCircle size={14} strokeWidth={2} aria-hidden="true" /><span>{customIntegrationError}</span></div>}
            {!customIntegrationLoading && (!editingCustomId || customIntegration.id === editingCustomId) && <form className="extension-custom-form" onSubmit={(event) => void createCustomIntegration(event)}>
              <div className="extension-custom-mode" role="radiogroup" aria-label={t("settings.extensions.customMode")}>
                {(["executable", "script"] as const).map((mode) => (
                  <button key={mode} type="button" role="radio" aria-checked={customIntegration.mode === mode} className={customIntegration.mode === mode ? "extension-custom-mode__item extension-custom-mode__item--active" : "extension-custom-mode__item"} onClick={() => updateCustomIntegration((current) => ({ ...current, mode }))}>
                    {t(mode === "executable" ? "settings.extensions.customModeExecutable" : "settings.extensions.customModeScript")}
                  </button>
                ))}
              </div>
              <div className="extension-custom-form__grid">
                <label><span>{t("settings.extensions.customName")}</span><input required maxLength={80} data-dialog-initial value={customIntegration.name} onChange={(event) => updateCustomIntegration((current) => ({ ...current, name: event.target.value }))} /></label>
                <label><span>{t("settings.extensions.customId")}</span><input required readOnly={Boolean(editingCustomId)} pattern="[a-z0-9]+([._-][a-z0-9]+)+" value={customIntegration.id} onChange={(event) => updateCustomIntegration((current) => ({ ...current, id: event.target.value.toLowerCase() }))} /></label>
                <label><span>{t("settings.extensions.customVersion")}</span><input required value={customIntegration.version} onChange={(event) => updateCustomIntegration((current) => ({ ...current, version: event.target.value }))} /></label>
                <label><span>{t("settings.extensions.customCommand")}</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={customIntegration.command} onChange={(event) => updateCustomIntegration((current) => ({ ...current, command: event.target.value.toLowerCase() }))} /></label>
                {customIntegration.mode === "executable" ? <>
                  <label className="extension-custom-form__wide"><span>{t("settings.extensions.customExecutable")}</span><input data-path-search="true" required autoComplete="off" placeholder={t("settings.extensions.customExecutablePlaceholder")} value={customIntegration.executablePath} onChange={(event) => updateCustomIntegration((current) => ({ ...current, executablePath: event.target.value }))} /></label>
                  <div className="extension-path-results extension-custom-form__wide" role="listbox" aria-label={t("settings.extensions.customPathResults")}>
                    {pathSearching ? <span>{t("settings.extensions.searching")}</span> : pathResults.length ? pathResults.map((candidate, index) => (
                      <button key={candidate.path} type="button" role="option" aria-selected={index === pathHighlight} className={index === pathHighlight ? "extension-path-result--active" : ""} onClick={() => choosePathExecutable(candidate)}><strong>{candidate.name}</strong><span>{candidate.path}</span></button>
                    )) : customIntegration.executablePath.trim() ? <span>{t("settings.extensions.customPathNoResults")}</span> : null}
                  </div>
                </> : <>
                  <label><span>{t("settings.extensions.customScriptLanguage")}</span><select value={customIntegration.scriptLanguage} onChange={(event) => updateCustomIntegration((current) => { const language = event.target.value as CustomIntegrationForm["scriptLanguage"]; const templates = ["", scriptTemplate("js"), scriptTemplate("shell"), scriptTemplate("powershell")]; return { ...current, scriptLanguage: language, scriptContent: templates.includes(current.scriptContent) ? scriptTemplate(language) : current.scriptContent }; })}><option value="js">JavaScript</option><option value="shell">Shell</option><option value="powershell">PowerShell</option></select></label>
                  <label className="extension-custom-form__wide"><span>{t("settings.extensions.customScriptContent")}</span><textarea required spellCheck={false} placeholder={t("settings.extensions.customScriptPlaceholder")} value={customIntegration.scriptContent} onChange={(event) => updateCustomIntegration((current) => ({ ...current, scriptContent: event.target.value }))} /></label>
                  <div className="extension-custom-form__actions extension-custom-form__wide"><button type="button" className="extensions-action-button" disabled={Boolean(customContentOperation)} onClick={() => void copyCustomContent(customIntegration.scriptContent, t("settings.extensions.customScriptCopied"))}><Copy size={14} />{t("settings.extensions.copyScript")}</button><button type="button" className="extensions-action-button" disabled={Boolean(customContentOperation)} onClick={() => void exportCustomScript()}><Download size={14} />{customContentOperation === "export" ? t("settings.extensions.exporting") : t("settings.extensions.exportScript")}</button></div>
                </>}
                {customIntegration.mode === "executable" && <div className="extension-custom-form__actions extension-custom-form__wide"><button type="button" className="extensions-action-button" disabled={Boolean(customContentOperation)} onClick={() => void copyCustomContent(customIntegration.executablePath, t("settings.extensions.customCommandCopied"))}><Copy size={14} />{t("settings.extensions.copyCommand")}</button><button type="button" className="extensions-action-button" disabled={Boolean(customContentOperation)} onClick={() => void copyExecutionPlan()}><Copy size={14} />{t("settings.extensions.copyPlan")}</button></div>}
                <ArgumentListEditor
                  values={customIntegration.argsPrefix}
                  label={t("settings.extensions.customArgsPrefix")}
                  addLabel={t("settings.extensions.customArgumentAdd")}
                  removeLabel={t("settings.extensions.customArgumentRemove")}
                  emptyLabel={t("settings.extensions.customNoArguments")}
                  onChange={(values) => updateCustomIntegration((current) => ({ ...current, argsPrefix: values }))}
                />
                <ArgumentListEditor
                  values={customIntegration.versionArgs}
                  label={t("settings.extensions.customVersionArgs")}
                  addLabel={t("settings.extensions.customArgumentAdd")}
                  removeLabel={t("settings.extensions.customArgumentRemove")}
                  emptyLabel={t("settings.extensions.customNoArguments")}
                  onChange={(values) => updateCustomIntegration((current) => ({ ...current, versionArgs: values }))}
                />
              </div>
              <fieldset className="extension-custom-permissions">
                <legend>{t("settings.extensions.customPlatforms")}</legend>
                {CUSTOM_PLATFORMS.map((platform) => (
                  <label key={platform}><input type="checkbox" checked={customIntegration.platforms.includes(platform)} onChange={(event) => updateCustomIntegration((current) => ({ ...current, platforms: event.target.checked ? [...current.platforms, platform] : current.platforms.filter((item) => item !== platform) }))} /><span>{platform === "darwin" ? "macOS" : platform === "linux" ? "Linux" : "Windows"}</span></label>
                ))}
              </fieldset>
              <div className="extension-custom-permission-boundary" role="note">
                <ShieldCheck size={15} strokeWidth={2} aria-hidden="true" />
                <span>{t("settings.extensions.permissionBoundary")}</span>
              </div>
              <fieldset className="extension-custom-permissions">
                <legend>{t("settings.extensions.customEnforcedPermissions")}</legend>
                <p className="extension-custom-permissions__hint">{t("settings.extensions.permissionEnforcedHint")}</p>
                {CUSTOM_ENFORCED_PERMISSION_NAMES.map((permission) => (
                  <label key={permission}><input type="checkbox" checked={customIntegration.permissions.includes(permission)} onChange={(event) => updateCustomIntegration((current) => ({ ...current, permissions: event.target.checked ? [...current.permissions, permission] : current.permissions.filter((item) => item !== permission) }))} /><span>{t(`settings.extensions.permission.${permission}`)}</span></label>
                ))}
              </fieldset>
              <fieldset className="extension-custom-permissions">
                <legend>{t("settings.extensions.customDeclaredPermissions")}</legend>
                <p className="extension-custom-permissions__hint">{t("settings.extensions.permissionDeclaredHint")}</p>
                {CUSTOM_DECLARED_PERMISSION_NAMES.map((permission) => (
                  <label key={permission}><input type="checkbox" checked={customIntegration.permissions.includes(permission)} onChange={(event) => updateCustomIntegration((current) => ({ ...current, permissions: event.target.checked ? [...current.permissions, permission] : current.permissions.filter((item) => item !== permission) }))} /><span>{t(`settings.extensions.permission.${permission}`)}</span></label>
                ))}
              </fieldset>
              <footer>
                <button type="button" className="extensions-action-button" disabled={Boolean(busy)} onClick={closeCustomIntegration}>{t("settings.extensions.cancel")}</button>
                <button type="submit" className="extensions-action-button extensions-action-button--primary" disabled={Boolean(busy) || customIntegration.platforms.length === 0 || (customIntegration.mode === "executable" ? !customIntegration.executablePath.trim() : !customIntegration.scriptContent.trim())}>{busy ? t(editingCustomId ? "settings.extensions.saving" : "settings.extensions.installing") : t(editingCustomId ? "settings.extensions.saveCustom" : "settings.extensions.createAndVerify")}</button>
              </footer>
            </form>}
          </section>
        </div>
      )}

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
              {detailLoading && <div className="extension-drawer__loading"><RefreshCw size={17} strokeWidth={2} />{t("settings.extensions.loadingDetails")}</div>}
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
                  <div><dt>{t("settings.extensions.signature")}</dt><dd>{t(selected.signatureVerified ? "settings.extensions.signatureVerified" : "settings.extensions.signatureMissing")}</dd></div>
                  <div><dt>{t("settings.extensions.trust")}</dt><dd>{t(selected.officialVerified ? "settings.extensions.trustOfficial" : "settings.extensions.trustCommunity")}</dd></div>
                  <div><dt>{t("settings.extensions.homepage")}</dt><dd>{selected.homepage ?? latestById[selected.id]?.homepage ?? t("settings.extensions.unavailable")}</dd></div>
                </dl>
                <p className="extension-detail-description">{provider?.description.provider.description || latestById[selected.id]?.description || t("settings.extensions.noDescription")}</p>
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
              {selected.generatedCustom && <button type="button" className="extensions-action-button extensions-action-button--primary" disabled={Boolean(busy)} onClick={() => void editCustomIntegration(selected)}>
                {t("settings.extensions.editCustom")}
              </button>}
              <button type="button" className="extensions-action-button" disabled={!selected.previousVersion || Boolean(busy)} onClick={() => void rollbackExtension(selected)}>
                <RotateCcw size={14} strokeWidth={2} />{t("settings.extensions.rollback")}
              </button>
              <button type="button" className="extensions-action-button" disabled={selected.distributionSource !== "npm" || !selected.packageName || Boolean(busy)} onClick={() => void reinstallExtension(selected)}>
                <RefreshCw size={14} strokeWidth={2} />{t("settings.extensions.reinstall")}
              </button>
              <button type="button" className="extensions-action-button extensions-action-button--danger" disabled={Boolean(busy)} onClick={() => uninstallExtension(selected)}>
                {removalKind(selected) === "system" ? <Unplug size={14} strokeWidth={2} /> : <Trash2 size={14} strokeWidth={2} />}
                {busy?.id === selected.id && busy.kind === "uninstall" ? t("settings.extensions.removing") : t(removalTextKey(selected, ""))}
              </button>
            </footer>
          </aside>
        </div>
      )}
    </section>
  );
}

type ExtensionRowProps = {
  extension: Extension;
  latest?: SearchResult;
  busy: boolean;
  t: Translate;
  onOpen: () => void;
  onConnect: () => void;
  onRepair: () => void;
  onReconnect: () => void;
  onToggle: () => void;
  onUpdate: () => void;
  onRollback: () => void;
  onReinstall: () => void;
  onEdit: () => void;
  onUninstall: () => void;
};

function ExtensionRow({ extension, latest, busy, t, onOpen, onConnect, onRepair, onReconnect, onToggle, onUpdate, onRollback, onReinstall, onEdit, onUninstall }: ExtensionRowProps) {
  const updateAvailable = Boolean(latest && compareVersions(latest.version, extension.currentVersion) > 0);
  return (
    <div className={`extension-row${extension.connected ? "" : " extension-row--detected"}`} onClick={extension.connected ? onOpen : undefined}>
      <div className="extension-row__icon"><Package size={17} strokeWidth={2} aria-hidden="true" /></div>
      <div className="extension-row__main">
        <div className="extension-row__title"><strong>{extension.name}</strong><span>v{extension.currentVersion}</span></div>
        <div className="extension-row__meta">
          <span>{t(integrationKindKey(extension))}</span>
          <span>{t(`settings.extensions.runtimeSource.${extension.runtimeSource}`)}</span>
          <span className={`extension-status extension-status--${extension.state}`}>{extension.connected ? t(`settings.extensions.status.${extension.state}`) : t("settings.extensions.status.notConnected")}</span>
          {!extension.runtimeAvailable && <span className="extension-status extension-status--broken">{t("settings.extensions.runtimeUnavailable")}</span>}
          {updateAvailable && <span className="extension-status extension-status--update">{t("settings.extensions.status.updateAvailable")}</span>}
        </div>
      </div>
      <div className="extension-row__actions" onClick={(event) => event.stopPropagation()}>
        {!extension.connected ? (
          <button type="button" className="extensions-action-button extensions-action-button--primary" disabled={busy || (!extension.runtimeAvailable && !extension.homepage)} onClick={extension.runtimeAvailable ? onConnect : onRepair}>
            {extension.runtimeAvailable ? <Link2 size={14} strokeWidth={2} aria-hidden="true" /> : <ExternalLink size={14} strokeWidth={2} aria-hidden="true" />}
            {extension.runtimeAvailable ? t("settings.extensions.connect") : t("settings.extensions.installTool")}
          </button>
        ) : extension.distributionSource === "npm" && (
          <button type="button" className={`extensions-action-button${updateAvailable ? " extensions-action-button--primary" : ""}`} disabled={!updateAvailable || busy} onClick={onUpdate}>
            {t("settings.extensions.update")}
          </button>
        )}
        {extension.connected && !extension.runtimeAvailable && (extension.reconnectAvailable || extension.homepage) && (
          <button type="button" className="extensions-action-button" disabled={busy} onClick={extension.reconnectAvailable ? onReconnect : onRepair}>
            {extension.reconnectAvailable ? <RefreshCw size={14} strokeWidth={2} aria-hidden="true" /> : <ExternalLink size={14} strokeWidth={2} aria-hidden="true" />}
            {t(extension.reconnectAvailable ? "settings.extensions.reconnect" : "settings.extensions.installTool")}
          </button>
        )}
        {extension.connected && <button
          type="button"
          role="switch"
          aria-checked={extension.enabled}
          aria-label={extension.enabled ? t("settings.extensions.disable") : t("settings.extensions.enable")}
          className={`settings-switch${extension.enabled ? " settings-switch--active" : ""}`}
          disabled={busy || extension.state === "broken"}
          onClick={onToggle}
        ><span className="settings-switch__thumb" /></button>}
        {extension.connected && <details className="extension-menu">
          <summary className="extensions-icon-button" aria-label={t("settings.extensions.moreActions")} title={t("settings.extensions.moreActions")}>
            <MoreHorizontal size={17} strokeWidth={2} aria-hidden="true" />
          </summary>
          <div className="extension-menu__items">
            {extension.generatedCustom && <button type="button" disabled={busy} onClick={onEdit}>{t("settings.extensions.editCustom")}</button>}
            <button type="button" disabled={!extension.previousVersion || busy} onClick={onRollback}><RotateCcw size={14} strokeWidth={2} />{t("settings.extensions.rollback")}</button>
            <button type="button" disabled={extension.distributionSource !== "npm" || !extension.packageName || busy} onClick={onReinstall}><RefreshCw size={14} strokeWidth={2} />{t("settings.extensions.reinstall")}</button>
            <button type="button" className="extension-menu__danger" disabled={busy} onClick={onUninstall}>
              {removalKind(extension) === "system" ? <Unplug size={14} strokeWidth={2} /> : <Trash2 size={14} strokeWidth={2} />}
              {t(removalKind(extension) === "custom" ? "settings.extensions.deleteCustom" : removalKind(extension) === "npm" ? "settings.extensions.uninstall" : removalKind(extension) === "system" ? "settings.extensions.disconnect" : "settings.extensions.removePackage")}
            </button>
          </div>
        </details>}
      </div>
      {extension.connected && <ChevronRight className="extension-row__chevron" size={15} strokeWidth={2} aria-hidden="true" />}
    </div>
  );
}

function EmptyState({ icon, text }: { icon: React.ReactNode; text: string }) {
  return <div className="extensions-empty">{icon}<span>{text}</span></div>;
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
