package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/reeschan/atelier-desk/internal/paths"
)

// VersionInfo is what a versioned skill (one with an ACTIVE file) says about its versions.
type VersionInfo struct {
	Active    string
	Versions  []string // numeric order: v7.9 before v7.10
	Changelog []ChangelogEntry
}

// ChangelogEntry is one "## <ISO time> <version> <kind>" block of CHANGELOG.md.
// "## Unreleased" is kept as an entry with only Heading and Body.
type ChangelogEntry struct {
	Heading string
	At      string
	Version string
	Kind    string
	Body    string
}

// ReadVersions reads ACTIVE, versions/ and CHANGELOG.md of skillDir.
// A skill without an ACTIVE file is not versioned: it gives nil, nil.
func ReadVersions(skillDir string) (*VersionInfo, error) {
	b, err := os.ReadFile(filepath.Join(skillDir, "ACTIVE"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	info := &VersionInfo{Active: strings.TrimSpace(string(b))}
	info.Versions, err = paths.ListVersions(skillDir)
	if err != nil {
		return nil, err
	}
	cl, err := os.ReadFile(filepath.Join(skillDir, "CHANGELOG.md"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		info.Changelog = ParseChangelog(string(cl))
	}
	return info, nil
}

// ParseChangelog splits CHANGELOG.md text at its "## " headings, in file order.
// Text before the first heading (the title) is dropped.
func ParseChangelog(text string) []ChangelogEntry {
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var out []ChangelogEntry
	var cur *ChangelogEntry
	var body []string
	flush := func() {
		if cur != nil {
			cur.Body = strings.TrimSpace(strings.Join(body, "\n"))
			out = append(out, *cur)
		}
		body = nil
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			e := ChangelogEntry{Heading: heading}
			if f := strings.Fields(heading); len(f) > 0 && !strings.EqualFold(f[0], "Unreleased") {
				e.At = f[0]
				if len(f) > 1 {
					e.Version = f[1]
				}
				if len(f) > 2 {
					e.Kind = f[2]
				}
			}
			cur = &e
			continue
		}
		if cur != nil {
			body = append(body, line)
		}
	}
	flush()
	return out
}
