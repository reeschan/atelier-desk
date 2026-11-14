package catalog

import (
	"path"
	"sort"
	"strings"

	"github.com/reeschan/atelier-desk/internal/atelier"
	"github.com/reeschan/atelier-desk/internal/skills"
)

// SkillKeyFor ties one component to the skill it calls.
//
// script and builtin components call no skill (LinkNone). A component whose
// link is "built-in" is a Claude Code built-in with no SKILL.md file
// (LinkBuiltIn, key empty). For skill and read-skill the invoke is tried first;
// only when it cannot be tied are requires.skills (in order) and then source
// tried. When nothing ties, the link is LinkMissing and the key is empty.
func SkillKeyFor(d atelier.ComponentData, sk []skills.Skill) (key string, link LinkKind) {
	kind := d.Invoke.Kind
	if kind != "skill" && kind != "read-skill" {
		return "", LinkNone
	}
	if d.Link == "built-in" {
		return "", LinkBuiltIn
	}

	var k string
	if kind == "skill" {
		k = byName(d.Invoke.Name, sk)
	} else {
		k = byPath(d.Invoke.Plugin, d.Invoke.Path, sk)
	}
	if k != "" {
		return k, LinkSkill
	}

	if d.Requires != nil {
		for _, name := range d.Requires.Skills {
			if k := byName(name, sk); k != "" {
				return k, LinkSkill
			}
		}
	}
	if k := byName(d.Source, sk); k != "" {
		return k, LinkSkill
	}
	return "", LinkMissing
}

// byName resolves a skill name as written in a component: "<name>" is a user
// skill (its directory name first, then its frontmatter name) and
// "<plugin>:<skill>" a plugin skill (skill is the frontmatter name). It returns
// the skill's Key, or "" when there is no such skill.
func byName(name string, sk []skills.Skill) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if plugin, skill, ok := strings.Cut(name, ":"); ok {
		if plugin == "" || skill == "" {
			return ""
		}
		want := "plugin:" + plugin + ":" + skill
		for _, s := range sk {
			if s.Key == want {
				return s.Key
			}
		}
		for _, s := range sk {
			if isPlugin(s) && pluginOf(s) == plugin && s.Name == skill {
				return s.Key
			}
		}
		return ""
	}
	for _, s := range sk {
		if isUser(s) && s.Dir != "" && dirName(s.Dir) == name {
			return s.Key
		}
	}
	for _, s := range sk {
		if isUser(s) && s.Name == name {
			return s.Key
		}
	}
	return ""
}

// byPath resolves a read-skill invoke: the skill of the plugin whose RelPath
// is the invoke path.
func byPath(plugin, p string, sk []skills.Skill) string {
	if plugin == "" || p == "" {
		return ""
	}
	want := cleanRel(p)
	for _, s := range sk {
		if isPlugin(s) && pluginOf(s) == plugin && cleanRel(s.RelPath) == want {
			return s.Key
		}
	}
	return ""
}

func isUser(s skills.Skill) bool {
	return s.Origin == skills.OriginUser || (s.Origin == "" && strings.HasPrefix(s.Key, "user:"))
}

func isPlugin(s skills.Skill) bool {
	return s.Origin == skills.OriginPlugin || (s.Origin == "" && strings.HasPrefix(s.Key, "plugin:"))
}

// pluginOf is the plugin name of a plugin skill (from Plugin, else from Key).
func pluginOf(s skills.Skill) string {
	if s.Plugin != "" {
		return s.Plugin
	}
	rest, ok := strings.CutPrefix(s.Key, "plugin:")
	if !ok {
		return ""
	}
	plugin, _, _ := strings.Cut(rest, ":")
	return plugin
}

// dirName is the last element of a directory path (either separator).
func dirName(dir string) string {
	dir = strings.TrimRight(strings.ReplaceAll(dir, "\\", "/"), "/")
	return path.Base(dir)
}

func cleanRel(p string) string {
	return strings.TrimPrefix(path.Clean(strings.ReplaceAll(p, "\\", "/")), "./")
}

// workflowIndex maps a component id to the names of the workflows that
// contain it (every stage, gate and optional stage), sorted and without
// duplicates.
func workflowIndex(wfs []atelier.Workflow) map[string][]string {
	sets := map[string]map[string]bool{}
	for _, w := range wfs {
		for _, step := range w.Steps {
			for _, e := range step.Components {
				if e.ID == "" {
					continue
				}
				if sets[e.ID] == nil {
					sets[e.ID] = map[string]bool{}
				}
				sets[e.ID][w.Name] = true
			}
		}
	}
	out := make(map[string][]string, len(sets))
	for id, set := range sets {
		names := make([]string, 0, len(set))
		for n := range set {
			names = append(names, n)
		}
		sort.Strings(names)
		out[id] = names
	}
	return out
}
