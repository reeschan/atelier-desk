import { useEffect, useState } from "react";
import { toDeskError, useClient } from "../api/client";
import type { Skill } from "../api/types";
import { errorText, t } from "../i18n";
import { navigate, useRoute } from "../shell/routes";
import { SkillDetail } from "../skills/SkillDetail";
import { SkillList } from "../skills/SkillList";

type Loaded =
  | { status: "loading" }
  | { status: "error"; code: string }
  | { status: "ok"; skills: Skill[] };

export function SkillsScreen() {
  const client = useClient();
  const route = useRoute();
  const skillKey = route.screen === "skills" ? route.skillKey : undefined;
  const [state, setState] = useState<Loaded>({ status: "loading" });

  useEffect(() => {
    let cancelled = false;
    client
      .skills()
      .then((skills) => {
        if (!cancelled) setState({ status: "ok", skills: skills ?? [] });
      })
      .catch((e: unknown) => {
        if (!cancelled) setState({ status: "error", code: toDeskError(e).code });
      });
    return () => {
      cancelled = true;
    };
  }, [client]);

  if (state.status === "loading") {
    return (
      <p className="p-6 text-sm text-muted-foreground" role="status">
        {t("skills", "loading")}
      </p>
    );
  }
  if (state.status === "error") {
    return (
      <div className="p-6 text-sm" role="alert">
        <p>{t("skills", "loadFailed")}</p>
        <p className="mt-1 text-muted-foreground">{errorText(state.code)}</p>
      </div>
    );
  }
  return (
    <div className="flex h-full">
      <SkillList
        skills={state.skills}
        selectedKey={skillKey}
        onSelect={(key) => navigate({ screen: "skills", skillKey: key })}
      />
      <section className="min-w-0 flex-1 overflow-y-auto" aria-label={t("skills", "detailLabel")}>
        {skillKey === undefined ? (
          <p className="p-8 text-sm text-muted-foreground">{t("skills", "noSelection")}</p>
        ) : (
          <SkillDetail key={skillKey} skillKey={skillKey} />
        )}
      </section>
    </div>
  );
}
