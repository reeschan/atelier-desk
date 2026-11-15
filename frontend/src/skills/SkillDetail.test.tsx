import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import type { SkillView } from "../api/types";
import { skills as skillsText } from "../i18n/ja/skills";
import { SkillDetail } from "./SkillDetail";
import { createFakeClient, renderWithClient } from "../test/fakeClient";

function view(over: Record<string, unknown> = {}, usedBy: unknown[] = []): SkillView {
  const detail = {
    Skill: {
      Key: "user/demo",
      Name: "demo",
      Description: "A demo skill",
      Origin: "user",
      Plugin: "",
      PluginVersion: "",
      RelPath: "demo",
      Dir: "/skills/demo",
      SkillFile: "/skills/demo/SKILL.md",
      DisableModelInvocation: false,
      Versioned: false,
    },
    Frontmatter: { name: "demo", description: "A demo skill" },
    Body: "Body text",
    Files: [],
    ...over,
  };
  return { detail, usedBy } as unknown as SkillView;
}

beforeEach(() => {
  window.location.hash = "";
});

describe("SkillDetail", () => {
  it("MarkdownTableAndFrontmatterTable", async () => {
    const body = "| Col A | Col B |\n| --- | --- |\n| cell-1 | cell-2 |\n";
    const client = createFakeClient();
    client.skill.mockResolvedValue(view({ Body: body }));
    renderWithClient(<SkillDetail skillKey="user/demo" />, client);

    const article = await screen.findByRole("article", { name: skillsText.bodyTitle });
    const bodyTable = within(article).getByRole("table");
    expect(within(bodyTable).getByRole("columnheader", { name: "Col A" })).toBeInTheDocument();
    expect(within(bodyTable).getByRole("cell", { name: "cell-2" })).toBeInTheDocument();

    const fmSection = screen.getByRole("region", { name: skillsText.frontmatterTitle });
    const fmTable = within(fmSection).getByRole("table");
    expect(fmTable).not.toBe(bodyTable);
    expect(within(fmTable).getByRole("rowheader", { name: "name" })).toBeInTheDocument();
    expect(within(fmTable).queryByText("cell-1")).not.toBeInTheDocument();
    expect(within(bodyTable).queryByText("name")).not.toBeInTheDocument();
  });

  it("AttachedFileShowsContent", async () => {
    const client = createFakeClient();
    client.skill.mockResolvedValue(view({ Files: [{ RelPath: "refs/notes.md", Size: 42 }] }));
    client.skillFile.mockResolvedValue("hello from the attached file");
    renderWithClient(<SkillDetail skillKey="user/demo" />, client);

    fireEvent.click(await screen.findByRole("button", { name: /refs\/notes\.md/ }));

    expect(await screen.findByText("hello from the attached file")).toBeInTheDocument();
    expect(client.skillFile).toHaveBeenCalledWith("user/demo", "refs/notes.md");
  });

  it("VersionedSkillShowsActiveAndChangelog", async () => {
    const client = createFakeClient();
    client.skill.mockResolvedValue(
      view({
        Versions: {
          Active: "v2",
          Versions: ["v1", "v2"],
          Changelog: [
            { Heading: "## v1", At: "2026-01-01", Version: "v1", Kind: "added", Body: "- first release entry" },
            { Heading: "## v2", At: "2026-02-01", Version: "v2", Kind: "changed", Body: "- second release entry" },
          ],
        },
      }),
    );
    renderWithClient(<SkillDetail skillKey="user/demo" />, client);

    const versions = await screen.findByRole("list", { name: skillsText.versionsListLabel });
    const items = within(versions).getAllByRole("listitem");
    const active = items.filter((li) => within(li).queryByText(skillsText.versionActive) !== null);
    expect(active).toHaveLength(1);
    expect(active[0]).toHaveTextContent("v2");

    expect(screen.getByText(skillsText.changelogTitle)).toBeInTheDocument();
    expect(screen.getByText("first release entry")).toBeInTheDocument();
    expect(screen.getByText("second release entry")).toBeInTheDocument();
  });

  it("UsedByLinksToFlow", async () => {
    const client = createFakeClient();
    client.skill.mockResolvedValue(view({}, [{ componentId: "comp-1", workflows: ["wf-a"] }]));
    renderWithClient(<SkillDetail skillKey="user/demo" />, client);

    const section = await screen.findByRole("region", { name: skillsText.usedByTitle });
    fireEvent.click(within(section).getByRole("button", { name: "comp-1" }));

    await waitFor(() => expect(window.location.hash).toBe("#/flow/wf-a?component=comp-1"));
  });
});
