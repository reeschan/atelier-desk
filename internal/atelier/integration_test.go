package atelier_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/reeschan/atelier-desk/internal/atelier"
	"github.com/reeschan/atelier-desk/internal/paths"
)

// TestRealAtelier runs the active Atelier version on this machine. It only
// reads (state, components, workflows, validate); validate never saves. It is
// off unless ATELIER_DESK_INTEGRATION=1, because it reads the real Atelier home.
func TestRealAtelier(t *testing.T) {
	if os.Getenv("ATELIER_DESK_INTEGRATION") != "1" {
		t.Skip("set ATELIER_DESK_INTEGRATION=1 to run against the real Atelier")
	}
	loc, err := paths.Resolve(paths.Options{})
	if err != nil {
		t.Skipf("no active Atelier version on this machine: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := atelier.Config{VersionDir: loc.VersionDir}
	probe, err := atelier.NewCLIClient(nil, cfg).Probe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if probe.Status != atelier.StatusReady {
		t.Fatalf("Probe = %q/%q: %s", probe.Status, probe.Code, probe.Diagnostic)
	}

	cfg.Home = probe.Home
	c := atelier.NewCLIClient(nil, cfg)

	st, err := c.State(ctx)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Home == "" || len(st.Raw) == 0 {
		t.Errorf("State home = %q, raw %d bytes; want both set", st.Home, len(st.Raw))
	}
	if comps, err := c.Components(ctx); err != nil {
		t.Errorf("Components: %v", err)
	} else if len(comps) == 0 {
		t.Error("Components returned no component")
	}
	if wfs, err := c.Workflows(ctx); err != nil {
		t.Errorf("Workflows: %v", err)
	} else if len(wfs) == 0 {
		t.Error("Workflows returned no workflow")
	}

	// validate tells the routing table the state reports (valid) from an empty body (not valid).
	var raw struct {
		Routing struct {
			Table json.RawMessage `json:"table"`
		} `json:"routing"`
	}
	if err := json.Unmarshal(st.Raw, &raw); err != nil {
		t.Fatal(err)
	}
	good, err := c.Validate(ctx, atelier.Request{Kind: "routing", Input: raw.Routing.Table})
	if err != nil {
		t.Fatalf("Validate (state's routing table): %v", err)
	}
	if !good.Valid {
		t.Errorf("the state's own routing table is not valid: %v", good.Problems)
	}
	bad, err := c.Validate(ctx, atelier.Request{Kind: "routing", Input: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("Validate (empty body): %v", err)
	}
	if bad.Valid || len(bad.Problems) == 0 {
		t.Errorf("an empty routing body = %+v, want not valid with problems", bad)
	}
}
