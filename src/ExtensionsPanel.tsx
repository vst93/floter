import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent as ReactKeyboardEvent, type MouseEvent, type RefObject } from "react";
import { listen } from "@tauri-apps/api/event";
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
import { OverflowMenu } from "./components/OverflowMenu";
import type { DeepLinkRegisterRequest } from "./deep-link";
import type { Translate } from "./i18n";
import type { CommandAliases } from "./command-aliases";
import { resolveCommandAliases } from "./command-aliases";
import { useExtensionActions } from "./hooks/useExtensionActions";
import { ExtensionRow as ExtensionRowComponent } from "./extensions/ExtensionRow";
import { CustomIntegrationDrawer } from "./extensions/CustomIntegrationDrawer";
import { DEFAULT_OUTPUT_MODE, formatRunDuration, hasRunOutput, runAvailability, runOutputSummary } from "./extensions/run-routing";
import { LocalInstallDialog } from "./extensions/LocalInstallDialog";
import { PermissionTierList } from "./extensions/PermissionTierList";
import { permissionTier } from "./extensions/permission-tiers";
import { approvalIsStale, shortDigest } from "./extensions/approval-record";
import {
  SCRIPT_LANGUAGES,
  scriptExtension,
  type ScriptLanguageId,
  type ScriptRuntimeCheck,
} from "./extensions/script-languages";
import {
  fromWireParams,
  paramIssues,
  toWireParams,
  type ScriptParam,
  type ScriptParamWire,
} from "./extensions/script-params";
import { customIntegrationDirty } from "./extensions/integration-dirty";
import {
  paramRunErrorMessage,
  seedParamValues,
  type ParamValues,
} from "./extensions/run-params";
import { runErrorMessage } from "./extensions/run-errors";
import { failureReason } from "./extensions/binding-errors";
import {
  createReprobeNoticeGate,
  decideDriftNotice,
  freshnessOf,
  relativeTime,
  type Freshness,
} from "./extensions/freshness";
import { RemovalConfirmation } from "./extensions/RemovalConfirmation";
import { ComponentizedUninstallDialog } from "./extensions/ComponentizedUninstallDialog";
import { SettingsEmpty } from "./settings/SettingsRows";
import { useImmediateState } from "./hooks/useImmediateState";
import { useTimedReset } from "./hooks/useTimedReset";
import { isDismissKey } from "./surface-policy";

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
  /** Why the runtime is unavailable, from the single availability projector
   *  (`extensions::runtime_binding`). `null`/absent exactly when
   *  `runtimeAvailable` is true; a failure the catalog used to only log now
   *  reaches the row instead of leaving it claiming the tool was fine. */
  runtimeUnavailableCode?: string | null;
  runtimeUnavailableDetail?: string | null;
  reconnectAvailable: boolean;
  homepage: string | null;
  state: ExtensionStateKind;
  enabled: boolean;
  packageName: string | null;
  packageVersion: string;
  toolVersion: string | null;
  integrity: string | null;
  signatureVerified: boolean;
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
  /** Where a manual run sends its output, from the manifest (`background`
   *  when the field is absent). The row's inline switch edits this through the
   *  ordinary update transaction. */
  output: "background" | "terminal";
  /** R9-2 slice 3 · the declared inputs the run-time form renders, from the
   *  manifest. The backend re-reads `manifest.params` as the argv authority,
   *  so this is a rendering projection only. Absent on older payloads and on
   *  detected rows, which is why the run form treats `undefined` as empty. */
  params?: ScriptParamWire[] | null;
  /** Command list comes from a descriptor shipped with the publisher's
   *  release, so it tracks the release payload, not the local binary. */
  publisherDescriptor: boolean;
  recommended: boolean;
  /** Suggested from a convention-location manifest (~/.config/floter/tools). */
  manifestSuggestion?: boolean;
  toolLockState: "connected" | "reconnect-required" | "reverify-required" | null;
  toolCandidates: ExecutableToolCandidate[];
  /** Permission set recorded at approval time (audit trail). */
  approvedPermissions?: string[] | null;
  approvedAt?: number | null;
  approvedManifestDigest?: string | null;
  /** Digest of the manifest currently on disk (`sha256-…`), compared against
   *  `approvedManifestDigest` to report a post-approval change. Optional so a
   *  build that predates the field still type-checks. */
  currentManifestDigest?: string | null;
  lastErrorCode?: string | null;
  lastErrorDetail?: string | null;
  lastErrorAt?: number | null;
  /** Why the extension is broken, persisted until repair succeeds. */
  brokenReason?: string | null;
  /** Unix seconds of the last command probe (R7-7 freshness). Read from the
   *  integration's `help-probe.json` sidecar; `null`/absent for publisher
   *  descriptors and anything Floter did not generate. */
  lastProbeAt?: number | null;
  /** Commands in the descriptor as of that probe, and the probe before it.
   *  `null`/absent means "unknown" — a first probe has nothing to compare to. */
  commandCount?: number | null;
  previousCommandCount?: number | null;
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

/** The last background drift re-probe observed in this session, as reported by
 *  a notice frame on `extension-op-progress` (R7-7). Display-only: the durable
 *  copy of these numbers is the `help-probe.json` sidecar the list reads. */
type DriftProbe = {
  extensionId: string;
  toolVersion: string | null;
  atSeconds: number;
  commandCount: number | null;
  previousCommandCount: number | null;
};

/** One `extension-op-progress` frame. `notice` is present only on the one-shot
 *  frame a silent drift re-probe emits (R7-7) — see `operation.rs`. */
type OperationProgressPayload = {
  extensionId: string;
  kind: string;
  phase: string;
  percent?: number;
  notice?: {
    toolVersion?: string | null;
    previousCommandCount?: number | null;
    commandCount?: number | null;
  };
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

/** Captured stdout/stderr of one background run (R9-2). */
export type RunOutput = {
  stdout: string;
  stderr: string;
  truncated: boolean;
};

/** The outcome of one manual run. `plan` is present only for the terminal
 *  route, `output`/`exitCode`/`success` only for the background route. */
export type RunOutcome = {
  id: string;
  route: "terminal" | "background";
  plan?: ExtensionExecutionPlan;
  exitCode?: number;
  success?: boolean;
  durationMs: number;
  output?: RunOutput;
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
export type ExtensionOperation = { id: string; kind: MutationKind; operationId?: string } | null;
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

export type CustomIntegrationForm = {
  mode: "executable" | "script";
  id: string;
  name: string;
  command: string;
  version: string;
  executablePath: string;
  scriptLanguage: ScriptLanguageId;
  scriptContent: string;
  argsPrefix: string[];
  versionArgs: string[];
  permissions: PermissionName[];
  platforms: Array<"darwin" | "linux" | "windows">;
  output: "background" | "terminal";
  params: ScriptParam[];
};

const CURRENT_PLATFORM: "darwin" | "linux" | "windows" = navigator.userAgent.includes("Mac") ? "darwin" : navigator.userAgent.includes("Win") ? "windows" : "linux";

const DEFAULT_CUSTOM_INTEGRATION: CustomIntegrationForm = {
  mode: "executable",
  // R9-3 · the id is minted by the backend at creation and never authored or
  // derived in the frontend. The placeholder is empty on purpose: on create the
  // drawer shows no id at all, and on edit the loaded definition carries the
  // real, immutable one.
  id: "",
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
  output: DEFAULT_OUTPUT_MODE,
  params: [],
};

const SCRIPT_LANGUAGE_LABELS: Record<ScriptLanguageId, string> = Object.fromEntries(
  SCRIPT_LANGUAGES.map((language) => [language.id, language.label]),
) as Record<ScriptLanguageId, string>;

/** The human label for a language id, for prose (a toast naming the missing
 *  toolchain). Never translated: these are proper nouns. */
const scriptLanguageLabel = (id: ScriptLanguageId) => SCRIPT_LANGUAGE_LABELS[id] ?? id;

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
      (initial ?? dialog?.querySelector<HTMLElement>(FOCUSABLE_SELECTOR) ?? dialog)?.focus({ preventScroll: true });
    }, 0);
    const inertElements = new Map<HTMLElement, boolean>();
    let branch: HTMLElement | null = dialogRef.current?.parentElement ?? null;
    while (branch?.parentElement) {
      for (const sibling of Array.from(branch.parentElement.children)) {
        if (sibling !== branch && sibling instanceof HTMLElement) {
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
      // The Esc-or-mod-W predicate is shared with the surface tables
      // (`surface-policy.ts`), so the dialog and the window handler cannot
      // disagree about which press dismisses.
      if (isDismissKey(event)) {
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
        dialog.focus({ preventScroll: true });
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && (document.activeElement === first || !dialog.contains(document.activeElement))) {
        event.preventDefault();
        last.focus({ preventScroll: true });
      } else if (!event.shiftKey && (document.activeElement === last || !dialog.contains(document.activeElement))) {
        event.preventDefault();
        first.focus({ preventScroll: true });
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

type ExcludedFieldWarning = {
  extensionId: string;
  fieldCount: number;
};

type ExtensionsImportReport = {
  path: string;
  succeeded: ExtensionsImportItem[];
  failed: ExtensionsImportItem[];
  skipped: ExtensionsImportItem[];
  excludedFields: ExcludedFieldWarning[];
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
  /** R7-11: the raw `settings.command_aliases` map. The detail drawer edits one
   * command's alias through `onChangeCommandAlias`, which debounces the write
   * along the ordinary settings path. */
  commandAliases: CommandAliases;
  onChangeCommandAlias: (command: string, alias: string) => void;
  /** Registered base plugins — built-in functionality that ships with floter
   * and is switched on/off here (the ONE obvious place). */
  basePlugins: BasePluginRow[];
  /** Enable/disable a base plugin; tears its runtime down when disabled. */
  onToggleBasePlugin: (id: string, enabled: boolean) => void;
  /** Push a toast onto the app-level stack (rendered by App outside any scroll
   * container, so feedback stays visible wherever the user scrolled to). The
   * optional action rides the same stack — R7-7's drift notice uses it to open
   * the integration it is talking about. */
  onNotify: (kind: "error" | "success" | "warning", text: string, action?: { label: string; run: () => void }) => void;
  /** A validated `floter://connect` request. The backend has already checked
   * the manifest's path and structure; the panel turns it into the *same*
   * review dialog the file picker opens, so a link can never install, approve
   * or enable anything on its own. `null` once it has been consumed. */
  pendingDeepLink: { manifestPath: string; extensionName: string; source: string } | null;
  /** Report the hand-off as done so the request is not re-opened on a later
   * render. */
  onDeepLinkConsumed: () => void;
  /** A validated `floter://register` request (R8-3). The backend resolved the
   * `cmd` name against the discovery inventory and stopped; the panel only
   * *highlights* the matching Detected row (or explains why there is none), so
   * the user's own Connect press remains the one action that binds it. `null`
   * once consumed. */
  pendingDeepLinkRegister: DeepLinkRegisterRequest | null;
  /** Report the register hand-off as done, for the same one-shot reason. */
  onDeepLinkRegisterConsumed: () => void;
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
  /** `enforcement` is the backend's own classification (Rust
   *  `permission_enforcement`); the review UI renders it rather than deciding
   *  locally. Optional so an older/hand-built payload degrades to the local
   *  projection instead of failing. */
  permissions: Array<{ permission: PermissionName; enforcement?: "enforced" | "disclosed"; title: string; description: string }>;
  publisherSigned: boolean;
  deprecation: string | null;
};

/** The permission names the panel can label. An unknown name (a manifest from
 *  a newer version) falls back to the raw identifier rather than a blank. */
const PERMISSION_LABEL_KEYS: Record<string, Parameters<Translate>[0]> = {
  "filesystem-read": "settings.extensions.permission.filesystem-read",
  "filesystem-write": "settings.extensions.permission.filesystem-write",
  "network-fetch": "settings.extensions.permission.network-fetch",
  "process-spawn": "settings.extensions.permission.process-spawn",
  "clipboard-read": "settings.extensions.permission.clipboard-read",
  "clipboard-write": "settings.extensions.permission.clipboard-write",
  environment: "settings.extensions.permission.environment",
};

const permissionLabel = (permission: string, t: Translate): string => {
  const key = PERMISSION_LABEL_KEYS[permission];
  return key ? t(key) : permission;
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

export function ExtensionsPanel({ settingsBusy, t, locale, onOpenCommand, showCommandsInSearch, onToggleCommandsInSearch, commandAliases, onChangeCommandAlias, basePlugins, onToggleBasePlugin, onNotify, pendingDeepLink, onDeepLinkConsumed, pendingDeepLinkRegister, onDeepLinkRegisterConsumed }: ExtensionsPanelProps) {
  const [extensions, setExtensions] = useState<Extension[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [provider, setProvider] = useState<ProviderResponse | null>(null);
  const [diagnose, setDiagnose] = useState<DiagnoseResult | null>(null);
  const [healthReport, setHealthReport] = useState<HealthReport | null>(null);
  const [healthLoading, setHealthLoading, healthLoadingRef] = useImmediateState(false);
  const [reprobingCommands, setReprobingCommands, reprobingRef] = useImmediateState(false);
  const [operationProgress, setOperationProgress] = useState<Record<string, { stage: string; message?: string }>>({});
  // R7-7: the silent drift re-probe's one-shot notice. `driftProbe` records the
  // *result* of the background re-probe for one integration so the drawer's
  // freshness row can report it immediately (the next `extensions_list` carries
  // the same numbers from disk). It deliberately does NOT go through
  // `operationProgress`: that strip is per-row and cleared when a user-initiated
  // mutation ends, so a background frame parked there would pin a stale
  // "Complete" line to the row forever.
  const [driftProbe, setDriftProbe] = useState<DriftProbe | null>(null);
  const reprobeNoticeGate = useRef(createReprobeNoticeGate());
  const extensionsRef = useRef<Extension[]>(extensions);
  // R7-11: the alias map with the conflict policy applied, so the editor can
  // flag a command whose alias another command claimed first. Computed once per
  // settings change rather than per row.
  const resolvedAliases = useMemo(() => resolveCommandAliases(commandAliases), [commandAliases]);
  extensionsRef.current = extensions;
  // Bumped to force the drawer's details effect (provider/diagnose/config)
  // to reload without a lock-entry change, e.g. after a command re-probe.
  const [detailReloadTick, setDetailReloadTick] = useState(0);
  const [configuration, setConfiguration] = useState<ExtensionConfiguration | null>(null);
  const [configValues, setConfigValues] = useState<Record<string, JsonValue>>({});
  const [savedConfigValues, setSavedConfigValues] = useState<Record<string, JsonValue>>({});
  const [configOperation, setConfigOperation, configOperationRef] = useImmediateState<ConfigOperation>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [pendingLocal, setPendingLocal] = useState<{ review: PermissionReview; request: InstallRequest; name: string; runtime: string; platforms: string[]; source: string } | null>(null);
  const [pendingToolSelection, setPendingToolSelection, toolSelectionRef] = useImmediateState<PendingToolSelection>(null);
  const [pendingPermissionReview, setPendingPermissionReview] = useState<PendingPermissionReview>(null);
  const [syncOperation, setSyncOperation, syncOperationRef] = useImmediateState<SyncOperation | null>(null);
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
  /** The last failed one-click connect, shown inline in the Detected section.
   *  A detected row has no drawer to open, so a failure with no in-place home
   *  would leave the click with no visible result. Cleared on the next
   *  attempt, and never a substitute for the toast (both are shown). */
  const [detectedError, setDetectedError] = useState<{ id: string; message: string } | null>(null);
  /** R9-2 · manual-run state. `runBusy` is the id currently running (only one
   *  run at a time: a second click while the first is in flight would race the
   *  same integration), `runOutputs` is the last background output per id (a
   *  mirror of the backend's session store, refreshed on each run), and
   *  `outputOpen` is the set of rows whose inline output scroller is expanded. */
  const [runBusy, setRunBusy] = useState<string | null>(null);
  const [runOutputs, setRunOutputs] = useState<Record<string, RunOutput>>({});
  const [outputOpen, setOutputOpen] = useState<Record<string, boolean>>({});
  /** R9-2 slice 3 · the run-time parameter form. `runFormId` is the row whose
   *  form is open (one at a time), `runParamValues` is the live answers for
   *  that form, `runParamMemory` is the last submitted answers per integration
   *  (session memory only — never persisted), and `runParamError` is a backend
   *  refusal mapped back onto the form. */
  const [runFormId, setRunFormId] = useState<string | null>(null);
  const [runParamValues, setRunParamValues] = useState<ParamValues>({});
  const [runParamMemory, setRunParamMemory] = useState<Record<string, ParamValues>>({});
  const [runParamError, setRunParamError] = useState<string | null>(null);
  /** R8-3: the row a `floter://register` link asked us to highlight, and the
   *  inline reason when the name was not found. Purely presentational — a
   *  highlight is not a connect, and the row's own button stays the only way
   *  forward. It clears after a moment so the surface returns to its resting
   *  state on its own. */
  const [registerTarget, setRegisterTarget] = useState<{ id: string; args: string[] | null } | null>(null);
  const [registerMiss, setRegisterMiss] = useState<{ command: string; alreadyConnected: boolean } | null>(null);
  /** The Detected list, so a register highlight scrolls *it* and never the
   *  settings page (the same rule the suggestions list follows). */
  const detectedListRef = useRef<HTMLDivElement | null>(null);
  const [customDirty, setCustomDirty] = useState(false);  // Inline discard confirmations (replacing window.confirm): armed while the
  // bar above a drawer footer asks "discard unsaved changes?". Cleared as
  // soon as the user edits again, saves, or dismisses the bar.
  const [detailsDiscardArmed, setDetailsDiscardArmed] = useState(false);
  const [customDiscardArmed, setCustomDiscardArmed] = useState(false);
  const customSavedRef = useRef<CustomIntegrationForm>(DEFAULT_CUSTOM_INTEGRATION);
  const customGeneration = useRef(0);
  // R9-1 · the language picker's inline toolchain status. Cached per language
  // so re-selecting a language the user already checked is instant, and so the
  // probe runs once per language per drawer session rather than per keystroke.
  // A `null` entry means "asked, still in flight"; an absent key means "not
  // asked yet".
  const [runtimeChecks, setRuntimeChecks] = useState<Partial<Record<ScriptLanguageId, ScriptRuntimeCheck | null>>>({});
  const runtimeChecksRef = useRef(new Map<ScriptLanguageId, ScriptRuntimeCheck>());
  const runtimeChecksPending = useRef(new Map<ScriptLanguageId, Promise<ScriptRuntimeCheck>>());
  const runtimeChecksEpoch = useRef(0);
  const suppressToolSearch = useRef(false);
  const [removalTarget, setRemovalTarget] = useState<RemovalTarget>(null);
  const [uninstallDialogTarget, setUninstallDialogTarget] = useState<Extension | null>(null);
  const detailGeneration = useRef(0);
  const localDialogRef = useRef<HTMLElement | null>(null);
  const toolSelectionDialogRef = useRef<HTMLElement | null>(null);
  const permissionReviewDialogRef = useRef<HTMLElement | null>(null);
  const customDialogRef = useRef<HTMLElement | null>(null);
  const toolResultsRef = useRef<HTMLDivElement | null>(null);
  const drawerRef = useRef<HTMLElement | null>(null);
  const refreshGeneration = useRef(0);
  const refreshPending = useRef<Promise<void> | null>(null);
  const refreshRef = useRef<() => Promise<void>>(async () => {});
  // All panel feedback goes onto the app-level toast stack rather than a local
  // one: a local stack rendered inside the page scroller drifted off-screen
  // when the integrations list was long and the user had scrolled down.
  const showError = useCallback((text: string) => onNotify("error", text), [onNotify]);
  const showSuccess = useCallback((text: string) => onNotify("success", text), [onNotify]);
  const showWarning = useCallback((text: string) => onNotify("warning", text), [onNotify]);
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
  // R9-4 · the custom drawer's discard bar is NOT on a timer. It used to
  // disarm itself after 3s: the user pressed close, a bar appeared, and by the
  // time they reached for "discard" it had vanished, leaving the dialog open
  // with no explanation — the same silent dead end the placement fix removes.
  // It now stays until the user answers it (discard, cancel, or another edit),
  // which is the only state in which the bar's own question is answered.

  const updateCustomIntegration = (update: (current: CustomIntegrationForm) => CustomIntegrationForm) => {
    setCustomIntegrationError(null);
    setCustomDiscardArmed(false);
    setCustomIntegration((current) => {
      const next = update(current);
      setCustomDirty(customIntegrationDirty(next, customSavedRef.current));
      return next;
    });
  };

  const resetCustomIntegration = () => {
    customGeneration.current += 1;
    // A fresh drawer gets a fresh toolchain verdict: the user may have
    // installed the runtime since the last one.
    runtimeChecksEpoch.current += 1;
    runtimeChecksRef.current.clear();
    runtimeChecksPending.current.clear();
    setRuntimeChecks({});
    setCustomIntegrationLoading(false);
    setEditingCustomId(null);
    const fresh = { ...DEFAULT_CUSTOM_INTEGRATION, argsPrefix: [], versionArgs: [], permissions: [...DEFAULT_CUSTOM_INTEGRATION.permissions], platforms: [CURRENT_PLATFORM] as Array<"darwin" | "linux" | "windows"> };
    setCustomIntegration(fresh);
    setCustomIntegrationError(null);
    setToolResults([]);
    setToolSearchFailed(false);
    setToolHighlight(0);
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

  /** Probe the local toolchain for one script language. Read-only and
   *  non-blocking: a miss only annotates the form. In-flight probes are
   *  deduplicated and finished ones cached in a ref, so re-selecting a
   *  language — or saving while a probe is running — never fires a second
   *  PATH scan. `null` renders as "checking"; the map is the authority and
   *  `runtimeChecks` is its render mirror. */
  const probeScriptRuntime = useCallback(async (language: ScriptLanguageId): Promise<ScriptRuntimeCheck> => {
    const cached = runtimeChecksRef.current.get(language);
    if (cached) return cached;
    const pending = runtimeChecksPending.current.get(language);
    if (pending) return pending;
    const epoch = runtimeChecksEpoch.current;
    setRuntimeChecks((current) => ({ ...current, [language]: null }));
    const record = (result: ScriptRuntimeCheck) => {
      // A drawer that closed and reopened in between must not inherit the
      // previous session's verdict: the machine may have changed.
      if (epoch !== runtimeChecksEpoch.current) return result;
      runtimeChecksRef.current.set(language, result);
      setRuntimeChecks((current) => ({ ...current, [language]: result }));
      return result;
    };
    const request = invoke<ScriptRuntimeCheck>("extensions_script_runtime_check", { language })
      .then(record)
      // A probe that cannot run is not a verdict about the machine, but the
      // line must stop spinning, so it is recorded as a miss.
      .catch(() => record({ available: false, path: null, version: null, versionOutput: null, compiled: false, candidates: [] }))
      .finally(() => runtimeChecksPending.current.delete(language));
    runtimeChecksPending.current.set(language, request);
    return request;
  }, []);

  // Probe whenever the drawer is open on a script integration. The effect keys
  // on the language so switching the picker checks the newly chosen one; the
  // cache above keeps a re-selection from probing twice.
  useEffect(() => {
    if (!showCustomIntegration || customIntegration.mode !== "script") return;
    void probeScriptRuntime(customIntegration.scriptLanguage);
  }, [showCustomIntegration, customIntegration.mode, customIntegration.scriptLanguage, probeScriptRuntime]);

  const connectedExtensions = useMemo(() => extensions.filter((extension) => extension.connected), [extensions]);
  const suggestedExtensions = useMemo(() => extensions.filter((extension) => !extension.connected), [extensions]);
  const connectedPaths = useMemo(
    () => new Set(connectedExtensions.map((extension) => extension.executablePath)),
    [connectedExtensions],
  );
  const selected = extensions.find((extension) => extension.id === selectedId) ?? null;
  const configDirty = configuration?.descriptor.owner === "host"
    && JSON.stringify(configValues) !== JSON.stringify(savedConfigValues);

  const loadExtensions = async () => {
    const generation = ++refreshGeneration.current;
    setLoading(true);
    try {
      const entries = await invoke<Extension[]>("extensions_list");
      if (generation !== refreshGeneration.current) return;
      setExtensions(entries);
    } catch (nextError) {
      if (generation === refreshGeneration.current) showError(localErrorMessage(nextError, t));
    } finally {
      if (generation === refreshGeneration.current) setLoading(false);
    }
  };
  const refresh = () => {
    if (refreshPending.current) return refreshPending.current;
    const request = loadExtensions().finally(() => { refreshPending.current = null; });
    refreshPending.current = request;
    return request;
  };
  refreshRef.current = refresh;

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    const unlisten = listen<OperationProgressPayload>(
      "extension-op-progress",
      (event: { payload: OperationProgressPayload }) => {
        // The drift-notice frame is not a progress phase: it is a completed
        // background re-probe that no user action is waiting on. Routing it
        // through the per-row progress strip would leave "Complete" pinned to
        // the row (only `runMutation` clears it), so it is handled on its own
        // and skipped here.
        if (event.payload.notice) {
          handleDriftNoticeRef.current(event.payload);
          return;
        }
        setOperationProgress((prev) => ({
          ...prev,
          [event.payload.extensionId]: {
            stage: event.payload.phase,
            message: event.payload.kind,
          },
        }));
      }
    );
    return () => {
      void unlisten.then((fn: () => void) => fn());
    };
  }, []);

  useEffect(() => {
    // F1: the drawer is edit-only, so its executable field is search-driven on
    // every open — there is no "empty and idle" create state that would fall
    // back to a suggestion list (that fallback was the duplicate rendering of
    // the Detected section, and it is gone).
    if (!showCustomIntegration || customIntegration.mode !== "executable") return;
    if (suppressToolSearch.current) {
      // A candidate was just chosen (a Detected row's prefill, or a click in the
      // results list). Do not immediately re-open the list under the field.
      suppressToolSearch.current = false;
      setToolSearching(false);
      setToolSearchFailed(false);
      setToolResults([]);
      return;
    }
    const query = customIntegration.executablePath.trim();
    let cancelled = false;
    const timer = window.setTimeout(() => {
      setToolSearching(true);
      setToolSearchFailed(false);
      void invoke<ToolCandidate[]>("extensions_search_tools", { query, limit: 12, forceRefresh: false, executableOnly: true })
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
          setToolResults([]);
          setToolSearchFailed(true);
        })
        .finally(() => { if (!cancelled) setToolSearching(false); });
    }, 120);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [showCustomIntegration, customIntegration.mode, customIntegration.executablePath, connectedPaths]);

  useEffect(() => {
    // Keep the highlighted suggestion in view WITHOUT moving the page. A plain
    // `scrollIntoView` walks up every scrollable ancestor, so it also scrolled
    // `.settings-content` back to the top whenever the integration list was
    // long and the user had scrolled down — the reported jump-to-top on every
    // notification. Confine the scroll to the suggestions list itself.
    const list = toolResultsRef.current;
    const active = list?.querySelector<HTMLElement>("[aria-selected='true']");
    if (!list || !active) return;
    const listRect = list.getBoundingClientRect();
    const activeRect = active.getBoundingClientRect();
    if (activeRect.top < listRect.top) {
      list.scrollTop -= listRect.top - activeRect.top;
    } else if (activeRect.bottom > listRect.bottom) {
      list.scrollTop += activeRect.bottom - listRect.bottom;
    }
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
    customDialogRef.current?.querySelector<HTMLElement>("[data-dialog-initial]")?.focus({ preventScroll: true });
  }, [showCustomIntegration, customIntegrationLoading]);

  const runMutation = async (id: string, kind: MutationKind, action: () => Promise<unknown>): Promise<boolean> => {
    try {
      const result = await extensionActions.runMutation(id, kind, action);
      if (result && kind === "uninstall") setSelectedId(null);
      return result;
    } finally {
      // The progress strips are keyed per extension id and would otherwise keep
      // the last phase ("Complete") pinned to the row forever.
      setOperationProgress((prev) => {
        if (!(id in prev)) return prev;
        const next = { ...prev };
        delete next[id];
        return next;
      });
    }
  };

  const exportExtensions = async () => {
    if (syncOperationRef.current) return;
    setSyncOperation("export");
    setImportReport(null);
    try {
      const result = await invoke<ExtensionsExportResult | null>("extensions_export");
      if (result) {
        showSuccess(t("settings.extensions.exportedNotice", { count: result.extensionCount }));
      }
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setSyncOperation(null);
    }
  };

  const importExtensions = async () => {
    if (syncOperationRef.current) return;
    setSyncOperation("import");
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
    const kind = removalKind(extension);

    // For npm/custom/system/package, show componentized uninstall dialog
    if (kind === "npm" || kind === "custom") {
      setUninstallDialogTarget(extension);
      setRemovalTarget(null);
      return;
    }

    // System/package integrations: simple disconnect (no data removal)
    const removed = await runMutation(
      extension.id,
      "uninstall",
      () => invoke("extensions_uninstall", { id: extension.id, removeData: false }),
    );
    if (removed) {
      setRemovalTarget(null);
      showSuccess(t(
        kind === "system"
          ? "settings.extensions.disconnectedNotice"
          : "settings.extensions.packageRemovedNotice",
        { name: extension.name },
      ));
    }
  };

  const confirmComponentizedUninstall = async (
    extension: Extension,
    components: { removeProgram: boolean; removeHostConfig: boolean; removeToolData: boolean; removeArtifacts: boolean }
  ) => {
    if (busyRef.current) return;
    const removed = await runMutation(
      extension.id,
      "uninstall",
      () => invoke("extensions_uninstall_componentized", {
        request: {
          extensionId: extension.id,
          removeProgram: components.removeProgram,
          removeHostConfig: components.removeHostConfig,
          removeToolData: components.removeToolData,
          removeArtifacts: components.removeArtifacts,
        },
      }),
    );
    if (removed) {
      setUninstallDialogTarget(null);
      const kind = removalKind(extension);
      showSuccess(t(
        kind === "custom"
          ? "settings.extensions.customDeleted"
          : "settings.extensions.uninstalledNotice",
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
      await reviewLocalManifest(manifestPath);
    } catch (nextError) {
      showError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  /** Turn a manifest path into the review dialog. One function, two callers:
   * the file picker above, and a validated `floter://connect` request. A deep
   * link therefore reaches exactly the dialog a manual pick reaches — the
   * permission tiers and the boundary note included — and never anything
   * further. */
  const reviewLocalManifest = async (manifestPath: string) => {
    const request: InstallRequest = {
      source: "linked",
      package: null,
      version: null,
      manifestPath,
      executablePath: null,
    };
    const details = await invoke<{ extensionName: string; runtime: string; platforms: string[]; source: string; permissions: PermissionReview }>("extensions_local_manifest_review", { manifestPath, locale });
    setPendingLocal({ review: details.permissions, request, name: details.extensionName, runtime: details.runtime, platforms: details.platforms, source: details.source });
  };

  // A `floter://connect` request opens the review dialog. Deliberately *not* a
  // silent install: the only button that can install is the dialog's own
  // Connect, which runs `extensions_install` and therefore leaves the ordinary
  // approval record (`approvedPermissions` / `approvedAt` /
  // `approvedManifestDigest`).
  useEffect(() => {
    if (!pendingDeepLink || busyRef.current) return;
    onDeepLinkConsumed();
    setBusy({ id: "deep-link", kind: "install" });
    void reviewLocalManifest(pendingDeepLink.manifestPath)
      .catch((nextError) => showError(localErrorMessage(nextError, t)))
      .finally(() => setBusy(null));
    // The request is a one-shot hand-off; the callbacks are stable for the
    // panel's lifetime and re-running on a language change would re-open a
    // dialog the user may have just closed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingDeepLink]);

  // A `floter://register` request highlights the Detected row the backend
  // resolved the command to. Deliberately *only* a highlight: there is no
  // `extensions_connect_tool` call here, no permission review, and no state
  // mutation beyond the visual target — the row's own Connect button is still
  // the one action that binds the tool. When the backend found no executable
  // by that name the section shows an inline reason instead of failing
  // silently.
  const [registerPending, setRegisterPending] = useState<DeepLinkRegisterRequest | null>(null);
  useEffect(() => {
    if (!pendingDeepLinkRegister) return;
    onDeepLinkRegisterConsumed();
    setRegisterPending(pendingDeepLinkRegister);
    // A terminal `floter register <cmd>` that was allowed to bind has already
    // run the ordinary connect path by the time the event arrives. The panel
    // has to re-read the list for that case: the row it would have highlighted
    // is now a Connected entry, and the Detected list is stale without this.
    if (pendingDeepLinkRegister.confirmed) void refreshAfterMutation();
    // Same one-shot reasoning as the connect effect above: the app hands the
    // request over exactly once, and the callbacks are stable for the panel's
    // lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingDeepLinkRegister]);

  // Resolve the parked request against the list, and wait for the list. A cold
  // start delivers the link *before* the first `extensions_list` reply, so
  // deciding immediately would report "not found" for a tool that is right
  // there. This is also why the request is parked in local state rather than
  // resolved inside the effect above.
  useEffect(() => {
    if (!registerPending || loading) return;
    const request = registerPending;
    setRegisterPending(null);
    // A confirmed terminal invocation already finished: the backend bound the
    // tool through the ordinary connect path and the row is a Connected entry
    // now. There is no Detected row left to highlight, and the accurate
    // sentence is the "already connected" one — the same one a second
    // `floter register rg --yes` produces.
    if (request.bound) {
      setRegisterTarget(null);
      setRegisterMiss({ command: request.command, alreadyConnected: true });
      return;
    }
    const path = request.candidate?.locator.kind === "executable"
      ? request.candidate.locator.path
      : undefined;
    const match = path
      ? suggestedExtensions.find((extension) => extension.executablePath === path)
      : undefined;
    if (match) {
      setRegisterMiss(null);
      setRegisterTarget({ id: match.id, args: request.args ?? null });
      // The Detected section may be below the fold. Confine the scroll to the
      // Detected list itself — a plain `scrollIntoView` walks up every
      // scrollable ancestor and would reset `.settings-content` to the top
      // (the reported jump-to-top the suggestions list already avoids).
      window.requestAnimationFrame(() => {
        const list = detectedListRef.current;
        const active = list?.querySelector<HTMLElement>(".extension-row--register");
        if (!list || !active) return;
        const listRect = list.getBoundingClientRect();
        const activeRect = active.getBoundingClientRect();
        if (activeRect.top < listRect.top) list.scrollTop -= listRect.top - activeRect.top;
        else if (activeRect.bottom > listRect.bottom) list.scrollTop += activeRect.bottom - listRect.bottom;
      });
      return;
    }
    // A tool the backend resolved but that is already connected has no
    // Detected row to highlight. Saying "not found on this device" would be
    // false, so the two outcomes get two sentences.
    const alreadyConnected = path
      ? connectedExtensions.some((extension) => extension.executablePath === path)
      : false;
    setRegisterTarget(null);
    setRegisterMiss({ command: request.command, alreadyConnected });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [registerPending, loading]);

  // The highlight is a moment, not a mode: it fades on its own so the surface
  // returns to rest. The miss notice stays longer — it carries a reason the
  // user may need to read.
  useTimedReset(registerTarget, () => setRegisterTarget(null));
  useTimedReset(registerMiss, () => setRegisterMiss(null), 6000);

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
    if (busyRef.current || customLoadingRef.current) return;
    runtimeChecksEpoch.current += 1;
    runtimeChecksRef.current.clear();
    runtimeChecksPending.current.clear();
    setRuntimeChecks({});
    const draft: CustomIntegrationForm = {
      ...DEFAULT_CUSTOM_INTEGRATION,
      argsPrefix: [],
      versionArgs: [],
      permissions: [...DEFAULT_CUSTOM_INTEGRATION.permissions],
      platforms: [CURRENT_PLATFORM],
      params: [],
    };
    customGeneration.current += 1;
    customSavedRef.current = draft;
    setEditingCustomId(null);
    setCustomIntegration(draft);
    setCustomIntegrationError(null);
    setCustomDirty(false);
    setCustomDiscardArmed(false);
    setToolResults([]);
    setToolSearchFailed(false);
    setToolHighlight(0);
    suppressToolSearch.current = false;
    setShowCustomIntegration(true);
  };

  const editCustomIntegration = async (extension: Extension) => {
    if (!extension.generatedCustom || busyRef.current || customLoadingRef.current) return;
    resetCustomIntegration();
    runtimeChecksEpoch.current += 1;
    runtimeChecksRef.current.clear();
    runtimeChecksPending.current.clear();
    setRuntimeChecks({});
    const generation = customGeneration.current;
    setCustomIntegrationLoading(true);
    setEditingCustomId(extension.id);
    setCustomIntegrationError(null);
    setShowCustomIntegration(true);
    try {
      const definition = await invoke<CustomIntegrationForm>("extensions_custom_get", { id: extension.id });
      if (generation !== customGeneration.current) return;
      // R9-4 · one normalized editor form is written to BOTH the live state
      // and the saved baseline. The two used to be built by separate literal
      // spreads, so the baseline could disagree with the form the user was
      // looking at — and a raw JSON comparison then reported an untouched
      // legacy definition as dirty the moment it opened.
      const loaded: CustomIntegrationForm = {
        ...definition,
        scriptLanguage: definition.scriptLanguage ?? "shell",
        scriptContent: definition.scriptContent ?? "",
        argsPrefix: [...definition.argsPrefix],
        versionArgs: [...definition.versionArgs],
        permissions: [...definition.permissions],
        platforms: [...definition.platforms],
        params: fromWireParams(definition.params),
      };
      setCustomIntegration(loaded);
      customSavedRef.current = loaded;
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
    setCustomIntegration((current) => {
      const next = {
        ...current,
        executablePath: candidate.locator.path,
        name: current.name === DEFAULT_CUSTOM_INTEGRATION.name ? command : current.name,
        command: current.command === DEFAULT_CUSTOM_INTEGRATION.command ? slug : current.command,
      };
      setCustomDirty(customIntegrationDirty(next, customSavedRef.current));
      return next;
    });
    setCustomIntegrationError(null);
    setCustomDiscardArmed(false);
    setToolResults([]);
  };

  // Connect entry for a Detected row. This NEVER reconnects: reconnect re-scans
  // the tool inventory and writes a tool binding — an explicit, system-only
  // action (R3/G3) that a detection row must not trigger implicitly.
  //   - recommended / convention-location manifest: an authored manifest
  //     exists, so `connectRecommended` runs the shared pipeline (permission
  //     review, multi-candidate chooser).
  //   - bare PATH discovery: one click, zero forms (R8-2). The row already
  //     carries the real `ToolCandidate` the backend produced, so it is handed
  //     straight to `extensions_connect_tool`; the backend derives the id,
  //     command, version and description and applies the disclosure set.
  //     Rebuilding that candidate in the frontend (the old path) would be a
  //     second source of truth for the same decision, and it lost `sources`,
  //     `quality` and `fingerprint` on the way.
  const connectDetected = async (extension: Extension) => {
    if (busyRef.current) return;
    if (extension.recommended || extension.manifestSuggestion) {
      connectRecommended(extension);
      return;
    }
    const candidate = extension.toolCandidates.find(
      (entry) => entry.locator.kind === "executable" && entry.locator.path === extension.executablePath,
    ) ?? extension.toolCandidates[0];
    if (!candidate || candidate.locator.kind !== "executable") return;
    setDetectedError(null);
    setBusy({ id: extension.id, kind: "install" });
    try {
      // `approvedPermissions` is deliberately omitted: the backend's
      // `tool_binding_permissions()` is the single source of the disclosure
      // set, and a hard-coded copy here would be a second one.
      await invoke("extensions_connect_tool", { candidate });
      await refreshAfterMutation();
      showSuccess(t("settings.extensions.connectedNotice", { name: extension.name }));
    } catch (nextError) {
      // Inline, on the section — not only a toast. A connect that fails must
      // leave a visible reason next to the row that failed, or the click looks
      // like it did nothing.
      setDetectedError({ id: extension.id, message: errorMessage(nextError) });
      showError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  // R9-2 · run a connected integration once.
  //
  // The route is the manifest's `output` mode, decided by the backend — this
  // handler never picks a route itself. A terminal run returns a *protected*
  // plan and the only thing done with it here is handing it to the existing
  // `onOpenCommand` channel (terminal page + `term_spawn`); the frontend never
  // sees or assembles argv. A background run completes inline and reports
  // through the ordinary toast stack, and its captured output is stashed for
  // the row's inline "last output" scroller.
  //
  // R9-2 slice 3 · if the integration declares parameters, the Run button opens
  // the inline form seeded from this session's last answers (falling back to
  // each parameter's default); confirming calls the same backend command with
  // the collected `values`. An integration with no parameters runs straight
  // away, exactly as before.
  const declaredRunParams = (extension: Extension): ScriptParam[] =>
    fromWireParams(extension.params);

  const openRunForm = (extension: Extension) => {
    if (runBusy) {
      showWarning(t("settings.extensions.customRunAlreadyRunning"));
      return;
    }
    // Re-opening the form for the row it is already showing keeps the user's
    // in-progress edits; only a freshly opened form is seeded from memory.
    if (runFormId === extension.id) return;
    setRunFormId(extension.id);
    setRunParamError(null);
    setRunParamValues(seedParamValues(declaredRunParams(extension), runParamMemory[extension.id]));
  };

  const runExtension = async (extension: Extension, values?: ParamValues) => {
    if (runBusy) {
      // A second run request while one is still in flight is refused here and
      // on the backend (`run_already_in_flight`). The hint names the state
      // instead of the click appearing to do nothing.
      showWarning(t("settings.extensions.customRunAlreadyRunning"));
      return;
    }
    const params = declaredRunParams(extension);
    if (values === undefined && params.length > 0) {
      openRunForm(extension);
      return;
    }
    setRunBusy(extension.id);
    setRunParamError(null);
    try {
      const outcome = await invoke<RunOutcome>("extensions_run", {
        id: extension.id,
        values: values ?? null,
      });
      if (outcome.route === "terminal") {
        if (values !== undefined) {
          setRunParamMemory((current) => ({ ...current, [extension.id]: values }));
        }
        if (outcome.plan) await onOpenCommand(outcome.plan, extension.name);
        setRunFormId(null);
        return;
      }
      if (outcome.output) {
        setRunOutputs((current) => ({ ...current, [extension.id]: outcome.output as RunOutput }));
      }
      const duration = formatRunDuration(outcome.durationMs);
      if (outcome.success) {
        // Only a *completed* run clears the form and banks the answers; a
        // refusal keeps the form open so the user can fix the value in place.
        if (values !== undefined) {
          setRunParamMemory((current) => ({ ...current, [extension.id]: values }));
        }
        setRunFormId(null);
        const outputSummary = outcome.output && hasRunOutput(outcome.output)
          ? ` · ${runOutputSummary(outcome.output, t)}`
          : "";
        const canViewOutput = Boolean(outcome.output && hasRunOutput(outcome.output));
        onNotify(
          "success",
          t("settings.extensions.customRunSucceeded", { name: extension.name, duration }) + outputSummary,
          canViewOutput
            ? { label: t("settings.extensions.customViewOutput"), run: () => setOutputOpen((current) => ({ ...current, [extension.id]: true })) }
            : undefined,
        );
      } else {
        // A failing script is still a *completed run*: the exit code is the
        // fact the user needs, and the output is one click away in the row.
        if (values !== undefined) {
          setRunParamMemory((current) => ({ ...current, [extension.id]: values }));
        }
        setRunFormId(null);
        onNotify("warning", t("settings.extensions.customRunFailedToast", {
          name: extension.name,
          code: outcome.exitCode ?? 0,
        }));
        setOutputOpen((current) => ({ ...current, [extension.id]: true }));
      }
    } catch (nextError) {
      // A `run_param_*` refusal is an inline problem under the input that
      // caused it, not a toast — the form is still open and the user is one
      // edit away from a valid run. Anything else keeps the ordinary toast.
      const message = errorMessage(nextError);
      if (message.startsWith("run_already_in_flight:")) {
        // The backend refused because this integration already has a run in
        // flight (a second window, or a request that raced the local guard).
        showWarning(t("settings.extensions.customRunAlreadyRunning"));
        return;
      }
      const mapped = paramRunErrorMessage(
        message,
        params,
        (key, arguments_) => t(key as Parameters<Translate>[0], arguments_),
      );
      if (mapped) {
        setRunParamError(mapped);
        return;
      }
      // R9-5 · a run that could not even start (a missing script, a missing
      // interpreter, a spawn refusal) arrives as a keyed backend message. It is
      // an error the user must see: the toast says what was not found and
      // where the host looked, instead of the launcher's generic sentence.
      const runError = runErrorMessage(message, t);
      if (runError) {
        setOutputOpen((current) => ({ ...current, [extension.id]: true }));
        showError(runError);
        return;
      }
      showError(message);
    } finally {
      setRunBusy(null);
    }
  };

  const cancelRunForm = () => {
    setRunFormId(null);
    setRunParamError(null);
  };

  const changeRunParam = (id: string, value: string) => {
    setRunParamValues((current) => ({ ...current, [id]: value }));
    setRunParamError(null);
  };

  // R9-2 slice 5 · the output mode is configured in the drawer editor only.
  // The row no longer carries an inline switch (and therefore no longer has a
  // toggle handler here): the manifest is edited through the ordinary update
  // transaction in the drawer, so there is exactly one place the mode is set.

  const toggleOutputView = (id: string) => {
    setOutputOpen((current) => ({ ...current, [id]: !current[id] }));
  };

  const handleToolSearchKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
    if (!toolResults.length) return;
    const suggestions = toolResults;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setToolHighlight((index) => Math.min(index + 1, suggestions.length - 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setToolHighlight((index) => Math.max(index - 1, 0));
    } else if (event.key === "Enter") {
      event.preventDefault();
      chooseToolCandidate(suggestions[toolHighlight]);
    }
  };

  const createCustomIntegration = async (event: FormEvent) => {
    event.preventDefault();
    if (busyRef.current || customLoadingRef.current) return;
    const generation = customGeneration.current;
    // R9-1 · a missing toolchain does not block the save (a user may install
    // the runtime and come back), but it warns once so the integration is not
    // silently dead on arrival. A cached probe answers instantly; a pending
    // one is awaited so the warning reflects the truth rather than a race.
    if (customIntegration.mode === "script") {
      const check = await probeScriptRuntime(customIntegration.scriptLanguage);
      if (!check.available) {
        onNotify("warning", t("settings.extensions.customScriptRuntimeSaveWarning", {
          language: scriptLanguageLabel(customIntegration.scriptLanguage),
        }));
      }
    }
    // R9-3 · the busy marker is keyed by the addressed entry on edit and by the
    // form itself on create (there is no id yet — the backend mints one).
    setBusy({ id: editingCustomId ?? customIntegration.command, kind: editingCustomId ? "save" : "install" });
    setCustomIntegrationError(null);
    // R9-2 slice 2 · the parameter list is a capability declaration, so an
    // invalid row must not reach the backend. `ScriptParamEditor` already
    // renders the per-row message; this is the submit-side guard, shown inline
    // (never a toast — the offending row is right there).
    const paramProblem = paramIssues(customIntegration.params)[0];
    if (paramProblem) {
      setCustomIntegrationError(t(paramProblem.key as Parameters<Translate>[0]));
      setBusy(null);
      return;
    }
    try {
      const request = {
        ...customIntegration,
        // R9-3 · the frontend never authors or derives an id. On create it
        // sends the empty placeholder; the backend mints one. On edit the
        // addressed id comes from the path argument and the body is ignored.
        id: editingCustomId ?? "",
        executablePath: customIntegration.mode === "executable" ? customIntegration.executablePath : "",
        scriptLanguage: customIntegration.mode === "script" ? customIntegration.scriptLanguage : null,
        scriptContent: customIntegration.mode === "script" ? customIntegration.scriptContent : null,
        argsPrefix: customIntegration.argsPrefix,
        versionArgs: customIntegration.versionArgs,
        params: toWireParams(customIntegration.params),
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
    const extension = scriptExtension(customIntegration.scriptLanguage);
    try {
      const path = await invoke<string | null>("extensions_custom_export_script", { id: editingCustomId ?? customIntegration.command, content: customIntegration.scriptContent, extension });
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
    try {
      const json = await invoke<string>("extensions_config_copy", { id: selected.id, values: configValues });
      await navigator.clipboard.writeText(json);
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
    try {
      const path = await invoke<string | null>("extensions_config_export", { id: selected.id, values: configValues });
      if (path) {
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
    if (uninstallDialogTarget) { setUninstallDialogTarget(null); return; }
    if (detailsDiscardArmed) { setDetailsDiscardArmed(false); return; }
    if (configDirty) {
      setDetailsDiscardArmed(true);
      return;
    }
    setSelectedId(null);
  };

  // R7-7 · the silent drift re-probe, made visible exactly once.
  //
  // G2 re-probes a generated integration when the upstream tool's version moved
  // and rewrites its command list in the background. Nothing used to say so. A
  // frame with a `notice` block now arrives after such a re-probe, and this is
  // the only consumer: it announces a *changed command count* once per
  // (integration, version, delta) for the session, and stays silent for an
  // unchanged count — there is nothing a user could do about "still the same".
  //
  // The frame also marks the row as freshly probed so the drawer's freshness
  // section can report the new timestamp/count without a second list round
  // trip; the next `extensions_list` carries the same numbers from disk.
  const handleDriftNotice = (payload: OperationProgressPayload) => {
    const notice = payload.notice;
    if (!notice) return;
    // The name is resolved before the decision because the decision's "can this
    // be announced?" rule needs it — see `decideDriftNotice` for the ordering
    // that keeps a no-op frame from consuming the once-only slot.
    const extension = extensionsRef.current.find((entry) => entry.id === payload.extensionId);
    const decision = decideDriftNotice(
      {
        extensionId: payload.extensionId,
        toolVersion: notice.toolVersion,
        previousCommandCount: notice.previousCommandCount,
        commandCount: notice.commandCount,
      },
      extension?.name ?? null,
      reprobeNoticeGate.current,
    );
    // The drawer's facts are updated for every completed re-probe, announced or
    // not: the freshness row reports what happened, the toast only reports what
    // is worth interrupting for.
    setDriftProbe({ ...decision.display, atSeconds: Math.floor(Date.now() / 1000) });
    // R7-7b · and so is the *list row*. The dot on the row is projected from
    // the same fields the drawer reads, so without this the one place G2's
    // background re-probe is visible in the list would stay stale until some
    // unrelated mutation happened to call `refresh()` — the row would say "in
    // sync" about a command list that just changed. Fields absent from the
    // frame keep the list's value rather than being zeroed.
    const atSeconds = Math.floor(Date.now() / 1000);
    setExtensions((current) => current.map((entry) => entry.id === payload.extensionId
      ? {
        ...entry,
        lastProbeAt: atSeconds,
        commandCount: typeof notice.commandCount === "number" ? notice.commandCount : entry.commandCount,
        previousCommandCount: typeof notice.previousCommandCount === "number"
          ? notice.previousCommandCount
          : entry.previousCommandCount,
      }
      : entry));
    if (!decision.announce || !extension) return;
    const { delta } = decision.announce;
    onNotify(
      "success",
      t("settings.extensions.reprobeNotice", {
        name: extension.name,
        delta: t(
          delta.kind === "increase"
            ? "settings.extensions.reprobeNoticeDeltaIncrease"
            : "settings.extensions.reprobeNoticeDeltaDecrease",
          { count: delta.count },
        ),
      }),
      { label: t("settings.extensions.viewDetails"), run: () => setSelectedId(payload.extensionId) },
    );
  };
  const handleDriftNoticeRef = useRef(handleDriftNotice);
  handleDriftNoticeRef.current = handleDriftNotice;

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
  const uninstallDialog = uninstallDialogTarget && (
    <ComponentizedUninstallDialog
      extension={uninstallDialogTarget}
      busy={Boolean(busy)}
      t={t}
      onCancel={() => setUninstallDialogTarget(null)}
      onConfirm={(components) => void confirmComponentizedUninstall(uninstallDialogTarget, components)}
    />
  );

  // R7-3b: the toolbar's frequency split (G-24).
  //
  // Before: four buttons and two always-on caption lines sat between the
  // heading and the list, so the action a user repeats — connecting a local
  // package — was one of four equal-weight buttons, and the explanations for
  // the rare half were permanently on screen.
  //
  // The judgement per action, from the surface's own text:
  //   * Connect extension package — works on a manifest that already exists,
  //     i.e. the whole job with no authoring. It is the row's primary action.
  //   * Create custom — a blank authoring form. Every real custom integration
  //     starts from something on PATH, and the Detected section (R7-3a) now
  //     opens this same form prefilled from a detection, so the blank toolbar
  //     entry is the low-frequency duplicate. Kept, in the menu.
  //   * Export / Import — whole-collection file transfer, deliberately the
  //     rarest pair on the page ("Import and export a local JSON file"), and
  //     the only two whose captions were pure explanation. Kept, in the menu,
  //     with that one sentence as the menu's note instead of two standing
  //     caption lines.
  //
  // Nothing is deleted: all four actions are still one click (or one
  // keystroke) away, and the row itself drops to two controls.
  const overflowItems = [
    {
      id: "create",
      label: t("settings.extensions.createCustom"),
      icon: <Plus size={14} strokeWidth={2} aria-hidden="true" />,
      disabled: Boolean(syncOperation) || Boolean(busy) || loading,
      onSelect: openCreateCustomIntegration,
    },
    {
      id: "export",
      label: t("settings.extensions.export"),
      icon: <FileDown size={14} strokeWidth={2} aria-hidden="true" />,
      disabled: Boolean(syncOperation) || Boolean(busy) || loading,
      onSelect: () => void exportExtensions(),
    },
    {
      id: "import",
      label: t("settings.extensions.import"),
      icon: <FileUp size={14} strokeWidth={2} aria-hidden="true" />,
      disabled: Boolean(syncOperation) || Boolean(busy) || loading,
      onSelect: () => void importExtensions(),
    },
  ];

  return (
    <div className="settings-page settings-page--wide">
      <header className="settings-page__header">
        <h1 className="settings-page__title">{t("settings.extensions.title")}</h1>
        <p className="settings-page__subtitle">{t("settings.page.integrations")}</p>
      </header>
      <section className="settings-section extensions-panel">
      {/* PAGES-APPLY: the group heading no longer repeats the page title. The
          panel's own `extensions-section-title` rows (Base plugins / Connected
          / Detected) are the groups; this row is the page's toolbar, so it
          carries the group's one-line explanation on the left and the refresh
          action on the right — the same "hint + trailing action" heading the
          deep-link section uses. */}
      <div className="settings-section__heading extensions-panel__heading">
        <div>
          <p className="settings-section__hint settings-section__hint--inline">{t("settings.extensions.hint")}</p>
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
          {/* The visible row is one connect action plus the overflow trigger —
              two controls, no captions. The remaining three actions, the busy
              spinners and the explanation all live inside the menu. */}
          <div className="extensions-sync-toolbar">
            <div className="extensions-sync-toolbar__group">
              <button
                type="button"
                className="extensions-action-button extensions-action-button--primary"
                disabled={Boolean(syncOperation) || Boolean(busy) || loading}
                title={t("settings.extensions.chooseManifestHint")}
                onClick={() => void connectLocal()}
              >
                <Link2 size={14} strokeWidth={2} aria-hidden="true" />
                {t("settings.extensions.chooseManifest")}
              </button>
            </div>
            <div className="extensions-sync-toolbar__group extensions-sync-toolbar__group--overflow">
              {syncOperation && (
                <span className="extensions-sync-status" role="status">
                  <LoaderCircle className="extensions-spinner" size={13} strokeWidth={2} aria-hidden="true" />
                  {t(syncOperation === "export" ? "settings.extensions.exporting" : "settings.extensions.importing")}
                </span>
              )}
              <OverflowMenu
                label={t("settings.extensions.moreActions")}
                note={t("settings.extensions.fileTransferHint")}
                items={overflowItems}
                disabled={Boolean(syncOperation) || Boolean(busy) || loading}
              />
                        </div>
          </div>
        </div>
        {importReport && (
          <>
            {importReport.excludedFields.length > 0 && (
              <div className="extensions-notice extensions-notice--warning" role="status">
                <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
                <span>{t("settings.extensions.importExcludedFields", {
                  extensions: importReport.excludedFields.map(w => w.extensionId).join(", ")
                })}</span>
              </div>
            )}
            <div
              className={`extensions-notice${importReport.failed.length ? " extensions-notice--error" : " extensions-notice--success"}`}
              role="status"
              title={importReport.failed.length > 3 ? importReport.failed.map((item) => `${item.id}: ${item.message}`).join("\n") : importReport.path}
            >
              {importReport.failed.length
                ? <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
                : <Check size={15} strokeWidth={2} aria-hidden="true" />}
              <span>
                {importReport.failed.length
                  ? (
                    <>
                      {t("settings.extensions.importRolledBack")}
                      <span style={{ display: "block", marginTop: "0.5em", fontSize: "0.95em", opacity: 0.85 }}>
                        {t("settings.extensions.importFailedDetail", {
                          failures: importReport.failed.slice(0, 3).map(item => `${item.id}: ${item.message}`).join("; ")
                        })}
                        {importReport.failed.length > 3 && ` (${t("settings.extensions.importFailedMore", { count: importReport.failed.length - 3 })})`}
                      </span>
                    </>
                  )
                  : t("settings.extensions.importSummary", {
                      succeeded: importReport.succeeded.length,
                      skipped: importReport.skipped.length,
                    })
                }
              </span>
            </div>
          </>
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
            {/* The chip is derived synchronously from `extensions` — the same
                state the list below renders from — so the number and the rows
                always land in one commit. While a REFRESH is in flight the old
                count stays on screen (the rows do too); only the very first
                load (no data at all yet) shows the spinner. Swapping the
                number for a spinner on every mutation was the visible
                "middle state" that made the heading flicker. */}
            <span className="extension-status">
              {loading && extensions.length === 0
                ? <LoaderCircle className="extensions-spinner" size={11} strokeWidth={2} aria-hidden="true" />
                : connectedExtensions.length}
            </span>
          </h3>
          <div className="extensions-list extensions-list--installed">
            {loading && extensions.length === 0 ? (
              // A full-screen spinner is only honest for the FIRST load. On a
              // refresh (every mutation calls `refresh()`), swapping the whole
              // list for this one-line placeholder collapsed `.settings-content`
              // and clamped its `scrollTop` back to 0 — the reported jump to
              // the top on every notification. Once rows exist they stay
              // mounted; the header chip's spinner covers the in-flight state.
              <EmptyState icon={<LoaderCircle className="extensions-spinner" size={20} strokeWidth={2} />} text={t("settings.extensions.loading")} />
            ) : connectedExtensions.length === 0 ? (
              <EmptyState icon={<Package size={20} strokeWidth={2} />} text={t("settings.extensions.emptyInstalled")} />
            ) : connectedExtensions.map((extension) => (
              <Fragment key={extension.id}>
              <ExtensionRowComponent
                extension={extension}
                operation={busy}
                progress={operationProgress[extension.id]}
                t={t}
                onOpen={() => setSelectedId(extension.id)}
                onRepair={() => void repairExtension(extension)}
                onReconnect={() => void reconnectSystem(extension)}
                onToggle={() => void toggleExtension(extension)}
                onEdit={() => void editCustomIntegration(extension)}
                onUninstall={() => uninstallExtension(extension)}
                onRun={() => void runExtension(extension)}
                runBusy={runBusy === extension.id}
                runAvailable={runAvailability(extension)}
                runParams={declaredRunParams(extension)}
                runFormOpen={runFormId === extension.id}
                runParamValues={runParamValues}
                onRunParamChange={changeRunParam}
                onRunConfirm={(values) => void runExtension(extension, values)}
                onRunCancel={cancelRunForm}
                runError={runFormId === extension.id ? runParamError : null}
                lastOutput={runOutputs[extension.id] ?? null}
                outputOpen={Boolean(outputOpen[extension.id])}
                onToggleOutput={() => toggleOutputView(extension.id)}
                onCancelOperation={() => void invoke("extensions_cancel_operation", { operationId: "active" }).then(() => {
                  setOperationProgress((prev) => {
                    const next = { ...prev };
                    delete next[extension.id];
                    return next;
                  });
                  void refreshAfterMutation();
                }).catch((err) => showError(localErrorMessage(err, t)))}
              />
              {!selected && removalTarget?.id === extension.id && removalConfirmation}
              </Fragment>
            ))}
          </div>
        </section>
        {/* Detected: tools this device has but that are not connected yet.
            This is NOT a storefront — it is the local inventory the backend
            already returns as `connected === false` rows. The section renders
            nothing at all when there is nothing detected (flat, no empty-state
            placeholder), and each row connects through the existing connect
            flow, never reconnect. */}
        {/* R8-3 · the `floter://register` outcome, rendered *outside* the
            Detected section so it survives the section's "nothing detected →
            render nothing" contract: when the link names a tool this device
            does not have, there may be no Detected section at all, and the
            click must still not look like it did nothing. The technical reason
            stays a log line; the user is told what to do next. */}
        {registerMiss && (
          <div className="extensions-notice extensions-notice--error" role="alert">
            <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
            <span>
              {t(
                registerMiss.alreadyConnected
                  ? "settings.extensions.registerAlreadyConnected"
                  : "settings.extensions.registerNotFound",
                { name: registerMiss.command },
              )}
            </span>
          </div>
        )}
        {suggestedExtensions.length > 0 && (
          <section className="extensions-section extensions-section--detected">
            <h3 className="extensions-section-title">
              <span>{t("settings.extensions.section.detected")}</span>
              <span className="extension-status">{suggestedExtensions.length}</span>
            </h3>
            {/* R8-2 · permanent, section-level disclosure. One click now
                connects a discovered tool with no form, so the explanation of
                what that grants cannot live in a dialog the user never sees —
                it is always on, above the rows, in the same place every time.
                It states the two enforced capabilities and the disclosure
                one; nothing about `filesystem-write` / `network-fetch`, which
                are never part of a tool binding. */}
            <p className="extensions-section-hint extensions-section-hint--detected">
              {t("settings.extensions.detectedDisclosure")}
            </p>
            {detectedError && (
              <div className="extensions-notice extensions-notice--error" role="alert">
                <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
                <span>{t("settings.extensions.detectedConnectFailed", { name: suggestedExtensions.find((extension) => extension.id === detectedError.id)?.name ?? "" })}: {detectedError.message}</span>
              </div>
            )}
            <div className="extensions-list extensions-list--installed" ref={detectedListRef}>
              {suggestedExtensions.map((extension) => (
                <Fragment key={extension.id}>
                <ExtensionRowComponent
                  extension={extension}
                  operation={busy}
                  progress={operationProgress[extension.id]}
                  t={t}
                  highlighted={registerTarget?.id === extension.id}
                  onConnect={() => connectDetected(extension)}
                  onRepair={() => extension.homepage ? void invoke("open_url", { url: extension.homepage }).catch((error) => {
                    onNotify("error", String(error));
                  }) : undefined}
                />
                {/* The link's argument hint, shown as context only. The
                    backend never runs it; it is here so the user can see what
                    the link was about before deciding to connect. Rendered as
                    a sibling (not inside the row) so the list's row rhythm and
                    its hairline rule are untouched. */}
                {registerTarget?.id === extension.id && registerTarget.args && (
                  <p className="extensions-detected-slot__hint">
                    {t("settings.extensions.registerArgsHint", { args: registerTarget.args.join(" ") })}
                  </p>
                )}
                </Fragment>
              ))}
            </div>
          </section>
        )}
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
            <div className="extension-permission-list extension-permission-list--tiered">
              <span className="extension-permission-list__label">{t("settings.extensions.permissionsRequired")}</span>
              <PermissionTierList permissions={pendingPermissionReview.review.permissions} t={t} />
              <p className="extension-permission-dialog__boundary">{t("settings.extensions.permissionReviewBoundary")}</p>
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

      <CustomIntegrationDrawer open={showCustomIntegration} editingId={editingCustomId} loading={customIntegrationLoading} error={customIntegrationError} integration={customIntegration} busy={Boolean(busy)} contentOperation={customContentOperation} discardArmed={customDiscardArmed} onDismissDiscard={() => setCustomDiscardArmed(false)} onDiscard={discardCustomIntegration} toolResults={toolResults} toolSearching={toolSearching} toolSearchFailed={toolSearchFailed} toolHighlight={toolHighlight} toolResultsRef={toolResultsRef} dialogRef={customDialogRef} t={t} onClose={closeCustomIntegration} onSubmit={(event) => void createCustomIntegration(event)} onUpdate={updateCustomIntegration} onToolKeyDown={handleToolSearchKeyDown} onToolHighlight={setToolHighlight} onChooseTool={chooseToolCandidate} onCopy={copyCustomContent} onCopyPlan={() => void copyExecutionPlan()} onExportScript={() => void exportCustomScript()} runtimeCheck={runtimeChecks[customIntegration.scriptLanguage] ?? null} />

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
                <div className="extensions-notice extensions-notice--error"><AlertCircle size={15} strokeWidth={2} /><span>{t("settings.extensions.brokenDetail")}: {failureReason(selected.lastErrorCode, selected.brokenReason, t)}</span></div>
              )}
              {/* Not broken, yet not runnable: the binding or the executable is
                  gone. The reason comes from the same projector as
                  `runtimeAvailable`, so the row can name the cause instead of
                  only the symptom (audit G5). R11 · the code AND the detail go
                  through the dictionary: the box used to print
                  `不可用: binding-changed Executable fingerprint changed at …`. */}
              {!selected.runtimeAvailable && selected.runtimeUnavailableCode && (
                <div className="extensions-notice extensions-notice--error"><AlertCircle size={15} strokeWidth={2} /><span>{t("settings.extensions.runtimeUnavailableDetail")}: {failureReason(selected.runtimeUnavailableCode, selected.runtimeUnavailableDetail, t)}</span></div>
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
                    <div><dt>{t("settings.extensions.brokenDetail")}</dt><dd className="extension-metadata__dd--wrap" title={failureReason(selected.lastErrorCode, selected.brokenReason, t)}>{failureReason(selected.lastErrorCode, selected.brokenReason, t)}</dd></div>
                  )}
                  <div><dt>{t("settings.extensions.signature")}</dt><dd title={t(selected.signatureVerified ? "settings.extensions.signatureVerified" : "settings.extensions.signatureMissing")}>{t(selected.signatureVerified ? "settings.extensions.signatureVerified" : "settings.extensions.signatureMissing")}</dd></div>
                  <div><dt>{t("settings.extensions.homepage")}</dt><dd className="extension-metadata__dd--wrap" title={selected.homepage ?? t("settings.extensions.unavailable")}>{selected.homepage ?? t("settings.extensions.unavailable")}</dd></div>
                </dl>
                <p className="extension-detail-description">{provider?.description.provider.description || t("settings.extensions.noDescription")}</p>
                {selected.publisherDescriptor && (
                  <p className="extension-detail-note">{t("settings.extensions.publisherDescriptorNote")}</p>
                )}
              </section>

              <section className="extension-detail-block">
                <h4>{t("settings.extensions.approvalRecord")}</h4>
                {selected.approvedAt ? (
                  <div className="extension-approval-record">
                    <dl className="extension-metadata">
                      <div><dt>{t("settings.extensions.approvalRecordApprovedAt")}</dt><dd title={new Date(selected.approvedAt * 1000).toLocaleString(locale)}>{new Date(selected.approvedAt * 1000).toLocaleString(locale)}</dd></div>
                      <div>
                        <dt>{t("settings.extensions.approvalRecordDigest")}</dt>
                        <dd title={selected.approvedManifestDigest ?? t("settings.extensions.approvalRecordDigestUnknown")}>
                          {shortDigest(selected.approvedManifestDigest)
                            ? <code>{shortDigest(selected.approvedManifestDigest)}</code>
                            : t("settings.extensions.approvalRecordDigestUnknown")}
                        </dd>
                      </div>
                      {/* R7-8b · the on-disk digest, side by side with the
                          recorded one, so "what I approved" and "what is
                          here now" are two readable rows rather than only a
                          boolean. Unknown stays a label, not a false match. */}
                      <div>
                        <dt>{t("settings.extensions.approvalRecordDigestCurrent")}</dt>
                        <dd title={selected.currentManifestDigest ?? t("settings.extensions.approvalRecordDigestUnknown")}>
                          {shortDigest(selected.currentManifestDigest)
                            ? <code>{shortDigest(selected.currentManifestDigest)}</code>
                            : t("settings.extensions.approvalRecordDigestUnknown")}
                        </dd>
                      </div>
                    </dl>
                    <div className="extension-approval-record__permissions">
                      <span className="extension-approval-record__label">{t("settings.extensions.approvalRecordPermissions")}</span>
                      {selected.approvedPermissions?.length ? (
                        <ul>
                          {selected.approvedPermissions.map((permission) => (
                            <li
                              key={permission}
                              className={`extension-approval-record__permission extension-approval-record__permission--${permissionTier(permission)}`}
                            >
                              {permissionLabel(permission, t)}
                            </li>
                          ))}
                        </ul>
                      ) : (
                        <span className="extension-approval-record__empty">{t("settings.extensions.approvalRecordPermissionsNone")}</span>
                      )}
                    </div>
                    {approvalIsStale(selected.approvedManifestDigest, selected.currentManifestDigest) && (
                      <p className="extension-approval-record__stale">{t("settings.extensions.approvalRecordChanged")}</p>
                    )}
                  </div>
                ) : (
                  <p className="extension-detail-empty">{t("settings.extensions.approvalRecordNone")}</p>
                )}
              </section>

              <FreshnessSection
                t={t}
                locale={locale}
                selected={selected}
                driftProbe={driftProbe}
                healthReport={healthReport}
                reprobing={reprobingCommands}
              />

              <section className="extension-detail-block">
                <h4>{t("settings.extensions.commands")}</h4>
                {provider?.description.commands.length ? (
                  <div className="extension-command-list">
                    {provider.description.commands.map((command) => {
                      // The alias the conflict policy actually honours for this
                      // command. When the user typed one but another command
                      // claimed it first, the field still holds their text (the
                      // settings map is keyed by command name and never rewrites
                      // it) — but the row says so rather than pretending the
                      // alias is live. See `resolveCommandAliases`.
                      const honored = Boolean(resolvedAliases[command.id]);
                      const taken = Boolean((commandAliases[command.id] ?? "").trim()) && !honored;
                      return (
                        <div key={command.id} className="extension-command-list__row">
                          <code>{command.name}</code>
                          <span>{command.description || t("settings.extensions.noDescription")}</span>
                          {/* R7-11: the alias editor rides the command list itself
                              — one input per connected command — rather than a
                              second settings surface, so "this command, this
                              alias" is edited where the command is already named.
                              The key is `command.id` (the catalog's searchable
                              command), not the display name: that is the string
                              the launcher matches and the backend scores. */}
                          <label className="extension-command-alias">
                            <span className="extension-command-alias__label">
                              {t("settings.extensions.commandAlias")}
                            </span>
                            <input
                              type="text"
                              className="extension-command-alias__input"
                              aria-label={t("settings.extensions.commandAliasFor", { command: command.id })}
                              aria-invalid={taken || undefined}
                              placeholder={t("settings.extensions.commandAliasPlaceholder")}
                              value={commandAliases[command.id] ?? ""}
                              onChange={(event) => onChangeCommandAlias(command.id, event.target.value)}
                            />
                            {taken && (
                              <span className="extension-command-alias__taken" role="status">
                                {t("settings.extensions.commandAliasTaken")}
                              </span>
                            )}
                          </label>
                        </div>
                      );
                    })}
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
                      {configuration.descriptor.schema.map((field) => (
                        <ConfigFieldControl
                          key={field.key}
                          disabled={Boolean(busy)}
                          field={field}
                          value={configValues[field.key] ?? field.default}
                          t={t}
                          onChange={(value) => {
                            setDetailsDiscardArmed(false);
                            setConfigValues((current) => ({ ...current, [field.key]: value }));
                          }}
                        />
                      ))}
                      <div className="extension-config-form__actions">
                        <button type="button" className="extensions-action-button" disabled={Boolean(busy) || Boolean(configOperation)} onClick={() => { setConfigValues(configurationDefaults()); }}>
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

      {uninstallDialog}
      </section>
    </div>
  );
}

/**
 * R7-7 · the drawer's freshness block.
 *
 * Three facts, in this order: when the command list was last probed, what that
 * probe did, and how the command count moved against the probe before it. All
 * three come from storage that already existed (the probe sidecar, the
 * persisted health report, the operation-progress event); this component only
 * projects them.
 *
 * Deliberately NOT here: a version list, an "update" control, a "latest"
 * badge. The section answers "how fresh is what I have", never "is there
 * something newer to fetch" — the NPM update chain was removed (`350e2d6`) and
 * nothing in this block may imply its return (AGENT-NOTES: no store-style
 * update centre).
 *
 * R7-7b · G2's one-shot visibility is COMPLETE and is not duplicated in the
 * list row: the notice lives here (the toast + this block, driven by
 * `handleDriftNotice`), and the row only carries the *persistent* state
 * (`freshnessDotState`, no toast, no re-announcement). One change, one
 * interruption.
 */
function FreshnessSection({
  t,
  locale,
  selected,
  driftProbe,
  healthReport,
  reprobing,
}: {
  t: Translate;
  locale: "en" | "zh";
  selected: Extension;
  driftProbe: DriftProbe | null;
  healthReport: HealthReport | null;
  reprobing: boolean;
}) {
  // A drift re-probe observed in this session is the freshest truth for this
  // row; the list fields are what the sidecar said on the last listing.
  const drift = driftProbe && driftProbe.extensionId === selected.id ? driftProbe : null;
  const freshness: Freshness = freshnessOf({
    lastProbeAt: drift ? drift.atSeconds : selected.lastProbeAt,
    healthCheckedAt: healthReport?.checkedAt,
    healthStatus: healthReport?.status ?? null,
    errorCode: selected.lastErrorCode,
    running: reprobing,
    commandCount: drift ? drift.commandCount : selected.commandCount,
    previousCommandCount: drift ? drift.previousCommandCount : selected.previousCommandCount,
  });

  const deltaKey = {
    increase: "settings.extensions.freshnessDeltaIncrease",
    decrease: "settings.extensions.freshnessDeltaDecrease",
    unchanged: "settings.extensions.freshnessDeltaUnchanged",
    unknown: "settings.extensions.freshnessDeltaUnknown",
  }[freshness.delta.kind] as Parameters<Translate>[0];

  return (
    <section className="extension-detail-block">
      <h4>{t("settings.extensions.freshness")}</h4>
      <dl className="extension-metadata">
        <div>
          <dt>{t("settings.extensions.freshnessLastProbe")}</dt>
          <dd
            title={freshness.atSeconds
              ? new Date(freshness.atSeconds * 1000).toLocaleString(locale)
              : t("settings.extensions.freshnessNever")}
          >
            {freshness.atSeconds
              ? relativeTime(freshness.atSeconds, Math.floor(Date.now() / 1000), locale, t("settings.extensions.freshnessJustNow"))
              : t("settings.extensions.freshnessNever")}
          </dd>
        </div>
        <div>
          <dt>{t("settings.extensions.freshnessResult")}</dt>
          <dd>{t(`settings.extensions.freshnessResult.${freshness.result}` as Parameters<Translate>[0])}</dd>
        </div>
      </dl>
      <div className="extension-freshness__commands">
        <span className="extension-freshness__label">{t("settings.extensions.freshnessCommands")}</span>
        <span className="extension-freshness__count">
          {freshness.commandCount !== null
            ? t("settings.extensions.freshnessCommandsValue", { count: freshness.commandCount })
            : t("settings.extensions.freshnessCommandsUnknown")}
        </span>
        <span
          className={`extension-freshness__delta extension-freshness__delta--${freshness.delta.kind}`}
          aria-hidden="true"
        >
          ·
        </span>
        <span className={`extension-freshness__delta-text extension-freshness__delta-text--${freshness.delta.kind}`}>
          {t(deltaKey, { count: freshness.delta.count ?? 0 })}
        </span>
      </div>
      {freshness.source === "health" && (
        <p className="extension-detail-note extension-detail-note--secondary">{t("settings.extensions.freshnessHealthSource")}</p>
      )}
    </section>
  );
}

function EmptyState({ icon, text, query }: { icon: React.ReactNode; text: string; query?: string }) {
  // PAGES-APPLY · the integrations list's empty and first-load placeholders go
  // through the one empty region the app shares (the clipboard page's language,
  // lifted into `SettingsEmpty`). `query` rides the hint line, so a filtered
  // empty state still names the query it matched nothing against.
  return (
    <SettingsEmpty
      icon={icon}
      title={text}
      hint={query ? <strong className="extensions-empty__query">"{query}"</strong> : undefined}
    />
  );
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
