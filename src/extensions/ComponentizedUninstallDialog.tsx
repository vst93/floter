import { useEffect, useRef, useState } from "react";
import { Check } from "lucide-react";
import type { Translate } from "../i18n";
import type { Extension } from "../ExtensionsPanel";
import "./ComponentizedUninstallDialog.css";

type UninstallComponents = {
  removeProgram: boolean;
  removeHostConfig: boolean;
  removeToolData: boolean;
  removeArtifacts: boolean;
};

type Props = {
  extension: Extension;
  busy: boolean;
  t: Translate;
  onCancel: () => void;
  onConfirm: (components: UninstallComponents) => void;
};

export function ComponentizedUninstallDialog({
  extension,
  busy,
  t,
  onCancel,
  onConfirm,
}: Props) {
  const [components, setComponents] = useState<UninstallComponents>({
    removeProgram: true,
    removeHostConfig: true,
    removeToolData: true,
    removeArtifacts: true,
  });

  const cancelRef = useRef<HTMLButtonElement>(null);
  const onCancelRef = useRef(onCancel);
  onCancelRef.current = onCancel;

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    cancelRef.current?.focus({ preventScroll: true });
    const escape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.isComposing || event.keyCode === 229)
        return;
      event.preventDefault();
      event.stopImmediatePropagation();
      onCancelRef.current();
    };
    document.addEventListener("keydown", escape, true);
    return () => {
      document.removeEventListener("keydown", escape, true);
      if (previous?.isConnected && previous.getClientRects().length) {
        previous.focus({ preventScroll: true });
      }
    };
  }, []);

  const toggleComponent = (key: keyof UninstallComponents) => {
    setComponents((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const handleConfirm = () => {
    onConfirm(components);
  };

  return (
    <div
      className="extensions-dialog-overlay"
      role="dialog"
      aria-labelledby="uninstall-title"
      aria-modal="true"
    >
      <div className="extensions-dialog extensions-uninstall-dialog">
        <h3 id="uninstall-title" className="extensions-dialog-title">
          {t("settings.extensions.uninstallComponentsTitle", {
            name: extension.name,
          })}
        </h3>
        <p className="extensions-dialog-hint">
          {t("settings.extensions.uninstallComponentsHint")}
        </p>

        <div className="extensions-uninstall-components">
          <label className="extensions-uninstall-component">
            <input
              type="checkbox"
              checked={components.removeProgram}
              disabled
              aria-label={t("settings.extensions.uninstallProgram")}
            />
            <span className="extensions-component-label">
              {t("settings.extensions.uninstallProgram")}
            </span>
            <span className="extensions-component-hint">
              {t("settings.extensions.uninstallProgramHint")}
            </span>
          </label>

          <label className="extensions-uninstall-component">
            <input
              type="checkbox"
              checked={components.removeHostConfig}
              onChange={() => toggleComponent("removeHostConfig")}
              aria-label={t("settings.extensions.uninstallHostConfig")}
            />
            <span className="extensions-component-label">
              {t("settings.extensions.uninstallHostConfig")}
            </span>
            <span className="extensions-component-hint">
              {t("settings.extensions.uninstallHostConfigHint")}
            </span>
          </label>

          <label className="extensions-uninstall-component">
            <input
              type="checkbox"
              checked={components.removeToolData}
              onChange={() => toggleComponent("removeToolData")}
              aria-label={t("settings.extensions.uninstallToolData")}
            />
            <span className="extensions-component-label">
              {t("settings.extensions.uninstallToolData")}
            </span>
            <span className="extensions-component-hint">
              {t("settings.extensions.uninstallToolDataHint")}
            </span>
          </label>

          <label className="extensions-uninstall-component">
            <input
              type="checkbox"
              checked={components.removeArtifacts}
              onChange={() => toggleComponent("removeArtifacts")}
              aria-label={t("settings.extensions.uninstallArtifacts")}
            />
            <span className="extensions-component-label">
              {t("settings.extensions.uninstallArtifacts")}
            </span>
            <span className="extensions-component-hint">
              {t("settings.extensions.uninstallArtifactsHint")}
            </span>
          </label>
        </div>

        <div className="extensions-dialog-actions">
          <button
            ref={cancelRef}
            type="button"
            className="extensions-action-button"
            onClick={onCancel}
            disabled={busy}
          >
            {t("settings.extensions.cancel")}
          </button>
          <button
            type="button"
            className="extensions-action-button extensions-action-button--danger"
            onClick={handleConfirm}
            disabled={busy}
            data-destructive-confirm
          >
            <Check size={14} strokeWidth={2} />
            {t("settings.extensions.uninstall")}
          </button>
        </div>
      </div>
    </div>
  );
}
