import type { Translate } from "../i18n";
import { DeepLinkRow } from "./DeepLinkRow";
import { SettingsCard, SettingsRow } from "./SettingsRows";

const formatBytes = (bytes: number): string => {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const index = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  const value = bytes / 1024 ** index;
  return `${index === 0 ? Math.round(value) : value.toFixed(1)} ${units[index]}`;
};

type AboutPageProps = {
  t: Translate;
  appVersion: string;
  updateInfo: { version: string } | null;
  updateDownloading: boolean;
  updateProgress: { downloaded: number; total: number } | null;
  updateFailed: boolean;
  onDownloadUpdate: () => void;
  /** Feedback for the deep-link row's copy button; the toast stack is owned by
   *  `App`, so the page reports the event rather than painting its own. */
  onCopiedLink: (message: string) => void;
};

/** The about settings page: the version row (with the updater's action on its
 *  trailing edge) and the deep-link row.
 *
 *  This is the App Store row at its purest — an icon, a title, a grey second
 *  line and one right-aligned action — so the updater that used to be a
 *  self-contained tinted banner is now just that row. The accent survives on
 *  the one button, which is still the single thing on the page the user
 *  presses. */
export function AboutPage({
  t,
  appVersion,
  updateInfo,
  updateDownloading,
  updateProgress,
  updateFailed,
  onDownloadUpdate,
  onCopiedLink,
}: AboutPageProps) {
  const updatePercent =
    updateProgress && updateProgress.total > 0
      ? Math.min(100, (updateProgress.downloaded / updateProgress.total) * 100)
      : 0;
  const updateState = updateFailed
    ? t("settings.updateFailed")
    : updateInfo
      ? `${t("settings.latestVersion")}: v${updateInfo.version}`
      : t("settings.upToDate");
  return (
    <div className="settings-page">
      <header className="settings-page__header">
        <h1 className="settings-page__title">{t("settings.menu.about")}</h1>
        <p className="settings-page__subtitle">{t("settings.page.about")}</p>
      </header>
      <section className="settings-section">
        <div className="settings-section__heading">
          <div className="settings-section__heading-main">
            <h2 className="settings-section__label">{t("settings.group.update")}</h2>
          </div>
        </div>
        <SettingsCard label={t("settings.group.update")}>
          <SettingsRow
            label={`${t("settings.currentVersion")}: v${appVersion}`}
            sublabel={updateState}
            control={
              updateProgress ? (
                <div className="update-banner__progress">
                  <div className="update-banner__progress-track">
                    <div
                      className="update-banner__progress-bar"
                      style={{ width: `${updatePercent}%` }}
                    />
                  </div>
                  <span className="update-banner__progress-label">
                    {updateProgress.total > 0
                      ? `${Math.round(updatePercent)}% · ${formatBytes(updateProgress.downloaded)} / ${formatBytes(updateProgress.total)}`
                      : formatBytes(updateProgress.downloaded)}
                  </span>
                </div>
              ) : updateDownloading ? (
                <button type="button" className="update-banner__button" disabled>
                  {t("settings.installing")}
                </button>
              ) : updateFailed ? (
                <button
                  type="button"
                  className="update-banner__button"
                  onClick={onDownloadUpdate}
                >
                  {t("settings.retry")}
                </button>
              ) : updateInfo ? (
                <button
                  type="button"
                  className="update-banner__button"
                  onClick={onDownloadUpdate}
                >
                  {t("settings.downloadUpdate")}
                </button>
              ) : undefined
            }
          />
        </SettingsCard>
      </section>
      {/* The scheme is the app's only externally reachable surface, and it is
          silent by design — this row is where a user can find out it exists
          and copy the one form worth copying. */}
      <DeepLinkRow t={t} onCopied={onCopiedLink} />
    </div>
  );
}
