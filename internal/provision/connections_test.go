package provision

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPresenceIncludesBootedBoardsAndNestedLocations(t *testing.T) {
	var tree plist
	err := xml.Unmarshal([]byte(`<plist><array><dict><key>idVendor</key><integer>9025</integer><key>locationID</key><integer>1234</integer><key>children</key><array><dict><key>idVendor</key><integer>1478</integer><key>locationID</key><integer>5678</integer></dict></array></dict></array></plist>`), &tree)
	if err != nil {
		t.Fatal(err)
	}
	if got := plistUSBPaths(tree); !reflect.DeepEqual(got, []string{"ioreg:1234", "ioreg:5678"}) {
		t.Fatal(got)
	}
}

func TestFixtureProgressSerial(t *testing.T) {
	e := ProgressEvent("[A07 0x8ED963AB] FAILED: USB disconnected", "image")
	if e.Serial != "0x8ED963AB" || e.Status != "failed" {
		t.Fatal(e)
	}
	e = ProgressEvent("[A01 0x12345678] write — 25.0% | 4 MiB / 16 MiB", "image")
	if e.Serial != "0x12345678" || e.Percent != 25 || e.Data != "4 MiB / 16 MiB" {
		t.Fatal(e)
	}
}

func TestFixtureRestoreArguments(t *testing.T) {
	a := &Adapter{Caps: Capabilities{BoardFirst: true}}
	args := a.RestoreArgs(Request{Board: "unoq", Image: "/image", Fixture: "/fixture with spaces.json"}, []string{"0x12345678"})
	if !strings.Contains(strings.Join(args, "|"), "--fixture|/fixture with spaces.json") {
		t.Fatal(args)
	}
}

func TestFailedRunRetainsOriginalPort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := &Engine{demo: true, running: true, done: make(chan struct{}), emit: func(Event) {}}
	dir := t.TempDir()
	e.run(ctx, Request{Mode: "golden"}, Plan{Targets: []string{"0x12345678"}}, dir, map[string]Device{"0x12345678": {Serial: "0x12345678", Slot: "A07", Path: "ioreg:1234"}})
	data, err := os.ReadFile(filepath.Join(dir, "results.json"))
	if err != nil {
		t.Fatal(err)
	}
	var results []Result
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Slot != "A07" || results[0].Path != "ioreg:1234" || results[0].Status != "cancelled" {
		t.Fatal(string(data))
	}
}
