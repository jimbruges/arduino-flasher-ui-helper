package provision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Adapter struct {
	CLI, QDL, Digest string
	Caps             Capabilities
}

func NewAdapter(dir string) *Adapter {
	dir, _ = filepath.Abs(dir)
	a := &Adapter{CLI: filepath.Join(dir, "arduino-flasher-cli"), QDL: filepath.Join(dir, "qdl"), Digest: filepath.Join(dir, "qdl-digest")}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	help, err := exec.CommandContext(ctx, a.CLI, "--help").CombinedOutput()
	if err != nil {
		a.Caps.Problem = "Build the bundled tools before connecting boards (scripts/build-tools.sh)."
		return a
	}
	a.Caps.CLI = true
	a.Caps.Golden = strings.Contains(string(help), "golden")
	version, _ := exec.CommandContext(ctx, a.CLI, "version").CombinedOutput()
	a.Caps.Version = strings.TrimSpace(string(version))
	if a.Caps.Golden {
		help, _ = exec.CommandContext(ctx, a.CLI, "golden", "flash", "--help").CombinedOutput()
		a.Caps.BoardFirst = strings.Contains(string(help), "flash BOARD IMAGE")
		a.Caps.Blocks = strings.Contains(string(help), "--digest-comparison")
		a.Caps.Digest = strings.Contains(string(help), "--digest-qdl") && executable(a.Digest)
	}
	return a
}
func executable(path string) bool {
	f, err := os.Stat(path)
	return err == nil && !f.IsDir() && f.Mode()&0111 != 0
}
func (a *Adapter) GoldenArgs(action, board, image string) []string {
	if a.Caps.BoardFirst {
		return []string{"golden", action, board, image}
	}
	return []string{"golden", action, image, "--board", board}
}
func (a *Adapter) RestoreArgs(r Request, targets []string) []string {
	args := a.GoldenArgs("flash", r.Board, r.Image)
	args = append(args, "--serial", strings.Join(targets, ","), "--jobs", strconv.Itoa(r.Jobs), "--timeout", strconv.Itoa(r.TimeoutMinutes)+"m")
	if r.Strategy != "readback" && a.Caps.Digest {
		args = append(args, "--verification", "digest", "--digest-qdl", a.Digest)
		if r.Strategy != "full" {
			args = append(args, "--skip-unchanged", "--digest-block-mib", "64")
		}
		if r.Strategy == "blocks" && a.Caps.Blocks {
			args = append(args, "--digest-comparison", "blocks")
		}
	} else if r.Strategy != "full" {
		args = append(args, "--skip-unchanged")
	}
	return args
}
func InspectImage(path string) (ImageInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ImageInfo{}, err
	}
	info := ImageInfo{Path: abs, Name: filepath.Base(abs)}
	f, err := os.Open(filepath.Join(abs, "manifest.json"))
	if err != nil {
		return info, errors.New("Choose a golden image folder containing manifest.json.")
	}
	defer f.Close()
	var m struct {
		Version int    `json:"version"`
		Board   string `json:"board"`
		Source  string `json:"source_serial"`
		LUNs    []struct {
			Sector  uint64 `json:"sector_size"`
			Sectors uint64 `json:"sectors"`
		} `json:"luns"`
	}
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return info, fmt.Errorf("Invalid golden image manifest: %w", err)
	}
	if m.Version != 1 || (m.Board != "unoq" && m.Board != "ventunoq") || len(m.LUNs) == 0 {
		return info, errors.New("Unsupported golden image manifest.")
	}
	info.Board = m.Board
	info.Source, err = NormalizeSerial("0x" + m.Source)
	if err != nil {
		return info, err
	}
	info.LUNs = len(m.LUNs)
	for _, l := range m.LUNs {
		if (l.Sector != 512 && l.Sector != 4096) || l.Sectors == 0 || l.Sectors > ^uint64(0)/l.Sector {
			return info, errors.New("Invalid image geometry.")
		}
		bytes := l.Sector * l.Sectors
		if info.Bytes > ^uint64(0)-bytes {
			return info, errors.New("Image size overflow.")
		}
		info.Bytes += bytes
	}
	return info, nil
}
func NormalizeSerial(value string) (string, error) {
	v := strings.TrimSpace(value)
	base := 10
	if strings.HasPrefix(strings.ToLower(v), "0x") {
		v = v[2:]
		base = 16
	} else if strings.ContainsAny(v, "abcdefABCDEF") {
		base = 16
	}
	n, err := strconv.ParseUint(v, base, 32)
	if err != nil {
		return "", fmt.Errorf("Invalid EDL serial %q", value)
	}
	return fmt.Sprintf("0x%08X", n), nil
}

var sourceSerial = regexp.MustCompile(`_SN:([0-9A-Fa-f]+)(?:[ _]|$)`)

func (a *Adapter) Devices(ctx context.Context) ([]Device, error) {
	if a.Caps.Golden {
		out, err := exec.CommandContext(ctx, a.CLI, "golden", "devices").Output()
		if err != nil {
			return nil, fmt.Errorf("EDL discovery: %w", err)
		}
		var devices []Device
		if err := json.Unmarshal(out, &devices); err != nil {
			return nil, err
		}
		return normalizeDevices(devices)
	}
	return systemDevices(ctx)
}
func normalizeDevices(devices []Device) ([]Device, error) {
	seen := map[string]bool{}
	out := make([]Device, 0, len(devices))
	for _, d := range devices {
		s, err := NormalizeSerial(d.Serial)
		if err != nil {
			return nil, err
		}
		if seen[s] {
			return nil, fmt.Errorf("Two devices report serial %s; reconnect them before flashing.", s)
		}
		seen[s] = true
		d.Serial = s
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Serial < out[j].Serial })
	return out, nil
}
