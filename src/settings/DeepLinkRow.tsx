import { useEffect, useState } from "react";
import { Check, Copy } from "lucide-react";
import type { Translate } from "../i18n";
import { DEEP_LINK_EXAMPLE, DEEP_LINK_REGISTER_EXAMPLE } from "../deep-link";
import { SettingsAction, SettingsCard, SettingsRow } from "./SettingsRows";

type DeepLinkRowProps = {
  t: Translate;
  /** Notified with the confirmation string once the copy lands, so the
   *  feedback rides the app's one toast stack rather than a local one. */
  onCopied: (message: string) => void;
};

/** R7-9b · the deep-link line.
 *
 *  A URL scheme is invisible when it works and invisible when it does not, so
 *  the About page is the one place a user can find out that Floter answers
 *  `floter://` links at all. The card shows the two forms worth copying —
 *  `connect` (a manifest path) and `register` (a tool already on `PATH`, the
 *  R8-3 form) — and a copy button per line that reuses the existing
 *  icon-button and copy idioms from the integrations panel.
 *
 *  The copy failure path is deliberately quiet: a browser that refuses the
 *  clipboard API (an insecure context, a denied permission) leaves the button
 *  in its resting state and says nothing, because there is nothing the user
 *  could do about it from here and a red toast would be noise about chrome. */
export function DeepLinkRow({ t, onCopied }: DeepLinkRowProps) {
  const [copied, setCopied] = useState<string | null>(null);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(null), 1600);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(value);
      onCopied(t("settings.deepLinkCopied"));
    } catch {
      setCopied(null);
    }
  };

  const copyControl = (value: string) => (
    <SettingsAction
      onClick={() => void copy(value)}
      title={t("settings.deepLinkCopy")}
    >
      {copied === value
        ? <Check size={13} strokeWidth={2} aria-hidden="true" />
        : <Copy size={13} strokeWidth={2} aria-hidden="true" />}
      <span>{copied === value ? t("settings.deepLinkCopied") : t("settings.deepLinkCopy")}</span>
    </SettingsAction>
  );

  return (
    <section className="settings-section">
      <div className="settings-section__heading">
        <div className="settings-section__heading-main">
          <h2 className="settings-section__label">{t("settings.deepLinkTitle")}</h2>
          <p className="settings-section__hint settings-section__hint--inline">{t("settings.deepLinkHint")}</p>
        </div>
      </div>
      <SettingsCard label={t("settings.deepLinkTitle")}>
        <SettingsRow
          label={<code className="settings-deep-link__value">{DEEP_LINK_EXAMPLE}</code>}
          control={copyControl(DEEP_LINK_EXAMPLE)}
        />
        {/* R8-3 · the register form. A tool that is already on `PATH` needs no
            manifest at all, and this is the one-line spelling of that — the
            link highlights the tool on the integrations page and stops there;
            connecting it is still the user's own press. */}
        <SettingsRow
          label={<code className="settings-deep-link__value">{DEEP_LINK_REGISTER_EXAMPLE}</code>}
          control={copyControl(DEEP_LINK_REGISTER_EXAMPLE)}
        />
      </SettingsCard>
    </section>
  );
}
