import { useState } from "react";
import { t } from "../i18n";
import { CaseTable } from "../runs/CaseTable";
import { RepoRegistry } from "../runs/RepoRegistry";
import { RunTable } from "../runs/RunTable";

/**
 * The run records: the registered repositories on top, the runs and the cases
 * below. Registering or removing a repository reloads both lists.
 */
export function RunsScreen() {
  const [reloadKey, setReloadKey] = useState(0);
  return (
    <div className="flex flex-col gap-5 px-6 py-6">
      <div className="flex flex-col gap-1">
        <h1 className="font-heading text-3xl">{t("runs", "title")}</h1>
        <p className="text-sm text-muted-foreground">{t("runs", "lead")}</p>
      </div>
      <RepoRegistry onChange={() => setReloadKey((k) => k + 1)} />
      <RunTable reloadKey={reloadKey} />
      <CaseTable reloadKey={reloadKey} />
    </div>
  );
}
