package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/reeschan/atelier-desk/internal/paths"
)

// copyTestdata copies internal/skills/testdata into a temporary directory and
// returns a Scanner that reads only that copy.
func copyTestdata(t *testing.T) (*Scanner, paths.Locations) {
	t.Helper()
	src := "testdata"
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatalf("copy testdata: %v", err)
	}
	loc := paths.Locations{
		SkillsDir:       filepath.Join(dst, "skills"),
		PluginsCacheDir: filepath.Join(dst, "plugins"),
	}
	return NewScanner(loc), loc
}

func TestScanKeysNamesOrigins(t *testing.T) {
	s, _ := copyTestdata(t)
	got, warns, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("warnings = %+v, want none", warns)
	}

	type row struct {
		Key, Name, Description string
		Origin                 Origin
		Plugin, RelPath        string
		Versioned              bool
	}
	want := []row{
		{"plugin:toolbox:alpha", "alpha", "Top level plugin skill.", OriginPlugin, "toolbox", "skills/alpha/SKILL.md", false},
		{"plugin:toolbox:beta", "beta", "Nested plugin skill.", OriginPlugin, "toolbox", "skills/group/beta/SKILL.md", false},
		{"user:noname", "noname", "Skill whose name is its directory.", OriginUser, "", "noname/SKILL.md", false},
		{"user:plain-skill", "plain-skill", "A plain skill without versions.", OriginUser, "", "plain/SKILL.md", false},
		{"user:versioned-skill", "versioned-skill", "From v7.10, the active version.", OriginUser, "", "versioned/versions/v7.10/SKILL.md", true},
	}
	var rows []row
	for _, k := range got {
		rows = append(rows, row{k.Key, k.Name, k.Description, k.Origin, k.Plugin, k.RelPath, k.Versioned})
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("skills mismatch\n got: %+v\nwant: %+v", rows, want)
	}
	for _, k := range got {
		if strings.Contains(k.Key, "stray") {
			t.Errorf("stray directory listed as skill: %s", k.Key)
		}
		if k.Origin == OriginPlugin && k.PluginVersion != "1.0.0" {
			t.Errorf("%s PluginVersion = %q, want 1.0.0", k.Key, k.PluginVersion)
		}
	}
}

func TestReadFileRejectsParentPath(t *testing.T) {
	s, _ := copyTestdata(t)

	for _, rel := range []string{"../versioned/CHANGELOG.md", "..", "../../skills/plain/SKILL.md", "references/../../noname/SKILL.md", "..\\noname\\SKILL.md"} {
		text, err := s.ReadFile("user:plain-skill", rel)
		if err == nil {
			t.Errorf("ReadFile(%q) = %q, want an error", rel, text)
			continue
		}
		if CodeOf(err) != ErrInvalidPath {
			t.Errorf("ReadFile(%q) code = %q, want %q", rel, CodeOf(err), ErrInvalidPath)
		}
	}

	// a path inside the skill directory still works
	text, err := s.ReadFile("user:plain-skill", "references/note.md")
	if err != nil {
		t.Fatalf("ReadFile inside the skill: %v", err)
	}
	// ReadFile returns the bytes as they are; the checkout may have turned LF
	// into CRLF (core.autocrlf), so compare the text with line endings unified.
	if strings.ReplaceAll(text, "\r\n", "\n") != "reference note\n" {
		t.Errorf("ReadFile text = %q", text)
	}
}

func TestParseFrontmatterBOMCRLFQuoted(t *testing.T) {
	src := "\xef\xbb\xbf---\r\nname: quoted-skill\r\ndescription: \"Use when: a colon, or # hash appears\"\r\ndisable-model-invocation: true\r\n---\r\nBody line\r\n"
	fm, body, err := ParseFrontmatter([]byte(src))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm["name"] != "quoted-skill" {
		t.Errorf("name = %v", fm["name"])
	}
	if fm["description"] != "Use when: a colon, or # hash appears" {
		t.Errorf("description = %q", fm["description"])
	}
	if fm["disable-model-invocation"] != true {
		t.Errorf("disable-model-invocation = %v, want true", fm["disable-model-invocation"])
	}
	if strings.TrimSpace(body) != "Body line" {
		t.Errorf("body = %q", body)
	}

	// no frontmatter: empty map and the whole text as body
	fm, body, err = ParseFrontmatter([]byte("# just markdown\n"))
	if err != nil || len(fm) != 0 || body != "# just markdown\n" {
		t.Errorf("no frontmatter: fm=%v body=%q err=%v", fm, body, err)
	}
}

// A frontmatter that is not valid YAML (a plain description holding ": ") is
// read line by line, so the skill is still listed with its description.
func TestParseFrontmatterBOMCRLFQuoted_LooseDescriptionStillListed(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	write := func(name, content string) {
		t.Helper()
		d := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("bomcrlf", "\xef\xbb\xbf---\r\nname: bom-crlf\r\ndescription: 'Quoted one'\r\n---\r\nbody\r\n")
	write("loose", "---\nname: loose\ndescription: Use this: when something happens\n---\nbody\n")

	s := NewScanner(paths.Locations{SkillsDir: skillsDir, PluginsCacheDir: filepath.Join(dir, "none")})
	got, warns, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("warnings = %+v, want none", warns)
	}
	desc := map[string]string{}
	for _, k := range got {
		desc[k.Key] = k.Description
	}
	if desc["user:bom-crlf"] != "Quoted one" {
		t.Errorf("bom-crlf description = %q", desc["user:bom-crlf"])
	}
	if desc["user:loose"] != "Use this: when something happens" {
		t.Errorf("loose description = %q", desc["user:loose"])
	}
}

func TestDetailVersionedSkill(t *testing.T) {
	s, _ := copyTestdata(t)
	d, err := s.Detail("user:versioned-skill")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	// name and description come from versions/<ACTIVE>/SKILL.md, not the relay SKILL.md
	if d.Skill.Name != "versioned-skill" {
		t.Errorf("Name = %q", d.Skill.Name)
	}
	if d.Skill.Description != "From v7.10, the active version." {
		t.Errorf("Description = %q", d.Skill.Description)
	}
	if !d.Skill.Versioned {
		t.Errorf("Versioned = false")
	}
	if strings.TrimSpace(d.Body) != "v7.10 body" {
		t.Errorf("Body = %q", d.Body)
	}
	if d.Versions == nil {
		t.Fatal("Versions = nil, want version info")
	}
	if d.Versions.Active != "v7.10" {
		t.Errorf("Active = %q", d.Versions.Active)
	}
	if want := []string{"v7.2", "v7.9", "v7.10"}; !reflect.DeepEqual(d.Versions.Versions, want) {
		t.Errorf("Versions = %v, want %v (numeric order)", d.Versions.Versions, want)
	}
	want := []ChangelogEntry{
		{Heading: "Unreleased", Body: "- pending change"},
		{Heading: "2026-05-02T10:00:00Z v7.10 feat", At: "2026-05-02T10:00:00Z", Version: "v7.10", Kind: "feat", Body: "Added B."},
		{Heading: "2026-04-01T09:00:00Z v7.9 fix", At: "2026-04-01T09:00:00Z", Version: "v7.9", Kind: "fix", Body: "Fixed A."},
	}
	if !reflect.DeepEqual(d.Versions.Changelog, want) {
		t.Errorf("Changelog mismatch\n got: %+v\nwant: %+v", d.Versions.Changelog, want)
	}

	// a skill without an ACTIVE file is not versioned
	p, err := s.Detail("user:plain-skill")
	if err != nil {
		t.Fatalf("Detail plain: %v", err)
	}
	if p.Versions != nil {
		t.Errorf("plain skill Versions = %+v, want nil", p.Versions)
	}
}
