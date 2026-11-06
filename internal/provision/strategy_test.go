package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutomaticStrategy(t *testing.T) {
	digest := Capabilities{Digest: true, Blocks: true}
	cases := []struct {
		name    string
		request Request
		targets int
		caps    Capabilities
		want    string
	}{
		{"one factory VENTUNO", Request{Mode: "golden", Board: "ventunoq", Strategy: "auto", Jobs: 16}, 1, digest, "full"},
		{"parallel factory VENTUNO", Request{Mode: "golden", Board: "ventunoq", Strategy: "auto", Jobs: 16}, 16, digest, "blocks"},
		{"serialised VENTUNO batch", Request{Mode: "golden", Board: "ventunoq", Strategy: "auto", Jobs: 1}, 16, digest, "full"},
		{"one UNO", Request{Mode: "golden", Board: "unoq", Strategy: "auto", Jobs: 16}, 1, digest, "hierarchical"},
		{"parallel factory UNO", Request{Mode: "golden", Board: "unoq", Strategy: "auto", Jobs: 16}, 16, digest, "blocks"},
		{"similar VENTUNO batch", Request{Mode: "golden", Board: "ventunoq", Strategy: "auto", Jobs: 16, TargetContents: "similar"}, 16, digest, "hierarchical"},
		{"no helper factory", Request{Mode: "golden", Board: "unoq", Strategy: "auto", Jobs: 16}, 16, Capabilities{}, "full"},
		{"no helper similar", Request{Mode: "golden", Board: "unoq", Strategy: "auto", Jobs: 16, TargetContents: "similar"}, 16, Capabilities{}, "readback"},
		{"older CLI", Request{Mode: "golden", Board: "unoq", Strategy: "auto", Jobs: 16}, 16, Capabilities{Digest: true}, "hierarchical"},
		{"factory archive", Request{Mode: "factory", Board: "unoq", Strategy: "auto", Jobs: 16}, 16, digest, "factory"},
		{"manual full", Request{Mode: "golden", Board: "unoq", Strategy: "full", Jobs: 16, TargetContents: "similar"}, 16, digest, "full"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, label, reason, automatic := selectStrategy(tc.request, tc.targets, tc.caps)
			if key != tc.want || label == "" || reason == "" {
				t.Fatalf("%s %s %s", key, label, reason)
			}
			if automatic != (tc.request.Strategy == "auto") {
				t.Fatalf("Unexpected automatic %v", automatic)
			}
		})
	}
}
func TestPlanSelectsActualTargetCountAndCommand(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{"version":1,"board":"ventunoq","source_serial":"12345678","luns":[{"sector_size":512,"sectors":100}]}`), 0600)
	e := &Engine{adapter: &Adapter{Digest: "/tools/qdl-digest", CLI: "/tools/cli", Caps: Capabilities{CLI: true, Golden: true, Digest: true, Blocks: true}}}
	r := Request{Mode: "golden", Board: "ventunoq", Image: root, Targets: []string{"0x12345678", "0xABCDEF12"}, Jobs: 16, TimeoutMinutes: 120, Strategy: "auto"}
	p, err := e.Plan(r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Strategy != "full" || len(p.Targets) != 1 {
		t.Fatalf("Source exclusion not used: %v", p)
	}
	command := strings.Join(p.Commands[0], " ")
	if strings.Contains(command, "--skip-unchanged") || !strings.Contains(command, "--verification digest") {
		t.Fatal(command)
	}
	r.Targets = append(r.Targets, "0x99999999")
	p, err = e.Plan(r)
	if err != nil {
		t.Fatal(err)
	}
	command = strings.Join(p.Commands[0], " ")
	if p.Strategy != "blocks" || !strings.Contains(command, "--digest-comparison blocks") || !strings.Contains(command, "--skip-unchanged") {
		t.Fatal(command)
	}
	r.Strategy = "full"
	p, err = e.Plan(r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Automatic || p.Strategy != "full" || strings.Contains(strings.Join(p.Commands[0], " "), "--skip-unchanged") {
		t.Fatal("Manual override ignored")
	}
}
