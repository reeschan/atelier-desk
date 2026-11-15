import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Skill } from "../api/types";
import { skills as skillsText } from "../i18n/ja/skills";
import { SkillList } from "./SkillList";
import { renderWithClient } from "../test/fakeClient";

function skill(name: string, description: string): Skill {
  return {
    Key: `user/${name}`,
    Name: name,
    Description: description,
    Origin: "user",
    Plugin: "",
    PluginVersion: "",
    RelPath: name,
    Dir: `/skills/${name}`,
    SkillFile: `/skills/${name}/SKILL.md`,
    DisableModelInvocation: false,
    Versioned: false,
  } as Skill;
}

describe("SkillList", () => {
  it("FilterNarrowsByDescription", () => {
    const list = [
      skill("alpha", "Reviews pull requests"),
      skill("beta", "Writes release notes"),
      skill("gamma", "Plans the sprint"),
    ];
    renderWithClient(<SkillList skills={list} onSelect={vi.fn()} />);
    expect(screen.getAllByRole("button")).toHaveLength(3);

    fireEvent.change(screen.getByRole("searchbox", { name: skillsText.filterLabel }), {
      target: { value: "release" },
    });

    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(1);
    expect(buttons[0]).toHaveTextContent("beta");
    expect(screen.queryByText("alpha")).not.toBeInTheDocument();
    expect(screen.queryByText("gamma")).not.toBeInTheDocument();
  });
});
