import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { toDeskError, useClient } from "../api/client";
import { errorText, t } from "../i18n";

/**
 * The repositories whose run records are read: the list from repos(), a text
 * input for an absolute path (addRepo) and a remove button on each row
 * (removeRepo). onChange tells the screen that the set changed.
 */
export function RepoRegistry({ onChange }: { onChange?: (repos: string[]) => void }) {
  const client = useClient();
  const [repos, setRepos] = useState<string[]>([]);
  const [path, setPath] = useState("");
  const [busy, setBusy] = useState<"" | "add" | "remove">("");
  const [failed, setFailed] = useState("");

  useEffect(() => {
    let live = true;
    client
      .repos()
      .then((list) => {
        if (live) setRepos(list ?? []);
      })
      .catch((e: unknown) => {
        if (live) setFailed(errorText(toDeskError(e).code));
      });
    return () => {
      live = false;
    };
  }, [client]);

  function apply(list: string[] | null) {
    const next = list ?? [];
    setRepos(next);
    onChange?.(next);
  }

  async function add(e: FormEvent) {
    e.preventDefault();
    const dir = path.trim();
    if (busy !== "" || dir === "") return;
    setBusy("add");
    setFailed("");
    try {
      apply(await client.addRepo(dir));
      setPath("");
    } catch (err) {
      setFailed(errorText(toDeskError(err).code));
    } finally {
      setBusy("");
    }
  }

  async function remove(dir: string) {
    if (busy !== "") return;
    setBusy("remove");
    setFailed("");
    try {
      apply(await client.removeRepo(dir));
    } catch (err) {
      setFailed(errorText(toDeskError(err).code));
    } finally {
      setBusy("");
    }
  }

  return (
    <section
      aria-label={t("runs", "reposTitle")}
      className="flex flex-col gap-3 rounded-lg border border-foreground/10 bg-card p-4"
    >
      <h2 className="flex items-baseline justify-between text-sm">
        {t("runs", "reposTitle")}
        <span className="font-mono text-xs text-muted-foreground tabular-nums">
          {t("runs", "reposCount", { n: repos.length })}
        </span>
      </h2>
      {repos.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("runs", "reposEmpty")}</p>
      ) : (
        <ul className="divide-y divide-foreground/10">
          {repos.map((dir) => (
            <li key={dir} className="flex items-center gap-3 py-2">
              <span className="min-w-0 flex-1 truncate font-mono text-xs" title={dir}>
                {dir}
              </span>
              <button
                type="button"
                aria-label={t("runs", "repoRemoveLabel", { dir })}
                disabled={busy !== ""}
                onClick={() => void remove(dir)}
                className="desk-tint inline-flex h-7 items-center rounded-md border border-foreground/10 px-2.5 text-xs hover:bg-muted disabled:opacity-50"
              >
                {t("runs", "repoRemove")}
              </button>
            </li>
          ))}
        </ul>
      )}
      <form onSubmit={(e) => void add(e)} className="flex flex-col gap-1.5">
        <label htmlFor="runs-repo-path" className="text-xs text-muted-foreground">
          {t("runs", "repoPathLabel")}
        </label>
        <div className="flex items-center gap-2">
          <input
            id="runs-repo-path"
            type="text"
            value={path}
            onChange={(e) => setPath(e.target.value)}
            placeholder={t("runs", "repoPathPlaceholder")}
            spellCheck={false}
            className="h-8 min-w-0 flex-1 rounded-md border border-foreground/10 bg-background px-2.5 font-mono text-xs"
          />
          <button
            type="submit"
            disabled={busy !== "" || path.trim() === ""}
            className="desk-tint desk-press inline-flex h-8 items-center rounded-md bg-primary px-3 text-sm text-primary-foreground disabled:opacity-50"
          >
            {busy === "add" ? t("runs", "repoAdding") : t("runs", "repoAdd")}
          </button>
        </div>
        <p className="text-[11px] text-muted-foreground">{t("runs", "repoPathNote")}</p>
      </form>
      {failed !== "" && (
        <p role="alert" className="text-xs text-destructive">
          {failed}
        </p>
      )}
    </section>
  );
}
