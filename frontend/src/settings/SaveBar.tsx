import { errorText, t } from "../i18n";
import type { HomeSave } from "./useHomeSave";

export type SaveBarProps = {
  save: Pick<
    HomeSave<unknown>,
    "state" | "problems" | "changed" | "result" | "error" | "save" | "overwrite" | "reload" | "discard"
  >;
  /** True while the connection to Atelier is ready: only then a save can start. */
  ready: boolean;
};

const STATE_TEXT = {
  viewing: "stateViewing",
  editing: "stateEditing",
  checking: "stateChecking",
  writing: "stateWriting",
  rejected: "stateRejected",
  external: "stateExternal",
} as const;

const PRIMARY =
  "desk-press inline-flex h-8 items-center rounded-md bg-primary px-4 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50";
const PLAIN =
  "desk-tint inline-flex h-8 items-center rounded-md border border-foreground/15 bg-background px-3 text-sm hover:bg-muted disabled:opacity-50";

/** What the state of one setting's edit is, what Atelier said, and what can be done now. */
export function SaveBar({ save: s, ready }: SaveBarProps) {
  const { state, result } = s;
  const idle = state === "editing" || state === "rejected";
  const busy = state === "checking" || state === "writing";
  const savedNote = state === "viewing" && result?.Outcome === "saved";
  const partial = result?.Outcome === "partial_failure" ? result : null;

  return (
    <section
      aria-label={t("settings", "barLabel")}
      data-save-state={state}
      className="mt-3 flex flex-col gap-2 border-t border-foreground/10 pt-3"
    >
      <p role="status" className="text-xs text-muted-foreground">
        {t("settings", STATE_TEXT[state])}
      </p>
      {savedNote && <p className="text-xs text-success">{t("settings", "saved")}</p>}
      {state === "rejected" && (
        <div role="alert" className="rounded-md border border-destructive/40 p-3">
          <p className="text-xs text-destructive">{t("settings", "problemsTitle")}</p>
          <ul className="mt-1 flex flex-col gap-1">
            {s.problems.map((p, i) => (
              <li key={i} className="text-sm break-words">
                {p}
              </li>
            ))}
          </ul>
        </div>
      )}
      {partial !== null && (
        <div role="alert" className="rounded-md border border-warning/50 p-3">
          <p className="text-sm text-warning">{t("settings", "partialTitle")}</p>
          <p className="mt-1 text-xs text-muted-foreground">{t("settings", "partialBody")}</p>
          <dl className="mt-2 flex flex-col gap-1 text-xs">
            {partial.Failed !== "" && (
              <div className="flex gap-2">
                <dt className="text-muted-foreground">{t("settings", "partialFailed")}</dt>
                <dd className="font-mono break-all">{partial.Failed}</dd>
              </div>
            )}
            <div className="flex gap-2">
              <dt className="text-muted-foreground">{t("settings", "backupDir")}</dt>
              <dd className="font-mono break-all">{partial.BackupDir}</dd>
            </div>
            <div className="flex gap-2">
              <dt className="text-muted-foreground">{t("settings", "recoveryFile")}</dt>
              <dd className="font-mono break-all">{partial.RecoveryFile}</dd>
            </div>
          </dl>
        </div>
      )}
      {s.error !== "" && (
        <p role="alert" className="text-xs text-destructive">
          {t("settings", "saveFailed")}: {errorText(s.error)}
        </p>
      )}
      {!ready && (state === "editing" || state === "rejected") && (
        <p role="alert" className="text-xs text-warning">
          {errorText("not_ready")}
        </p>
      )}
      {state === "external" && (
        <p className="text-xs text-muted-foreground">{t("settings", "externalNote")}</p>
      )}
      {(idle || busy) && (
        <div className="flex items-center gap-2">
          <button
            type="button"
            className={PRIMARY}
            disabled={!ready || busy || (state === "editing" && !s.changed)}
            onClick={() => void s.save()}
          >
            {t("settings", "save")}
          </button>
          <button type="button" className={PLAIN} disabled={busy} onClick={() => s.discard()}>
            {t("settings", "cancel")}
          </button>
        </div>
      )}
      {state === "external" && (
        <div className="flex items-center gap-2">
          <button type="button" className={PRIMARY} onClick={() => void s.reload()}>
            {t("settings", "reload")}
          </button>
          <button
            type="button"
            className={PLAIN}
            disabled={!ready}
            onClick={() => void s.overwrite()}
          >
            {t("settings", "overwrite")}
          </button>
        </div>
      )}
    </section>
  );
}
