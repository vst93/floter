import type { Translate } from "../i18n";
import { groupPermissions } from "./permission-tiers";

// R7-8 · one rendering of the two permission tiers, shared by every review
// surface (recommended/manifest connect, local install, custom integration).
// The shape difference is the message:
//   * enforced — a neutral raised pane with the accent keyline (a *check* the
//     host owns);
//   * disclosure — the plain neutral pane, no accent (an FYI, not a promise).

export type PermissionEntry = {
  permission: string;
  title: string;
  description?: string;
};

const PermissionGroup = ({
  tier,
  labelKey,
  hintKey,
  entries,
  t,
}: {
  tier: "enforced" | "disclosure";
  labelKey: Parameters<Translate>[0];
  hintKey: Parameters<Translate>[0];
  entries: PermissionEntry[];
  t: Translate;
}) => (
  <section className={`extension-permission-tier extension-permission-tier--${tier}`}>
    <span className="extension-permission-tier__label">{t(labelKey)}</span>
    <p className="extension-permission-tier__hint">{t(hintKey)}</p>
    <div className="extension-permission-tier__items">
      {entries.map((entry) => (
        <div key={entry.permission} className="extension-permission-item">
          <strong>{entry.title}</strong>
          {entry.description ? <span>{entry.description}</span> : null}
        </div>
      ))}
    </div>
  </section>
);

export function PermissionTierList({
  permissions,
  t,
  enforcedLabelKey = "settings.extensions.permissionTierEnforced",
  disclosureLabelKey = "settings.extensions.permissionTierDisclosure",
  enforcedHintKey = "settings.extensions.permissionTierEnforcedHint",
  disclosureHintKey = "settings.extensions.permissionTierDisclosureHint",
}: {
  permissions: readonly PermissionEntry[];
  t: Translate;
  enforcedLabelKey?: Parameters<Translate>[0];
  disclosureLabelKey?: Parameters<Translate>[0];
  enforcedHintKey?: Parameters<Translate>[0];
  disclosureHintKey?: Parameters<Translate>[0];
}) {
  const { enforced, disclosure } = groupPermissions(permissions);
  return (
    <>
      {enforced.length > 0 && (
        <PermissionGroup
          tier="enforced"
          labelKey={enforcedLabelKey}
          hintKey={enforcedHintKey}
          entries={enforced}
          t={t}
        />
      )}
      {disclosure.length > 0 && (
        <PermissionGroup
          tier="disclosure"
          labelKey={disclosureLabelKey}
          hintKey={disclosureHintKey}
          entries={disclosure}
          t={t}
        />
      )}
    </>
  );
}
