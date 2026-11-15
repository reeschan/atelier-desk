import { useMemo, useState } from "react";
import type { Skill } from "../api/types";
import { t } from "../i18n";

export interface SkillListProps {
  skills: Skill[];
  selectedKey?: string;
  onSelect(key: string): void;
}

/** Name and description contain the text, upper and lower case alike. */
export function matchesFilter(s: Skill, text: string): boolean {
  const q = text.trim().toLowerCase();
  if (q === "") return true;
  return s.Name.toLowerCase().includes(q) || s.Description.toLowerCase().includes(q);
}

function originText(s: Skill): string {
  return s.Origin === "plugin"
    ? t("skills", "originPlugin", { plugin: s.Plugin })
    : t("skills", "originUser");
}

export function SkillList({ skills, selectedKey, onSelect }: SkillListProps) {
  const [filter, setFilter] = useState("");
  const shown = useMemo(() => skills.filter((s) => matchesFilter(s, filter)), [skills, filter]);

  return (
    <section
      className="flex h-full w-72 shrink-0 flex-col gap-3 overflow-y-auto border-r border-foreground/10 p-4"
      aria-label={t("skills", "listLabel")}
    >
      <input
        type="search"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder={t("skills", "filterPlaceholder")}
        aria-label={t("skills", "filterLabel")}
        className="h-9 w-full rounded-md border border-foreground/10 bg-card px-3 text-sm text-foreground outline-none placeholder:text-muted-foreground focus-visible:outline-2 focus-visible:outline-ring"
      />
      <p className="px-1 text-xs text-muted-foreground" aria-live="polite">
        {t("skills", "count", { shown: shown.length, total: skills.length })}
      </p>
      {skills.length === 0 ? (
        <p className="px-1 text-sm text-muted-foreground">{t("skills", "empty")}</p>
      ) : shown.length === 0 ? (
        <p className="px-1 text-sm text-muted-foreground">{t("skills", "noMatch")}</p>
      ) : (
        <ul className="flex flex-col gap-0.5">
          {shown.map((s) => {
            const selected = s.Key === selectedKey;
            return (
              <li key={s.Key}>
                <button
                  type="button"
                  onClick={() => onSelect(s.Key)}
                  aria-current={selected ? "true" : undefined}
                  className={
                    "desk-tint flex w-full flex-col gap-1 rounded-md px-3 py-2 text-left " +
                    (selected ? "bg-accent text-accent-foreground" : "hover:bg-card/60")
                  }
                >
                  <span className="truncate font-mono text-sm">{s.Name}</span>
                  <span className="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
                    <span>{originText(s)}</span>
                    {s.Versioned && (
                      <span className="rounded-sm bg-muted px-1.5">{t("skills", "badgeVersioned")}</span>
                    )}
                    {s.DisableModelInvocation && (
                      <span className="rounded-sm bg-muted px-1.5">{t("skills", "badgeNoAuto")}</span>
                    )}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
