package provision

import (
	"archive/tar"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRestoreArguments(t *testing.T) {
	for _, boardFirst := range []bool{false, true} {
		a := &Adapter{Digest: "/tools/qdl-digest", Caps: Capabilities{BoardFirst: boardFirst, Digest: true, Blocks: true}}
		r := Request{Board: "ventunoq", Image: "/golden image", Strategy: "blocks", Jobs: 16, TimeoutMinutes: 120}
		args := strings.Join(a.RestoreArgs(r, []string{"0x12345678", "0xABCDEF12"}), "|")
		for _, want := range []string{"--serial|0x12345678,0xABCDEF12", "--jobs|16", "--verification|digest", "--skip-unchanged", "--digest-comparison|blocks", "--digest-block-mib|64"} {
			if !strings.Contains(args, want) {
				t.Fatalf("Missing %s in %s", want, args)
			}
		}
		if boardFirst && !strings.HasPrefix(args, "golden|flash|ventunoq|/golden image") {
			t.Fatal(args)
		}
	}
}
func TestGoldenMixedResults(t *testing.T) {
	out := []byte("progress\n[ {\"serial\":\"12345678\",\"status\":\"verified\"}, {\"serial\":\"ABCDEF12\",\"status\":\"failed\",\"error\":\"mismatch\"} ]\n")
	results, err := parseGoldenResults(out, []string{"0x12345678", "0xABCDEF12"})
	if err == nil || len(results) != 2 || results[0].Status != "verified" || results[1].Status != "failed" {
		t.Fatalf("%v %v", results, err)
	}
	_, err = parseGoldenResults([]byte(`[{"serial":"99999999","status":"verified"}]`), []string{"0x12345678"})
	if err == nil {
		t.Fatal("Accepted unexpected target")
	}
}
func TestArchiveTraversalAndLinks(t *testing.T) {
	for _, header := range []*tar.Header{{Name: "../escape", Mode: 0600, Size: 1, Typeflag: tar.TypeReg}, {Name: "link", Linkname: "/tmp", Typeflag: tar.TypeSymlink}} {
		t.Run(header.Name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "image.tar")
			f, _ := os.Create(archive)
			w := tar.NewWriter(f)
			w.WriteHeader(header)
			if header.Size > 0 {
				w.Write([]byte("x"))
			}
			w.Close()
			f.Close()
			if err := extractArchive(context.Background(), archive, filepath.Join(root, "extract")); err == nil {
				t.Fatal("Accepted unsafe archive")
			}
		})
	}
}
func TestWorkerMetadataIsolation(t *testing.T) {
	source := t.TempDir()
	os.WriteFile(filepath.Join(source, "rawprogram0.xml"), []byte("original"), 0600)
	os.WriteFile(filepath.Join(source, "gpt_main0.bin"), []byte("GPT"), 0600)
	os.WriteFile(filepath.Join(source, "rootfs.img"), []byte("payload"), 0600)
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	for _, dest := range []string{first, second} {
		if err := privateFactory(context.Background(), source, dest); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(first, "gpt_main0.bin"), []byte("changed"), 0600)
	os.WriteFile(filepath.Join(first, "rawprogram0.xml"), []byte("changed"), 0600)
	for _, dest := range []string{source, second} {
		data, _ := os.ReadFile(filepath.Join(dest, "gpt_main0.bin"))
		if string(data) != "GPT" {
			t.Fatal("GPT shared across workers")
		}
		data, _ = os.ReadFile(filepath.Join(dest, "rawprogram0.xml"))
		if string(data) != "original" {
			t.Fatal("XML shared across workers")
		}
	}
}
func TestFactoryMissingPayload(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "rawprogram0.xml"), []byte(`<data><program filename="missing.img"/></data>`), 0600)
	os.WriteFile(filepath.Join(root, "patch0.xml"), []byte(`<patches/>`), 0600)
	os.WriteFile(filepath.Join(root, "prog_firehose_ddr.elf"), []byte("ELF"), 0600)
	if _, err := validateFactory(root); err == nil {
		t.Fatal("Accepted missing payload")
	}
}
func TestProcessGroupCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process-group test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	out, err := runCommand(ctx, "/bin/sh", []string{"-c", "sleep 30 & echo child-started; wait"}, "", nil)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Second || !strings.Contains(string(out), "child-started") {
		t.Fatalf("Cancellation failed: %s %v", out, err)
	}
}
func TestGoldenSourceExcluded(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{"version":1,"board":"unoq","source_serial":"12345678","luns":[{"sector_size":512,"sectors":100}]}`), 0600)
	e := &Engine{adapter: &Adapter{Caps: Capabilities{CLI: true, Golden: true, Digest: true}}}
	p, err := e.Plan(Request{Mode: "golden", Board: "unoq", Image: root, Targets: []string{"0x12345678", "0xABCDEF12"}, Jobs: 16, TimeoutMinutes: 120, Strategy: "recommended"})
	if err != nil || len(p.Targets) != 1 || p.Targets[0] != "0xABCDEF12" {
		t.Fatalf("%v %v", p, err)
	}
}

func TestFactoryBatchMixedResults(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	root := t.TempDir()
	image := filepath.Join(root, "image", "flash")
	os.MkdirAll(image, 0700)
	for name, data := range map[string]string{"rawprogram0.xml": `<data><program filename="gpt_main0.bin"/></data>`, "patch0.xml": `<patches/>`, "gpt_main0.bin": "original", "prog_firehose_ddr.elf": "ELF"} {
		if err := os.WriteFile(filepath.Join(image, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cli := filepath.Join(root, "mock-cli")
	script := "#!/bin/sh\nif [ \"$1\" != flash ]; then exit 9; fi\nif [ \"$(cat \"$3/flash/gpt_main0.bin\")\" != original ]; then exit 8; fi\nprintf changed > \"$3/flash/gpt_main0.bin\"\nif [ \"$5\" = 0xABCDEF12 ]; then exit 2; fi\necho 'flashed rootfs successfully'\n"
	os.WriteFile(cli, []byte(script), 0700)
	e := &Engine{adapter: &Adapter{CLI: cli}, emit: func(Event) {}}
	targets := []string{"0x12345678", "0xABCDEF12", "0x99999999"}
	results, err := e.runFactory(context.Background(), Request{Mode: "factory", Board: "unoq", Image: filepath.Dir(image), TimeoutMinutes: 1}, Plan{Targets: targets, Jobs: 2}, root)
	if err == nil || len(results) != 3 || results[0].Status != "flashed" || results[1].Status != "failed" || results[2].Status != "flashed" {
		t.Fatalf("%v %v", results, err)
	}
	data, _ := os.ReadFile(filepath.Join(image, "gpt_main0.bin"))
	if string(data) != "original" {
		t.Fatal("User factory image modified")
	}
}

func TestRestoreSourceOverride(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{"version":1,"board":"unoq","source_serial":"12345678","luns":[{"sector_size":512,"sectors":100}]}`), 0600)
	e := &Engine{adapter: &Adapter{Caps: Capabilities{CLI: true, Golden: true, Digest: true}}}
	r := Request{Mode: "golden", Board: "unoq", Image: root, Targets: []string{"0x12345678"}, Jobs: 16, TimeoutMinutes: 120, Strategy: "recommended", RestoreSource: true}
	p, err := e.Plan(r)
	if err != nil || len(p.Targets) != 1 || p.Excluded != "" || !strings.Contains(strings.Join(p.Commands[0], " "), "--restore-source") {
		t.Fatalf("%v %v", p, err)
	}
	r.Targets = append(r.Targets, "0xABCDEF12")
	if _, err := e.Plan(r); err == nil {
		t.Fatal("Accepted source restoration with other targets")
	}
	r.RestoreSource = false
	r.Targets = []string{"0x12345678"}
	if _, err := e.Plan(r); err == nil {
		t.Fatal("Accepted source restoration without override")
	}
}
