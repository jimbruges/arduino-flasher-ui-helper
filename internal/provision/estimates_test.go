package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaleUsesImageAndTargets(t *testing.T) {
	p := Plan{Bytes: 15675162624, Targets: []string{"a", "b", "c"}, Strategy: "blocks"}
	describeScale(Request{Mode: "golden", Board: "unoq"}, &p)
	for _, text := range []string{"14.60 GiB", "43.80 GiB", "total", "Up to"} {
		if !strings.Contains(p.DataScale, text) {
			t.Fatalf("missing %q: %s", text, p.DataScale)
		}
	}
	if p.Timing != "No reliable batch estimate yet" {
		t.Fatal(p.Timing)
	}
}
func TestCaptureAndFactoryScale(t *testing.T) {
	p := Plan{}
	describeScale(Request{Mode: "capture", Board: "ventunoq"}, &p)
	if p.DataScale != "Read from board during capture" || !strings.Contains(p.Timing, "typical capture") {
		t.Fatalf("%+v", p)
	}
	describeScale(Request{Mode: "factory", Board: "ventunoq"}, &p)
	if !strings.Contains(p.DataScale, "preparation") || p.Timing != "Step ETA shown during transfer" {
		t.Fatalf("%+v", p)
	}
}
func TestCompactAndLegacyProgress(t *testing.T) {
	for _, line := range []string{
		"[0x123] readback 5.1% | 0.75 GiB/14.60 GiB | 30s | 15.1 MiB/s | phase ETA ~9m",
		"[0x123] readback — 5.1% (0.75 GiB/14.60 GiB) | elapsed 30s | 15.1 MiB/s | ETA ~9m",
	} {
		e := ProgressEvent(line, "")
		if e.Phase != "readback" || e.Elapsed != "30s" || e.Data != "0.75 GiB / 14.60 GiB" || e.Percent != 5.1 || e.ETA != "9m" {
			t.Fatalf("%+v", e)
		}
	}
	e := ProgressEvent("[0x123] write done: 14.60 GiB in 20m30s (12.1 MiB/s)", "")
	if e.Kind != "progress" || e.Phase != "write" || e.Percent != 100 || e.Data != "14.60 GiB / 14.60 GiB" || e.Elapsed != "20m30s" {
		t.Fatalf("%+v", e)
	}
}

func TestFactorySpanUsesDeclaredRanges(t *testing.T) {
	dir := t.TempDir()
	for _, content := range []string{
		`<data><program filename="compressed.sparse" num_partition_sectors="1024" SECTOR_SIZE_IN_BYTES="4096"/><program filename="" num_partition_sectors="100" SECTOR_SIZE_IN_BYTES="4096"/></data>`,
		`<data><program filename="x" num_partition_sectors="18446744073709551615" SECTOR_SIZE_IN_BYTES="4096"/></data>`,
	} {
		if err := os.WriteFile(filepath.Join(dir, "rawprogram0.xml"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		bytes, ok := factoryProgramSpan(dir, []string{"rawprogram0.xml", "patch0.xml"})
		if strings.Contains(content, "compressed.sparse") {
			if !ok || bytes != 4194304 {
				t.Fatalf("%d %v", bytes, ok)
			}
		} else if ok {
			t.Fatal("overflow accepted")
		}
	}
}

func TestStartedTransferHasSize(t *testing.T) {
	e := ProgressEvent("[0x123] write started (7.48 GiB); 12 chunks, progress on acknowledgement", "")
	if e.Data != "0 B / 7.48 GiB" || e.Phase != "write" || e.Kind != "progress" {
		t.Fatalf("%+v", e)
	}
}
func TestTimingScalesWithSelectedImage(t *testing.T) {
	small := Plan{Bytes: 15675162624 / 2, Targets: []string{"a"}, Strategy: "full"}
	describeScale(Request{Mode: "golden", Board: "unoq"}, &small)
	if small.DataScale != "7.30 GiB per board" || small.Timing != "About 7–18 min" {
		t.Fatalf("%+v", small)
	}
}
