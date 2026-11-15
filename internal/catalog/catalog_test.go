package catalog

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/reeschan/atelier-desk/internal/atelier"
	"github.com/reeschan/atelier-desk/internal/skills"
)

// fakeClient is an atelier.Client that returns fixed values.
type fakeClient struct {
	comps   []atelier.Component
	compErr error
	wfs     []atelier.Workflow
	wfErr   error
}

func (f *fakeClient) Probe(context.Context) (atelier.ProbeResult, error) {
	return atelier.ProbeResult{}, nil
}

func (f *fakeClient) Components(context.Context) ([]atelier.Component, error) {
	return f.comps, f.compErr
}

func (f *fakeClient) Workflows(context.Context) ([]atelier.Workflow, error) {
	return f.wfs, f.wfErr
}

func (f *fakeClient) State(context.Context) (atelier.State, error) {
	return atelier.State{}, nil
}

func (f *fakeClient) Validate(context.Context, atelier.Request) (atelier.Validation, error) {
	return atelier.Validation{}, nil
}

func (f *fakeClient) Plan(context.Context, atelier.Request) (atelier.PlanResult, error) {
	return atelier.PlanResult{}, nil
}

var _ atelier.Client = (*fakeClient)(nil)

// fixedSkills is the skill list every test ties against.
func fixedSkills() []skills.Skill {
	return []skills.Skill{
		{
			Key: "user:review", Name: "review", Origin: skills.OriginUser,
			Dir: "/home/u/.claude/skills/review", RelPath: "review/SKILL.md",
		},
		{
			Key: "user:other", Name: "other", Origin: skills.OriginUser,
			Dir: "/home/u/.claude/skills/other", RelPath: "other/SKILL.md",
		},
		{
			Key: "plugin:pstack:principle", Name: "principle", Origin: skills.OriginPlugin,
			Plugin: "pstack", RelPath: "skills/principle/SKILL.md",
		},
		{
			Key: "plugin:pstack:docs", Name: "docs", Origin: skills.OriginPlugin,
			Plugin: "pstack", RelPath: "skills/docs/SKILL.md",
		},
	}
}

func comp(id string, d atelier.ComponentData) atelier.Component {
	d.ID = id
	return atelier.Component{ID: id, Source: "bundled", Data: d}
}

func skillInvoke(name string) atelier.Invoke {
	return atelier.Invoke{Kind: "skill", Name: name}
}

func build(t *testing.T, comps []atelier.Component, wfs []atelier.Workflow) *Catalog {
	t.Helper()
	cat, err := Build(context.Background(), &fakeClient{comps: comps, wfs: wfs}, fixedSkills())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return cat
}

func view(t *testing.T, cat *Catalog, id string) ComponentView {
	t.Helper()
	v, ok := cat.Component(id)
	if !ok {
		t.Fatalf("component %q not found", id)
	}
	return v
}

func TestSkillKeysAndMissing(t *testing.T) {
	comps := []atelier.Component{
		comp("c.user", atelier.ComponentData{Invoke: skillInvoke("review")}),
		comp("c.plugin", atelier.ComponentData{Invoke: skillInvoke("pstack:principle")}),
		comp("c.read", atelier.ComponentData{Invoke: atelier.Invoke{
			Kind: "read-skill", Plugin: "pstack", Path: "skills/docs/SKILL.md",
		}}),
		comp("c.missing-user", atelier.ComponentData{Invoke: skillInvoke("nosuch")}),
		comp("c.missing-plugin", atelier.ComponentData{Invoke: skillInvoke("pstack:nosuch")}),
		comp("c.missing-read", atelier.ComponentData{Invoke: atelier.Invoke{
			Kind: "read-skill", Plugin: "pstack", Path: "skills/gone/SKILL.md",
		}}),
	}
	cat := build(t, comps, nil)

	tests := []struct {
		id      string
		key     string
		link    LinkKind
		missing bool
	}{
		{"c.user", "user:review", LinkSkill, false},
		{"c.plugin", "plugin:pstack:principle", LinkSkill, false},
		{"c.read", "plugin:pstack:docs", LinkSkill, false},
		{"c.missing-user", "", LinkMissing, true},
		{"c.missing-plugin", "", LinkMissing, true},
		{"c.missing-read", "", LinkMissing, true},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			v := view(t, cat, tc.id)
			if v.SkillKey != tc.key {
				t.Errorf("SkillKey = %q, want %q", v.SkillKey, tc.key)
			}
			if v.Link != tc.link {
				t.Errorf("Link = %q, want %q", v.Link, tc.link)
			}
			if v.SkillMissing != tc.missing {
				t.Errorf("SkillMissing = %v, want %v", v.SkillMissing, tc.missing)
			}
		})
	}
}

func TestLinkKinds(t *testing.T) {
	comps := []atelier.Component{
		// built-in wins over a skill of the same name.
		comp("c.builtin", atelier.ComponentData{
			Invoke: skillInvoke("review"), Link: "built-in",
		}),
		comp("c.script", atelier.ComponentData{
			Invoke: atelier.Invoke{Kind: "script", Name: "review"},
		}),
		comp("c.builtin-kind", atelier.ComponentData{
			Invoke: atelier.Invoke{Kind: "builtin", Name: "review"},
		}),
		comp("c.unknown", atelier.ComponentData{
			Invoke: atelier.Invoke{Kind: "teleport", Name: "review"},
		}),
		// invoke cannot be tied; the second requires.skills entry can.
		comp("c.requires", atelier.ComponentData{
			Invoke:   skillInvoke("nosuch"),
			Requires: &atelier.Requires{Skills: []string{"alsonosuch", "other"}},
		}),
		// invoke cannot be tied and requires gives nothing either.
		comp("c.requires-none", atelier.ComponentData{
			Invoke:   skillInvoke("nosuch"),
			Requires: &atelier.Requires{Skills: []string{"alsonosuch"}},
		}),
	}
	cat := build(t, comps, nil)

	tests := []struct {
		id      string
		key     string
		link    LinkKind
		missing bool
	}{
		{"c.builtin", "", LinkBuiltIn, false},
		{"c.script", "", LinkNone, false},
		{"c.builtin-kind", "", LinkNone, false},
		{"c.unknown", "", LinkNone, false},
		{"c.requires", "user:other", LinkSkill, false},
		{"c.requires-none", "", LinkMissing, true},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			v := view(t, cat, tc.id)
			if v.Link != tc.link {
				t.Errorf("Link = %q, want %q", v.Link, tc.link)
			}
			if v.SkillKey != tc.key {
				t.Errorf("SkillKey = %q, want %q", v.SkillKey, tc.key)
			}
			if v.SkillMissing != tc.missing {
				t.Errorf("SkillMissing = %v, want %v", v.SkillMissing, tc.missing)
			}
		})
	}
}

func TestUsedBy(t *testing.T) {
	comps := []atelier.Component{
		comp("b.second", atelier.ComponentData{Invoke: skillInvoke("review")}),
		comp("a.first", atelier.ComponentData{Invoke: skillInvoke("review")}),
		comp("c.lonely", atelier.ComponentData{Invoke: skillInvoke("review")}),
		comp("d.plugin", atelier.ComponentData{Invoke: skillInvoke("pstack:docs")}),
		comp("e.script", atelier.ComponentData{Invoke: atelier.Invoke{Kind: "script", Name: "review"}}),
		comp("f.missing", atelier.ComponentData{Invoke: skillInvoke("nosuch")}),
	}
	entries := func(ids ...string) []atelier.WorkflowEntry {
		out := make([]atelier.WorkflowEntry, len(ids))
		for i, id := range ids {
			out[i] = atelier.WorkflowEntry{ID: id}
		}
		return out
	}
	wfs := []atelier.Workflow{
		{Name: "zeta", Steps: []atelier.WorkflowStep{
			{Kind: atelier.StepStage, Stage: "build", Components: entries("a.first", "d.plugin")},
		}},
		{Name: "alpha", Steps: []atelier.WorkflowStep{
			{Kind: atelier.StepStage, Stage: "build", Components: entries("a.first", "b.second")},
			{Kind: atelier.StepGate, Stage: "gate", Components: entries("a.first")},
		}},
	}
	cat := build(t, comps, wfs)

	got := cat.UsedBy("user:review")
	want := []Usage{
		{ComponentID: "a.first", Workflows: []string{"alpha", "zeta"}},
		{ComponentID: "b.second", Workflows: []string{"alpha"}},
		{ComponentID: "c.lonely", Workflows: []string{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UsedBy(user:review) = %+v, want %+v", got, want)
	}

	got = cat.UsedBy("plugin:pstack:docs")
	want = []Usage{{ComponentID: "d.plugin", Workflows: []string{"zeta"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UsedBy(plugin:pstack:docs) = %+v, want %+v", got, want)
	}

	// A skill nothing calls: an empty list, not nil.
	got = cat.UsedBy("user:other")
	if got == nil || len(got) != 0 {
		t.Errorf("UsedBy(user:other) = %#v, want empty non-nil", got)
	}

	// The component side shows the same workflows.
	if v := view(t, cat, "a.first"); !reflect.DeepEqual(v.Workflows, []string{"alpha", "zeta"}) {
		t.Errorf("a.first Workflows = %v, want [alpha zeta]", v.Workflows)
	}
}

func TestBuildPropagatesErrBroken(t *testing.T) {
	broken := &atelier.Error{Code: atelier.ErrBroken, ExitCode: 3, Diagnostic: "boom"}

	t.Run("workflows", func(t *testing.T) {
		fc := &fakeClient{
			comps: []atelier.Component{comp("a", atelier.ComponentData{Invoke: skillInvoke("review")})},
			wfErr: broken,
		}
		cat, err := Build(context.Background(), fc, fixedSkills())
		if err == nil {
			t.Fatal("Build returned no error")
		}
		if cat != nil {
			t.Errorf("Build returned a catalog with an error: %+v", cat)
		}
		var ae *atelier.Error
		if !errors.As(err, &ae) || ae.Code != atelier.ErrBroken {
			t.Errorf("err = %v, want atelier.ErrBroken", err)
		}
	})

	t.Run("components", func(t *testing.T) {
		fc := &fakeClient{compErr: broken}
		_, err := Build(context.Background(), fc, fixedSkills())
		var ae *atelier.Error
		if !errors.As(err, &ae) || ae.Code != atelier.ErrBroken {
			t.Errorf("err = %v, want atelier.ErrBroken", err)
		}
	})
}
