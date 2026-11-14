package runs

import (
	"encoding/json"
	"sort"
)

// SchemaCase is the schema value of a row of cases.jsonl.
const SchemaCase = "atelier.case/1"

// Case is one PBI of one work (one row of cases.jsonl).
type Case struct {
	Repo    string
	WorkKey string
	PBI     string

	// Causes of result.attempts[].cause, as in the file, without duplicates.
	Causes []string

	Summary  string // task.title
	Archived bool
	Raw      json.RawMessage
}

// Cases lists the cases ordered by (repo, work_key, pbi). A case found both in a
// repository and in the archive is kept once, as the repository one. Rows whose
// schema is not SchemaCase are skipped with a warning.
func (r *Reader) Cases() ([]Case, []ReadWarning, error) {
	rows, warns := r.readRows(casesFile)
	seen := map[string]bool{}
	var out []Case
	for _, rw := range rows {
		if s := rw.str("schema"); s != SchemaCase {
			warns = append(warns, ReadWarning{Path: rw.src.path, Line: rw.line, Reason: "unsupported schema: " + s})
			continue
		}
		repo := rw.str("repo")
		if repo == "" {
			repo = rw.src.repoName
		}
		c := Case{
			Repo:     repo,
			WorkKey:  rw.str("work_key"),
			PBI:      rw.str("pbi"),
			Archived: rw.src.archived,
			Raw:      rw.raw,
		}
		k := c.Repo + "\x00" + c.WorkKey + "\x00" + c.PBI
		if seen[k] {
			continue
		}
		seen[k] = true

		if t, ok := rw.obj["task"]; ok {
			var to map[string]json.RawMessage
			if json.Unmarshal(t, &to) == nil {
				c.Summary = objStr(to, "title")
			}
		}
		if res, ok := rw.obj["result"]; ok {
			var ro struct {
				Attempts []struct {
					Cause json.RawMessage `json:"cause"`
				} `json:"attempts"`
			}
			if json.Unmarshal(res, &ro) == nil {
				have := map[string]bool{}
				for _, a := range ro.Attempts {
					for _, cause := range flattenCauses(a.Cause) {
						if !have[cause] {
							have[cause] = true
							c.Causes = append(c.Causes, cause)
						}
					}
				}
			}
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		if out[i].WorkKey != out[j].WorkKey {
			return out[i].WorkKey < out[j].WorkKey
		}
		return out[i].PBI < out[j].PBI
	})
	return out, warns, nil
}

// FilterCases keeps the cases that have at least one of causes. An empty causes
// keeps every case.
func FilterCases(cs []Case, causes []string) []Case {
	if len(causes) == 0 {
		return cs
	}
	want := map[string]bool{}
	for _, c := range causes {
		want[c] = true
	}
	var out []Case
	for _, c := range cs {
		for _, got := range c.Causes {
			if want[got] {
				out = append(out, c)
				break
			}
		}
	}
	return out
}
