// Package catalog joins Atelier's components and workflows with the skills on
// disk: component -> the skill it calls, skill -> the components that call it
// -> the workflows that hold them. It holds no Atelier rule; it only ties the
// values it is given.
package catalog

import (
	"context"
	"sort"

	"github.com/reeschan/atelier-desk/internal/atelier"
	"github.com/reeschan/atelier-desk/internal/skills"
)

// LinkKind tells how a component relates to a skill.
type LinkKind string

const (
	// LinkNone: the component calls no skill (script, builtin).
	LinkNone LinkKind = "none"
	// LinkSkill: the component calls a skill that is on hand.
	LinkSkill LinkKind = "skill"
	// LinkMissing: the component calls a skill that is not installed.
	LinkMissing LinkKind = "missing"
	// LinkBuiltIn: a Claude Code built-in (no SKILL.md file).
	LinkBuiltIn LinkKind = "built_in"
)

// ComponentView is a component with the skill it calls. SkillKey is a
// skills.Skill.Key (empty unless Link is LinkSkill). SkillMissing is
// Link == LinkMissing. Workflows are the names of the workflows that hold it.
type ComponentView struct {
	Component    atelier.Component `json:"component"`
	SkillKey     string            `json:"skillKey"`
	Link         LinkKind          `json:"link"`
	SkillMissing bool              `json:"skillMissing"`
	Workflows    []string          `json:"workflows"`
}

// WorkflowView is a workflow as the screen reads it.
type WorkflowView struct {
	Workflow atelier.Workflow `json:"workflow"`
}

// Usage is a component that calls a skill, with the workflows that hold it.
type Usage struct {
	ComponentID string   `json:"componentId"`
	Workflows   []string `json:"workflows"`
}

// Catalog is the joined view. It is read-only after Build.
type Catalog struct {
	components []ComponentView
	byID       map[string]int
	workflows  []WorkflowView
	byName     map[string]int
	bySkill    map[string][]Usage
}

// Build asks the client for components and workflows and ties them to sk. An
// error from either call is returned as it is.
func Build(ctx context.Context, c atelier.Client, sk []skills.Skill) (*Catalog, error) {
	comps, err := c.Components(ctx)
	if err != nil {
		return nil, err
	}
	wfs, err := c.Workflows(ctx)
	if err != nil {
		return nil, err
	}

	wfs = append([]atelier.Workflow(nil), wfs...)
	sort.SliceStable(wfs, func(i, j int) bool { return wfs[i].Name < wfs[j].Name })
	comps = append([]atelier.Component(nil), comps...)
	sort.SliceStable(comps, func(i, j int) bool { return comps[i].ID < comps[j].ID })

	inWorkflows := workflowIndex(wfs)
	cat := &Catalog{
		byID:    map[string]int{},
		byName:  map[string]int{},
		bySkill: map[string][]Usage{},
	}
	for _, comp := range comps {
		key, link := SkillKeyFor(comp.Data, sk)
		names := append([]string{}, inWorkflows[comp.ID]...)
		if _, dup := cat.byID[comp.ID]; !dup {
			cat.byID[comp.ID] = len(cat.components)
		}
		cat.components = append(cat.components, ComponentView{
			Component:    comp,
			SkillKey:     key,
			Link:         link,
			SkillMissing: link == LinkMissing,
			Workflows:    names,
		})
		if link == LinkSkill && key != "" {
			cat.bySkill[key] = append(cat.bySkill[key], Usage{ComponentID: comp.ID, Workflows: append([]string{}, names...)})
		}
	}
	for _, w := range wfs {
		if _, dup := cat.byName[w.Name]; !dup {
			cat.byName[w.Name] = len(cat.workflows)
		}
		cat.workflows = append(cat.workflows, WorkflowView{Workflow: w})
	}
	return cat, nil
}

// Components returns every component, id ascending.
func (c *Catalog) Components() []ComponentView {
	return append([]ComponentView{}, c.components...)
}

// Component returns the component with the id.
func (c *Catalog) Component(id string) (ComponentView, bool) {
	i, ok := c.byID[id]
	if !ok {
		return ComponentView{}, false
	}
	return c.components[i], true
}

// Workflows returns every workflow, name ascending.
func (c *Catalog) Workflows() []WorkflowView {
	return append([]WorkflowView{}, c.workflows...)
}

// Workflow returns the workflow with the name.
func (c *Catalog) Workflow(name string) (WorkflowView, bool) {
	i, ok := c.byName[name]
	if !ok {
		return WorkflowView{}, false
	}
	return c.workflows[i], true
}

// UsedBy returns the components that call the skill (a skills.Skill.Key), id
// ascending, each with the workflows that hold it. It is empty (not nil) when
// nothing calls the skill.
func (c *Catalog) UsedBy(skillKey string) []Usage {
	us := c.bySkill[skillKey]
	out := make([]Usage, len(us))
	for i, u := range us {
		out[i] = Usage{ComponentID: u.ComponentID, Workflows: append([]string{}, u.Workflows...)}
	}
	return out
}
