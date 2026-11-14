package atelier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// The schema names this client understands. A schema with any other name or
// version suffix is "unsupported" (ErrUnsupported).
const (
	SchemaComponents = "atelier.components/1"
	SchemaWorkflows  = "atelier.workflows/1"
	SchemaState      = "atelier.state/1"
	SchemaValidation = "atelier.validation/1"
	SchemaSavePlan   = "atelier.save-plan/1"
)

// Plan write operations in the contract. Nothing else is accepted.
const (
	opReplace = "replace"
	opDelete  = "delete"
)

// checkSchema verifies that raw is a JSON object whose "schema" equals want.
// Not JSON gives ErrMalformed; a different (or missing) schema gives
// ErrUnsupported with the actual schema in Diagnostic. Unknown keys are fine.
func checkSchema(raw []byte, want string) error {
	var head struct {
		Schema *string `json:"schema"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return &Error{Code: ErrMalformed, ExitCode: 0, Diagnostic: err.Error()}
	}
	got := ""
	if head.Schema != nil {
		got = *head.Schema
	}
	if got != want {
		return &Error{Code: ErrUnsupported, Diagnostic: fmt.Sprintf("schema %q (want %q)", got, want)}
	}
	return nil
}

// Text is Atelier's {"ja","en"} pair.
type Text struct {
	Ja string `json:"ja"`
	En string `json:"en"`
}

// String returns the Japanese text, or the English one when ja is empty.
func (t Text) String() string {
	if t.Ja != "" {
		return t.Ja
	}
	return t.En
}

// Invoke tells how a component is called. Kind is skill|read-skill|script|builtin.
type Invoke struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Plugin string `json:"plugin"`
	Path   string `json:"path"`
	Args   any    `json:"args"`
}

// Requires lists what a component needs.
type Requires struct {
	Skills  []string `json:"skills"`
	Plugins []string `json:"plugins"`
	Cli     []string `json:"cli"`
	Env     []string `json:"env"`
}

// ComponentData is the component definition as Atelier stores it.
type ComponentData struct {
	ID          string    `json:"id"`
	Stages      []string  `json:"stages"`
	Title       Text      `json:"title"`
	Invoke      Invoke    `json:"invoke"`
	Doc         string    `json:"doc"`
	Where       string    `json:"where"`
	Memory      string    `json:"memory"`
	When        string    `json:"when"`
	Cost        string    `json:"cost"`
	Interactive bool      `json:"interactive"`
	CallsBack   bool      `json:"calls_back"`
	Requires    *Requires `json:"requires"`
	Forbid      []string  `json:"forbid"`
	License     string    `json:"license"`
	Link        string    `json:"link"`
	Source      string    `json:"source"`
	Then        []string  `json:"then"`
}

// Component is one item of atelier.components/1. Source is bundled|home.
type Component struct {
	ID     string        `json:"id"`
	Source string        `json:"source"`
	Data   ComponentData `json:"data"`
}

// WorkflowEntry is one component reference in a workflow. Atelier writes it
// either as a bare id string or as {"id", "when"?, "where"?, "models"?}.
type WorkflowEntry struct {
	ID     string   `json:"id"`
	When   string   `json:"when"`
	Where  string   `json:"where"`
	Models []string `json:"models"`
}

// UnmarshalJSON reads both the string form and the object form.
func (e *WorkflowEntry) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var id string
		if err := json.Unmarshal(b, &id); err != nil {
			return err
		}
		*e = WorkflowEntry{ID: id}
		return nil
	}
	type plain WorkflowEntry
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*e = WorkflowEntry(p)
	return nil
}

// Step kinds in Workflow.Steps.
const (
	StepStage    = "stage"
	StepOptional = "optional"
	StepGate     = "gate"
)

// WorkflowStep is one box of the flow diagram: a stage, an optional stage, or
// a gate. After names the stage it follows (empty for a plain stage). For a
// gate, Stage is "gate".
type WorkflowStep struct {
	Kind       string          `json:"kind"`
	Stage      string          `json:"stage"`
	After      string          `json:"after"`
	Components []WorkflowEntry `json:"components"`
}

// Workflow is one item of atelier.workflows/1, keyed by Name. Raw keeps the
// original "data" object.
type Workflow struct {
	Name        string         `json:"name"`
	Source      string         `json:"source"`
	Bundled     bool           `json:"bundled"`
	Description Text           `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
	Raw         map[string]any `json:"data"`
}

// stageOrder is the fixed order of the nine stages (flow.py STAGES).
var stageOrder = []string{
	"understand", "design", "plan", "route", "build", "verify", "review", "fix", "finish",
}

type wireWorkflow struct {
	Name    string          `json:"name"`
	Source  string          `json:"source"`
	Bundled bool            `json:"bundled"`
	Data    json.RawMessage `json:"data"`
}

type wireGroup struct {
	Stage      string          `json:"stage"`
	After      string          `json:"after"`
	Components []WorkflowEntry `json:"components"`
}

// buildWorkflow turns one wire item into a Workflow.
func buildWorkflow(w wireWorkflow) (Workflow, error) {
	out := Workflow{Name: w.Name, Source: w.Source, Bundled: w.Bundled}
	if len(bytes.TrimSpace(w.Data)) == 0 || string(bytes.TrimSpace(w.Data)) == "null" {
		return out, nil
	}
	if err := json.Unmarshal(w.Data, &out.Raw); err != nil {
		return out, err
	}
	var data struct {
		Description Text            `json:"description"`
		Stages      json.RawMessage `json:"stages"`
		Gates       []wireGroup     `json:"gates"`
		Optional    []wireGroup     `json:"optional"`
	}
	if err := json.Unmarshal(w.Data, &data); err != nil {
		return out, err
	}
	out.Description = data.Description

	names, byName, err := orderedObject(data.Stages)
	if err != nil {
		return out, err
	}
	// Stage order: the fixed stages first, then unknown names in document order.
	known := map[string]bool{}
	for _, s := range stageOrder {
		known[s] = true
	}
	order := make([]string, 0, len(names))
	for _, s := range stageOrder {
		if _, ok := byName[s]; ok {
			order = append(order, s)
		}
	}
	for _, s := range names {
		if !known[s] {
			order = append(order, s)
		}
	}

	usedOpt := make([]bool, len(data.Optional))
	usedGate := make([]bool, len(data.Gates))
	steps := []WorkflowStep{}
	for _, s := range order {
		var entries []WorkflowEntry
		if err := json.Unmarshal(byName[s], &entries); err != nil {
			return out, err
		}
		steps = append(steps, WorkflowStep{Kind: StepStage, Stage: s, Components: entries})
		for i, o := range data.Optional {
			if o.After == s {
				usedOpt[i] = true
				steps = append(steps, WorkflowStep{Kind: StepOptional, Stage: o.Stage, After: o.After, Components: o.Components})
			}
		}
		for i, g := range data.Gates {
			if g.After == s {
				usedGate[i] = true
				steps = append(steps, WorkflowStep{Kind: StepGate, Stage: "gate", After: g.After, Components: g.Components})
			}
		}
	}
	// Anything that follows a stage the flow does not have is kept, at the end.
	for i, o := range data.Optional {
		if !usedOpt[i] {
			steps = append(steps, WorkflowStep{Kind: StepOptional, Stage: o.Stage, After: o.After, Components: o.Components})
		}
	}
	for i, g := range data.Gates {
		if !usedGate[i] {
			steps = append(steps, WorkflowStep{Kind: StepGate, Stage: "gate", After: g.After, Components: g.Components})
		}
	}
	out.Steps = steps
	return out, nil
}

// orderedObject returns the keys of a JSON object in document order and the
// raw value of each key. An absent or null object gives no keys.
func orderedObject(raw json.RawMessage) ([]string, map[string]json.RawMessage, error) {
	vals := map[string]json.RawMessage{}
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || string(t) == "null" {
		return nil, vals, nil
	}
	dec := json.NewDecoder(bytes.NewReader(t))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, fmt.Errorf("stages is not an object")
	}
	var keys []string
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, nil, fmt.Errorf("unexpected token %v", kt)
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, nil, err
		}
		if _, dup := vals[key]; !dup {
			keys = append(keys, key)
		}
		vals[key] = v
	}
	if _, err := dec.Token(); err != nil && err != io.EOF {
		return nil, nil, err
	}
	return keys, vals, nil
}

// ModelRef is the price and quality reference of a model (values may be null).
type ModelRef struct {
	Quality  *float64 `json:"quality"`
	PriceIn  *float64 `json:"price_in"`
	PriceOut *float64 `json:"price_out"`
	Sample   *bool    `json:"sample"`
	Source   *string  `json:"source"`
}

// Model is one entry of a runner's model list.
type Model struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Efforts []string  `json:"efforts"`
	Ref     *ModelRef `json:"ref"`
	Manual  bool      `json:"manual"`
}

// ModelList is a runner's fetched model list.
type ModelList struct {
	FetchedAt string  `json:"fetched_at"`
	Method    string  `json:"method"`
	Error     *string `json:"error"`
	FailedAt  *string `json:"failed_at"`
	List      []Model `json:"list"`
}

// Runner is one runner of atelier.state/1. Selected nil means every model.
type Runner struct {
	Enabled         bool       `json:"enabled"`
	Path            string     `json:"path"`
	Version         string     `json:"version"`
	CheckedAt       string     `json:"checked_at"`
	Billing         string     `json:"billing"`
	Selected        []string   `json:"selected"`
	MissingSelected []string   `json:"missing_selected"`
	Models          *ModelList `json:"models"`
}

// Step is one routing step.
type Step struct {
	Runner string  `json:"runner"`
	Model  string  `json:"model"`
	Effort *string `json:"effort"`
	Until  string  `json:"until,omitempty"`
}

// RoutingTable is the routing table (tiers haiku/sonnet/opus).
type RoutingTable struct {
	Name  string            `json:"name"`
	Tiers map[string][]Step `json:"tiers"`
}

// Routing is the routing state. Source is default|home.
type Routing struct {
	Source string       `json:"source"`
	Table  RoutingTable `json:"table"`
}

// TemplateStep is a step of a priority template: either a plain step or a
// {"rotate": [step, ...]} group.
type TemplateStep struct {
	Runner string  `json:"runner,omitempty"`
	Model  string  `json:"model,omitempty"`
	Effort *string `json:"effort,omitempty"`
	Until  string  `json:"until,omitempty"`
	Rotate []Step  `json:"rotate,omitempty"`
}

// Template is a priority template (or a bundled example).
type Template struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Tiers       map[string][]TemplateStep `json:"tiers"`
}

// Application is the template currently applied, if any.
type Application struct {
	ID         string  `json:"id"`
	Template   string  `json:"template"`
	Until      *string `json:"until"`
	OnExpire   string  `json:"on_expire"`
	Reason     string  `json:"reason"`
	StartedAt  string  `json:"started_at"`
	StartedVia string  `json:"started_via"`
}

// State is atelier.state/1. Raw is the stdout as read (not sent to the screen).
type State struct {
	Home              string              `json:"home"`
	Runners           map[string]Runner   `json:"runners"`
	Routing           Routing             `json:"routing"`
	Templates         map[string]Template `json:"templates"`
	Examples          map[string]Template `json:"examples"`
	Application       *Application        `json:"application"`
	ApplicationStatus string              `json:"application_status"`
	Warnings          []string            `json:"warnings"`
	Raw               json.RawMessage     `json:"-"`
}

// Validation is atelier.validation/1.
type Validation struct {
	Kind     string   `json:"kind"`
	Valid    bool     `json:"valid"`
	Problems []string `json:"problems"`
}

// PlanResult is the outcome of Plan: either a failed validation (Plan nil) or
// a valid result with the save plan.
type PlanResult struct {
	Validation Validation `json:"validation"`
	Plan       *SavePlan  `json:"plan"`
}

// PlanEntry is one file of a directory dependency.
type PlanEntry struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	SHA256 string `json:"sha256"`
}

// PlanRead is a file or directory the plan depends on.
type PlanRead struct {
	Root    string      `json:"root"`
	Type    string      `json:"type"`
	Path    string      `json:"path"`
	SHA256  string      `json:"sha256"`
	Missing bool        `json:"missing"`
	Entries []PlanEntry `json:"entries"`
}

// PlanWrite is one write of the plan. Op is replace or delete.
type PlanWrite struct {
	Op          string `json:"op"`
	Path        string `json:"path"`
	ContentUTF8 string `json:"content_utf8"`
}

// SavePlan is atelier.save-plan/1.
type SavePlan struct {
	Kind        string      `json:"kind"`
	EvaluatedAt string      `json:"evaluated_at"`
	Reads       []PlanRead  `json:"reads"`
	Writes      []PlanWrite `json:"writes"`
}
