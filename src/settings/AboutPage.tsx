import type { Translate } from "../i18n";

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
};

/** The about settings page: current version plus the updater banner. All
 * state lives in `App` and arrives through props. */
export function AboutPage({
  t,
  appVersion,
  updateInfo,
  updateDownloading,
  updateProgress,
  updateFailed,
  onDownloadUpdate,
}: AboutPageProps) {
  const updatePercent =
    updateProgress && updateProgress.total > 0
      ? Math.min(100, (updateProgress.downloaded / updateProgress.total) * 100)
      : 0;
  return (
    <section className="settings-section">
      <div className="update-banner">
        <div className="update-banner__info">
          <span className="update-banner__title">
            {t("settings.currentVersion")}: v{appVersion}
          </span>
          <span className="update-banner__desc">
            {updateFailed
              ? t("settings.updateFailed")
              : updateInfo
                ? `${t("settings.latestVersion")}: v${updateInfo.version}`
                : t("settings.upToDate")}
          </span>
        </div>
        {updateProgress ? (
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
        ) : null}
      </div>
    </section>
  );
}
