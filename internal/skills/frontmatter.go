package skills

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFrontmatter splits a SKILL.md into its YAML frontmatter and its body.
// A leading BOM and CRLF line ends are accepted. The text between the first
// line "---" and the next line "---" is read as YAML. When there is no
// frontmatter, the map is empty and the body is the whole text. A frontmatter
// that is not a YAML mapping is an error.
func ParseFrontmatter(b []byte) (map[string]any, string, error) {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	text := string(b)
	empty := map[string]any{}

	first, rest, ok := cutLine(text)
	if !ok || trimEOL(first) != "---" {
		return empty, text, nil
	}
	var yamlPart []byte
	pos := 0
	for pos <= len(rest) {
		line, after, more := cutLine(rest[pos:])
		if trimEOL(line) == "---" {
			yamlPart = []byte(rest[:pos])
			body := ""
			if more {
				body = after
			}
			return decodeFrontmatter(yamlPart, body)
		}
		if !more {
			break
		}
		pos += len(line)
	}
	// no closing delimiter: not a frontmatter
	return empty, text, nil
}

func decodeFrontmatter(y []byte, body string) (map[string]any, string, error) {
	m := map[string]any{}
	if len(bytes.TrimSpace(y)) == 0 {
		return m, body, nil
	}
	var v any
	if err := yaml.Unmarshal(y, &v); err != nil {
		return nil, "", fmt.Errorf("skills: frontmatter is not valid YAML: %w", err)
	}
	if v == nil {
		return m, body, nil
	}
	mm, ok := normalize(v).(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("skills: frontmatter is not a YAML mapping")
	}
	return mm, body, nil
}

// normalize turns map[any]any (non-string keys) into map[string]any so the
// result can go through encoding/json.
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = normalize(x)
		}
		return t
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[fmt.Sprint(k)] = normalize(x)
		}
		return out
	case []any:
		for i, x := range t {
			t[i] = normalize(x)
		}
		return t
	}
	return v
}

// cutLine returns the first line including its end of line, the remainder, and
// whether the text had a line end (more input may follow).
func cutLine(s string) (line, rest string, more bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i+1], s[i+1:], true
		}
	}
	return s, "", false
}

func trimEOL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// splitFrontmatter returns the raw YAML text and the body, and whether the text had a frontmatter.
func splitFrontmatter(b []byte) (yamlText, body string, ok bool) {
	text := string(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")))
	first, rest, has := cutLine(text)
	if !has || trimEOL(first) != "---" {
		return "", text, false
	}
	pos := 0
	for pos <= len(rest) {
		line, after, more := cutLine(rest[pos:])
		if trimEOL(line) == "---" {
			if !more {
				after = ""
			}
			return rest[:pos], after, true
		}
		if !more {
			break
		}
		pos += len(line)
	}
	return "", text, false
}

// parseFrontmatterLenient is the fallback for a frontmatter that is not valid
// YAML (typically a plain description holding ": "). It reads each top-level
// "key: value" line as one scalar: quotes are removed, true and false become
// bool. Indented lines are ignored. It returns false when no key was found.
func parseFrontmatterLenient(b []byte) (map[string]any, string, bool) {
	y, body, ok := splitFrontmatter(b)
	if !ok {
		return nil, "", false
	}
	m := map[string]any{}
	for _, line := range strings.Split(strings.ReplaceAll(y, "\r\n", "\n"), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
		switch val {
		case "true":
			m[key] = true
		case "false":
			m[key] = false
		default:
			m[key] = val
		}
	}
	return m, body, len(m) > 0
}
