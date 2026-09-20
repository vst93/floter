import { AlertCircle, Check, Copy, Download, LoaderCircle, Plus, ShieldCheck, TriangleAlert, X } from "lucide-react";
import type { FormEvent, KeyboardEvent, RefObject } from "react";
import type { Translate } from "../i18n";
import type { CustomIntegrationForm, ExecutableToolCandidate } from "../ExtensionsPanel";
import { HOST_ENFORCED_PERMISSIONS, permissionTier } from "./permission-tiers";
import {
  SCRIPT_LANGUAGES,
  scriptRuntimeStatus,
  scriptTemplate,
  type ScriptLanguageId,
  type ScriptRuntimeCheck,
} from "./script-languages";
import {
  emptyParam,
  formatParamOptions,
  PARAM_KINDS,
  paramIssues,
  parseParamOptions,
  type ScriptParam,
  type ScriptParamKind,
} from "./script-params";

const PLATFORMS = ["darwin", "linux", "windows"] as const;
// R7-8 · the two lists come from the shared tier vocabulary, so the editor can
// never disagree with the review dialog about which permissions the host
// actually refuses. The enforced list is its own ordering; the declared list is
// every permission the tier map calls disclosure.
const ALL_PERMISSIONS = ["environment", "process-spawn", "filesystem-read", "filesystem-write", "network-fetch", "clipboard-read", "clipboard-write"] as const;
const ENFORCED = HOST_ENFORCED_PERMISSIONS;
const DECLARED = ALL_PERMISSIONS.filter((permission) => permissionTier(permission) === "disclosure");

type Props = {
  open: boolean; editingId: string | null; loading: boolean; error: string | null; integration: CustomIntegrationForm; busy: boolean; contentOperation: "copy" | "export" | null;
  discardArmed: boolean; onDismissDiscard: () => void; onDiscard: () => void;
  /** Executable search results only. R7-3b removed the gated suggestion
   *  fallback (received recommendations / detected extensions) that used to be
   *  slotted in here on an empty idle field: that list was a second rendering
   *  of the Detected section, and the drawer it lived in is now the *edit*
   *  entry (the blank create flow is reached from a Detected row instead). */
  toolResults: ExecutableToolCandidate[]; toolSearching: boolean; toolSearchFailed: boolean; toolHighlight: number; toolResultsRef: RefObject<HTMLDivElement | null>; dialogRef: RefObject<HTMLElement | null>; t: Translate;
  onClose: () => void; onSubmit: (event: FormEvent) => void; onUpdate: (update: (current: CustomIntegrationForm) => CustomIntegrationForm) => void; onToolKeyDown: (event: KeyboardEvent<HTMLInputElement>) => void; onToolHighlight: (index: number) => void; onChooseTool: (candidate: ExecutableToolCandidate) => void; onCopy: (content: string, notice: string) => void; onCopyPlan: () => void; onExportScript: () => void;
  /** R9-1 · the last `extensions_script_runtime_check` result for the selected
   *  language, or `null` while the probe is in flight / not yet asked. */
  runtimeCheck: ScriptRuntimeCheck | null;
};

type ToolResultOptionProps = {
  item: ExecutableToolCandidate;
  index: number;
  highlight: number;
  onHighlight: () => void;
  onChoose: () => void;
};

function ToolResultOption({ item, index, highlight, onHighlight, onChoose }: ToolResultOptionProps) {
  return <button id={`extension-tool-result-${index}`} type="button" role="option" tabIndex={-1} aria-selected={index === highlight} className={`extension-tool-result${index === highlight ? " extension-tool-result--active" : ""}`} onMouseEnter={onHighlight} onClick={onChoose}><strong>{item.name}{<small>{item.sources.join(" · ")}</small>}</strong><span>{item.locator.path}</span></button>;
}

function ArgumentListEditor({ values, label, addLabel, removeLabel, emptyLabel, onChange }: { values: string[]; label: string; addLabel: string; removeLabel: string; emptyLabel: string; onChange: (values: string[]) => void }) {
  return <div className="extension-argument-editor"><div className="extension-argument-editor__heading"><span>{label}</span><button type="button" className="extensions-icon-button extension-argument-editor__add" aria-label={addLabel} title={addLabel} onClick={() => onChange([...values, ""])}><Plus size={13} strokeWidth={2} aria-hidden="true" /></button></div>{values.length === 0 ? <span className="extension-argument-editor__empty">{emptyLabel}</span> : values.map((value, index) => <div className="extension-argument-editor__row" key={index}><input aria-label={`${label} ${index + 1}`} value={value} onChange={(event) => onChange(values.map((item, i) => i === index ? event.target.value : item))} /><button type="button" className="extensions-icon-button" aria-label={removeLabel} title={removeLabel} onClick={() => onChange(values.filter((_, i) => i !== index))}><X size={13} strokeWidth={2} /></button></div>)}</div>;
}

/** R9-2 slice 2 · the parameter *definition* editor.
 *
 *  This is the configuration half of "declare the inputs a script accepts".
 *  It is deliberately a sibling of `ArgumentListEditor` (same row language,
 *  same add/remove icon buttons) but each row expands into the fields a
 *  definition needs, because a fixed argument is one string and a parameter is
 *  a small record.
 *
 *  Everything here is inline: no dialog, no popover. The validation errors
 *  render under the row that caused them (`paramIssues` is the same rule the
 *  backend enforces), so a save never has to bounce back from the server to
 *  explain a typo. */
function ScriptParamEditor({ params, onChange, t }: { params: ScriptParam[]; onChange: (params: ScriptParam[]) => void; t: Translate }) {
  const issues = paramIssues(params);
  const issueFor = (index: number) => issues.find((issue) => issue.index === index);
  const kindLabel = (kind: ScriptParamKind) => t(`settings.extensions.customParamType.${kind}` as Parameters<Translate>[0]);
  const patch = (index: number, next: Partial<ScriptParam>) =>
    onChange(params.map((param, i) => (i === index ? { ...param, ...next } : param)));
  return <div className="extension-param-editor extension-custom-form__wide">
    <div className="extension-argument-editor__heading"><span>{t("settings.extensions.customParams")}</span><button type="button" className="extensions-icon-button extension-argument-editor__add" aria-label={t("settings.extensions.customParamAdd")} title={t("settings.extensions.customParamAdd")} onClick={() => onChange([...params, emptyParam()])}><Plus size={13} strokeWidth={2} aria-hidden="true" /></button></div>
    <p className="extension-param-editor__hint">{t("settings.extensions.customParamsHint")}</p>
    {params.length === 0
      ? <span className="extension-argument-editor__empty">{t("settings.extensions.customParamEmpty")}</span>
      : params.map((param, index) => <div className="extension-param-editor__row" key={index}>
        <div className="extension-param-editor__fields">
          <label><span>{t("settings.extensions.customParamLabel")}</span><input value={param.label} onChange={(event) => patch(index, { label: event.target.value })} /></label>
          <label><span>{t("settings.extensions.customParamId")}</span><input value={param.id} spellCheck={false} onChange={(event) => patch(index, { id: event.target.value })} /></label>
          <label><span>{t("settings.extensions.customParamType")}</span><select value={param.kind} onChange={(event) => patch(index, { kind: event.target.value as ScriptParamKind })}>{PARAM_KINDS.map((kind) => <option key={kind} value={kind}>{kindLabel(kind)}</option>)}</select></label>
          {param.kind !== "boolean" && <label><span>{t("settings.extensions.customParamPlaceholder")}</span><input value={param.placeholder ?? ""} onChange={(event) => patch(index, { placeholder: event.target.value })} /></label>}
          <label><span>{t("settings.extensions.customParamDefault")}</span><input value={param.default ?? ""} onChange={(event) => patch(index, { default: event.target.value })} /></label>
          <label><span>{t("settings.extensions.customParamFlag")}</span><input value={param.flag ?? ""} spellCheck={false} placeholder="--target" onChange={(event) => patch(index, { flag: event.target.value })} /></label>
        </div>
        <div className="extension-param-editor__controls">
          <label className="extension-param-editor__toggle"><input type="checkbox" checked={param.required} onChange={(event) => patch(index, { required: event.target.checked })} /><span>{t("settings.extensions.customParamRequired")}</span></label>
          <button type="button" className="extensions-icon-button" aria-label={t("settings.extensions.customParamRemove")} title={t("settings.extensions.customParamRemove")} onClick={() => onChange(params.filter((_, i) => i !== index))}><X size={13} strokeWidth={2} /></button>
        </div>
        {param.kind === "select" && <label className="extension-param-editor__options"><span>{t("settings.extensions.customParamOptions")}</span><input value={formatParamOptions(param.options)} onChange={(event) => patch(index, { options: parseParamOptions(event.target.value) })} /></label>}
        {issueFor(index) && <p className="extension-param-editor__error" role="alert"><AlertCircle size={12} strokeWidth={2} aria-hidden="true" /><span>{t(issueFor(index)!.key as Parameters<Translate>[0])}</span></p>}
      </div>)}
  </div>;
}

/** R9-1 · the inline toolchain status under the language picker.
 *
 *  Deliberately a caption, not a banner: it uses the same muted/hint voice as
 *  the rest of the form (no accent, no warm fill) because a missing toolchain
 *  is *not* an error — the integration can be saved and the runtime installed
 *  later. `check === null` means "probe in flight", which must not be painted
 *  as "missing". */
function ScriptRuntimeLine({ language, check, t }: { language: ScriptLanguageId; check: ScriptRuntimeCheck | null; t: Translate }) {
  const status = scriptRuntimeStatus(language, check);
  if (status.state === "checking") {
    return <p className="extension-custom-runtime" role="status"><LoaderCircle className="extensions-spinner" size={12} strokeWidth={2} aria-hidden="true" /><span>{t("settings.extensions.customScriptRuntimeChecking")}</span></p>;
  }
  if (status.state === "missing") {
    return <p className="extension-custom-runtime extension-custom-runtime--missing" role="status"><TriangleAlert size={12} strokeWidth={2} aria-hidden="true" /><span>{t("settings.extensions.customScriptRuntimeMissing", { names: status.names.join(", ") })}</span></p>;
  }
  return <p className="extension-custom-runtime extension-custom-runtime--ready" role="status" title={status.path ?? undefined}><Check size={12} strokeWidth={2} aria-hidden="true" /><span>{status.version ? `${status.name} ${status.version}` : status.name}</span>{status.compiled && <em>{t("settings.extensions.customScriptCompiled")}</em>}{status.path && <code>{status.path}</code>}</p>;
}

export function CustomIntegrationDrawer({ open, editingId, loading, error, integration, busy, contentOperation, discardArmed, onDismissDiscard, onDiscard, toolResults, toolSearching, toolSearchFailed, toolHighlight, toolResultsRef, dialogRef, t, onClose, onSubmit, onUpdate, onToolKeyDown, onToolHighlight, onChooseTool, onCopy, onCopyPlan, onExportScript, runtimeCheck }: Props) {
  if (!open) return null;
  const update = (fn: (current: CustomIntegrationForm) => CustomIntegrationForm) => onUpdate(fn);
  return <div className="extension-permission-backdrop" role="presentation" onMouseDown={onClose}><section ref={dialogRef} className="extension-custom-dialog" role="dialog" aria-modal="true" aria-labelledby="custom-integration-title" tabIndex={-1} onMouseDown={(event) => event.stopPropagation()}>
    <header className="extension-custom-dialog__header"><div><h3 id="custom-integration-title">{t(editingId ? "settings.extensions.editCustomTitle" : "settings.extensions.createCustomTitle")}</h3><p>{t(editingId ? "settings.extensions.editCustomHint" : "settings.extensions.createCustomHint")}</p></div><button type="button" className="extensions-icon-button" aria-label={t("settings.extensions.cancel")} onClick={onClose}><X size={16} strokeWidth={2} /></button></header>
    {loading && <div className="extension-custom-dialog__loading"><LoaderCircle className="extensions-spinner" size={15} strokeWidth={2} /><span>{t("settings.extensions.loadingCustom")}</span></div>}
    {error && <div className="extensions-notice extensions-notice--error extension-custom-form__error" role="alert"><AlertCircle size={14} strokeWidth={2} /><span>{error}</span></div>}
    {!loading && (!editingId || integration.id === editingId) && <form className="extension-custom-form" onSubmit={onSubmit}>
      <fieldset className="extension-custom-form__fields" disabled={busy} aria-busy={busy}>
      <div className="extension-custom-mode" role="radiogroup" aria-label={t("settings.extensions.customMode")}>{(["executable", "script"] as const).map((mode) => <button key={mode} type="button" role="radio" tabIndex={integration.mode === mode ? 0 : -1} aria-checked={integration.mode === mode} className={integration.mode === mode ? "extension-custom-mode__item extension-custom-mode__item--active" : "extension-custom-mode__item"} onClick={() => update((current) => ({ ...current, mode }))}>{t(mode === "executable" ? "settings.extensions.customModeExecutable" : "settings.extensions.customModeScript")}</button>)}</div>
      <div className="extension-custom-form__grid"><label><span>{t("settings.extensions.customName")}</span><input required maxLength={80} data-dialog-initial value={integration.name} onChange={(event) => update((current) => ({ ...current, name: event.target.value }))} /></label><label><span>{t("settings.extensions.customVersion")}</span><input required value={integration.version} onChange={(event) => update((current) => ({ ...current, version: event.target.value }))} /></label><label><span>{t("settings.extensions.customCommand")}</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={integration.command} onChange={(event) => update((current) => ({ ...current, command: event.target.value.toLowerCase() }))} /></label>
      {/* R9-3 · the id is minted by the backend at creation and is immutable
          afterwards, so the frontend never shows a value it could derive. On
          create the line is hidden entirely (there is no id yet); on edit it is
          a read-only caption identifying the integration. */}
      {editingId && <p className="extension-custom-form__derived"><span>{t("settings.extensions.customId")}</span><code>{integration.id}</code></p>}
      {integration.mode === "executable" ? <><div className="extension-custom-form__wide extension-tool-results-anchor"><label><span>{t("settings.extensions.customExecutable")}</span><input required role="combobox" aria-autocomplete="list" aria-controls="extension-tool-results" aria-expanded={toolSearching || toolResults.length > 0} aria-activedescendant={toolResults.length ? `extension-tool-result-${toolHighlight}` : undefined} autoComplete="off" placeholder={t("settings.extensions.customExecutablePlaceholder")} value={integration.executablePath} onKeyDown={onToolKeyDown} onChange={(event) => update((current) => ({ ...current, executablePath: event.target.value }))} /></label><p className="extension-custom-form__hint">{t("settings.extensions.changeExecutableHint")}</p><div ref={toolResultsRef} id="extension-tool-results" className="extension-tool-results" role="listbox" aria-label={t("settings.extensions.customExecutable")}>{toolSearching ? <span>{t("settings.extensions.searching")}</span> : toolResults.length ? toolResults.map((candidate, index) => <ToolResultOption key={`candidate-${candidate.id}`} item={candidate} index={index} highlight={toolHighlight} onHighlight={() => onToolHighlight(index)} onChoose={() => onChooseTool(candidate)} />) : toolSearchFailed ? <span role="status">{t("settings.extensions.customToolSearchFailed")}</span> : integration.executablePath.trim() ? <span>{t("settings.extensions.customToolNoResults")}</span> : null}</div></div></> : <><label className="extension-custom-form__wide"><span>{t("settings.extensions.customScriptLanguage")}</span><select value={integration.scriptLanguage} onChange={(event) => update((current) => { const language = event.target.value as ScriptLanguageId; return { ...current, scriptLanguage: language, scriptContent: ["", ...SCRIPT_LANGUAGES.map((option) => option.template)].includes(current.scriptContent) ? scriptTemplate(language) : current.scriptContent }; })}>{SCRIPT_LANGUAGES.map((language) => <option key={language.id} value={language.id}>{language.label}</option>)}</select></label><ScriptRuntimeLine language={integration.scriptLanguage} check={runtimeCheck} t={t} /><label className="extension-custom-form__wide"><span>{t("settings.extensions.customScriptContent")}</span><textarea required spellCheck={false} placeholder={t("settings.extensions.customScriptPlaceholder")} value={integration.scriptContent} onChange={(event) => update((current) => ({ ...current, scriptContent: event.target.value }))} /></label><div className="extension-custom-form__actions extension-custom-form__wide"><button type="button" className="extensions-action-button" disabled={Boolean(contentOperation)} onClick={() => onCopy(integration.scriptContent, t("settings.extensions.customScriptCopied"))}><Copy size={14} />{t("settings.extensions.copyScript")}</button><button type="button" className="extensions-action-button" disabled={Boolean(contentOperation)} onClick={onExportScript}><Download size={14} />{contentOperation === "export" ? t("settings.extensions.exporting") : t("settings.extensions.exportScript")}</button></div></>}
      {integration.mode === "executable" && <div className="extension-custom-form__actions extension-custom-form__wide"><button type="button" className="extensions-action-button" disabled={Boolean(contentOperation)} onClick={() => onCopy(integration.executablePath, t("settings.extensions.customCommandCopied"))}><Copy size={14} />{t("settings.extensions.copyCommand")}</button><button type="button" className="extensions-action-button" disabled={Boolean(contentOperation)} onClick={onCopyPlan}><Copy size={14} />{t("settings.extensions.copyPlan")}</button></div>}
      {/* R9-2 · where a manual run sends its output. Inline radio pair (not a
          select): two states, both named, and the hint states the consequence
          so "background" is not read as "discarded". */}
      <div className="extension-custom-output extension-custom-form__wide"><span className="extension-custom-output__label">{t("settings.extensions.customOutput")}</span><div className="extension-custom-mode" role="radiogroup" aria-label={t("settings.extensions.customOutput")}>{(["background", "terminal"] as const).map((mode) => <button key={mode} type="button" role="radio" tabIndex={integration.output === mode ? 0 : -1} aria-checked={integration.output === mode} className={integration.output === mode ? "extension-custom-mode__item extension-custom-mode__item--active" : "extension-custom-mode__item"} onClick={() => update((current) => ({ ...current, output: mode }))}>{t(mode === "terminal" ? "settings.extensions.customOutputTerminal" : "settings.extensions.customOutputBackground")}</button>)}</div><p className="extension-custom-form__hint">{t("settings.extensions.customOutputHint")}</p></div>
      <ArgumentListEditor values={integration.argsPrefix} label={t("settings.extensions.customArgsPrefix")} addLabel={t("settings.extensions.customArgumentAdd")} removeLabel={t("settings.extensions.customArgumentRemove")} emptyLabel={t("settings.extensions.customNoArguments")} onChange={(values) => update((current) => ({ ...current, argsPrefix: values }))} /><ArgumentListEditor values={integration.versionArgs} label={t("settings.extensions.customVersionArgs")} addLabel={t("settings.extensions.customArgumentAdd")} removeLabel={t("settings.extensions.customArgumentRemove")} emptyLabel={t("settings.extensions.customNoArguments")} onChange={(values) => update((current) => ({ ...current, versionArgs: values }))} />
      <ScriptParamEditor params={integration.params} t={t} onChange={(params) => update((current) => ({ ...current, params }))} />
      </div>
      <fieldset className="extension-custom-permissions"><legend>{t("settings.extensions.customPlatforms")}</legend>{PLATFORMS.map((platform) => <label key={platform}><input type="checkbox" checked={integration.platforms.includes(platform)} onChange={(event) => update((current) => ({ ...current, platforms: event.target.checked ? [...current.platforms, platform] : current.platforms.filter((item) => item !== platform) }))} /><span>{platform === "darwin" ? "macOS" : platform === "linux" ? "Linux" : "Windows"}</span></label>)}</fieldset>
      <div className="extension-custom-permission-boundary" role="note"><ShieldCheck size={15} strokeWidth={2} aria-hidden="true" /><span>{t("settings.extensions.permissionBoundary")}</span></div>
      {[ ["customEnforcedPermissions", ENFORCED], ["customDeclaredPermissions", DECLARED] ].map(([key, permissions]) => <fieldset className="extension-custom-permissions" key={key as string}><legend>{t(`settings.extensions.${key as string}` as Parameters<Translate>[0])}</legend><p className="extension-custom-permissions__hint">{t(key === "customEnforcedPermissions" ? "settings.extensions.permissionEnforcedHint" : "settings.extensions.permissionDeclaredHint")}</p>{(permissions as readonly string[]).map((permission) => <label key={permission}><input type="checkbox" checked={integration.permissions.includes(permission as never)} onChange={(event) => update((current) => ({ ...current, permissions: event.target.checked ? [...current.permissions, permission as never] : current.permissions.filter((item) => item !== permission) }))} /><span>{t(`settings.extensions.permission.${permission}` as Parameters<Translate>[0])}</span></label>)}</fieldset>)}
      </fieldset>
      <footer><button type="button" className="extensions-action-button" onClick={onClose}>{t("settings.extensions.cancel")}</button><button type="submit" className="extensions-action-button extensions-action-button--primary" disabled={busy || integration.platforms.length === 0 || (integration.mode === "executable" ? !integration.executablePath.trim() : !integration.scriptContent.trim())}>{busy ? t(editingId ? "settings.extensions.saving" : "settings.extensions.installing") : t(editingId ? "settings.extensions.saveCustom" : "settings.extensions.createAndVerify")}</button></footer>
    </form>}
    {/* R9-4 · the discard confirmation lives on the DIALOG, not inside the
        scrolling form. It used to be the last child of `.extension-custom-form`
        — a grid that is itself the `overflow-y: auto` viewport — so when the
        user had not scrolled to the very bottom the bar rendered below the
        visible area. A close attempt armed an invisible bar and the dialog
        appeared impossible to close (device report: “编辑弹窗没法关闭”).
        As a non-shrinking sibling of the form it is pinned to the dialog's
        bottom edge and always on screen, whatever the scroll position. */}
    {discardArmed && <div className="extensions-notice extensions-notice--error extension-discard-bar extension-discard-bar--dialog" role="alert"><AlertCircle size={14} strokeWidth={2} aria-hidden="true" /><span>{t("settings.extensions.customDiscardConfirm")}</span><button type="button" className="extensions-action-button" onClick={onDismissDiscard}>{t("settings.extensions.cancel")}</button><button type="button" data-destructive-confirm className="extensions-action-button extensions-action-button--danger" onClick={onDiscard}>{t("settings.extensions.discard")}</button></div>}
  </section></div>;
}
