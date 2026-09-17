import { useEffect, useState } from "react";
import { Check, Copy } from "lucide-react";
import type { Translate } from "../i18n";
import { DEEP_LINK_EXAMPLE } from "../deep-link";

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
 *  `floter://` links at all. The row shows the *connect* form — the only one
 *  with a parameter worth copying — and a copy button that reuses the existing
 *  icon-button and copy idioms from the integrations panel.
 *
 *  The copy failure path is deliberately quiet: a browser that refuses the
 *  clipboard API (an insecure context, a denied permission) leaves the button
 *  in its resting state and says nothing, because there is nothing the user
 *  could do about it from here and a red toast would be noise about chrome. */
export function DeepLinkRow({ t, onCopied }: DeepLinkRowProps) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1600);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(DEEP_LINK_EXAMPLE);
      setCopied(true);
      onCopied(t("settings.deepLinkCopied"));
    } catch {
      setCopied(false);
    }
  };

  return (
    <section className="settings-section">
      <div className="settings-section__heading">
        <h3 className="settings-section__label">{t("settings.deepLinkTitle")}</h3>
        <button
          type="button"
          className="settings-copy-button"
          onClick={() => void copy()}
          aria-label={t("settings.deepLinkCopy")}
          title={t("settings.deepLinkCopy")}
        >
          {copied
            ? <Check size={13} strokeWidth={2} aria-hidden="true" />
            : <Copy size={13} strokeWidth={2} aria-hidden="true" />}
        </button>
      </div>
      <p className="settings-section__hint">{t("settings.deepLinkHint")}</p>
      <code className="settings-deep-link__value">{DEEP_LINK_EXAMPLE}</code>
    </section>
  );
}
