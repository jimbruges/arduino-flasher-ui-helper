package provision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type Engine struct {
	mu      sync.Mutex
	adapter *Adapter
	demo    bool
	running bool
	job     string
	logs    string
	cancel  context.CancelFunc
	done    chan struct{}
	emit    func(Event)
	log     *os.File
}

func NewEngine(tools string, demo bool) (*Engine, error) {
	data, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	data = filepath.Join(data, "arduino-flasher-ui-helper", "runs")
	if err := os.MkdirAll(data, 0700); err != nil {
		return nil, err
	}
	return &Engine{adapter: NewAdapter(tools), demo: demo, logs: data, emit: func(Event) {}}, nil
}
func (e *Engine) SetEmitter(fn func(Event)) { e.mu.Lock(); e.emit = fn; e.mu.Unlock() }
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Status{Demo: e.demo, Running: e.running, Capabilities: e.adapter.Caps, Logs: e.logs, Job: e.job}
}
func (e *Engine) event(event Event) {
	e.mu.Lock()
	event.Job = e.job
	event.Time = time.Now()
	if e.log != nil {
		_ = json.NewEncoder(e.log).Encode(event)
	}
	emit := e.emit
	e.mu.Unlock()
	emit(event)
}
func (e *Engine) Devices(ctx context.Context) ([]Device, error) {
	if e.demo {
		return []Device{{Serial: "0x4A712118", Product: "Simulated EDL board", Path: "Demo USB hub"}, {Serial: "0x8ED963AB", Product: "Simulated EDL board", Path: "Demo USB hub"}, {Serial: "0xC2B2F977", Product: "Simulated EDL board", Path: "Demo USB hub"}}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return e.adapter.Devices(ctx)
}
func (e *Engine) Plan(r Request) (Plan, error) {
	p := Plan{Targets: []string{}, Commands: [][]string{}, Warnings: []string{}}
	if r.Board != "unoq" && r.Board != "ventunoq" {
		return p, errors.New("Choose UNO Q or VENTUNO Q.")
	}
	if !e.demo && !e.adapter.Caps.CLI {
		return p, errors.New(e.adapter.Caps.Problem)
	}
	if r.Jobs < 1 || r.Jobs > 32 {
		return p, errors.New("Concurrency must be between 1 and 32.")
	}
	if r.TimeoutMinutes < 1 || r.TimeoutMinutes > 1440 {
		return p, errors.New("Choose a timeout between 1 minute and 24 hours.")
	}
	if !slices.Contains([]string{"capture", "golden", "factory"}, r.Mode) {
		return p, errors.New("Choose a capture or flashing workflow.")
	}
	if !slices.Contains([]string{"auto", "recommended", "hierarchical", "blocks", "full", "readback"}, r.Strategy) {
		return p, errors.New("Unknown flashing strategy.")
	}
	if r.TargetContents != "" && r.TargetContents != "factory" && r.TargetContents != "similar" {
		return p, errors.New("Unknown target contents expectation.")
	}
	if (r.Mode == "golden" || r.Mode == "capture") && !e.demo && !e.adapter.Caps.Golden {
		return p, errors.New("This CLI does not support golden images. Use a compatible branch or choose a factory image.")
	}
	if r.Mode == "capture" {
		source, err := NormalizeSerial(r.Source)
		if err != nil {
			return p, err
		}
		if strings.TrimSpace(r.Image) == "" {
			return p, errors.New("Choose a new folder for the captured image.")
		}
		if _, err := os.Lstat(r.Image); err == nil {
			return p, errors.New("Capture needs a new folder; the selected path already exists.")
		} else if !os.IsNotExist(err) {
			return p, err
		}
		if r.Programmer != "" {
			if f, err := os.Stat(r.Programmer); err != nil || f.IsDir() {
				return p, errors.New("Choose an existing Firehose programmer file.")
			}
			if r.Version != "" || r.OS != "" {
				return p, errors.New("A local programmer cannot be combined with an online release or OS.")
			}
		}
		p.Strategy, p.StrategyLabel, p.StrategyReason, p.Automatic = selectStrategy(r, 1, e.adapter.Caps)
		p.Title = "Capture your configured board"
		p.Explanation = "Read the complete Linux storage into an offline golden image. The source is not written."
		p.Verification = "Source checksums"
		p.Targets = []string{source}
		p.Jobs = 1
		args := e.adapter.GoldenArgs("capture", r.Board, r.Image)
		args = append(args, "--serial", source, "--timeout", fmt.Sprintf("%dm", r.TimeoutMinutes))
		if r.Programmer != "" {
			args = append(args, "--programmer", r.Programmer)
		} else {
			if r.Version != "" {
				args = append(args, "--version", r.Version)
			}
			if r.OS != "" {
				args = append(args, "--os", r.OS)
			}
		}
		p.Commands = append(p.Commands, append([]string{e.adapter.CLI}, args...))
		p.Warnings = append(p.Warnings, "Shut Linux down cleanly before capture. The separate MCU firmware is not included.")
		describeScale(r, &p)
		return p, nil
	}
	seen := map[string]bool{}
	for _, target := range r.Targets {
		s, err := NormalizeSerial(target)
		if err != nil {
			return p, err
		}
		if seen[s] {
			return p, fmt.Errorf("Duplicate serial %s.", s)
		}
		seen[s] = true
		p.Targets = append(p.Targets, s)
	}
	if len(p.Targets) == 0 {
		return p, errors.New("Select at least one connected target.")
	}
	if r.Mode == "golden" {
		if !e.demo {
			image, err := InspectImage(r.Image)
			if err != nil {
				return p, err
			}
			if image.Board != r.Board {
				return p, fmt.Errorf("This image is for %s, not %s.", image.Board, r.Board)
			}
			if r.RestoreSource {
				if len(p.Targets) != 1 || p.Targets[0] != image.Source {
					return p, errors.New("Restore the golden source separately, selecting only that board.")
				}
				p.Warnings = append(p.Warnings, "The original golden source will be overwritten with this image.")
			} else {
				p.Excluded = image.Source
			}
			p.Bytes = image.Bytes
			if !r.RestoreSource {
				p.Targets = slices.DeleteFunc(p.Targets, func(s string) bool { return s == image.Source })
			}
			if len(p.Targets) == 0 {
				return p, errors.New("Only the captured source is selected. Connect other boards to restore.")
			}
		}
		caps := e.adapter.Caps
		if e.demo {
			caps.Digest = true
			caps.Blocks = true
		}
		p.Strategy, p.StrategyLabel, p.StrategyReason, p.Automatic = selectStrategy(r, len(p.Targets), caps)
		r.Strategy = p.Strategy

		p.Title = "Restore your golden image"
		p.Verification = "Complete readback verification"
		p.Explanation = "Compare before writing; every captured sector is verified after changes."
		if e.adapter.Caps.Digest || e.demo {
			p.Verification = "Complete storage-digest verification"
			p.Explanation = "Hash on each board, transfer only changed blocks, then verify every captured sector."
		}
		if r.Strategy == "full" {
			p.Explanation = "Write all captured storage and verify it. Useful when factory contents differ almost everywhere."
		}
		if r.Strategy == "readback" {
			p.Verification = "Complete readback verification"
			p.Warnings = append(p.Warnings, "Readback needs an image-sized scratch file per active board and uses more USB bandwidth.")
		}
		if !e.adapter.Caps.Digest && !e.demo && r.Strategy != "readback" {
			p.Warnings = append(p.Warnings, "The digest helper is unavailable. This plan uses full readback verification; build the helper to reduce USB traffic.")
		}
		if r.Strategy == "hierarchical" && !e.adapter.Caps.Digest && !e.demo {
			return p, errors.New("Hierarchical comparison requires the digest helper. Choose Automatic or build the tools.")
		}
		if r.Strategy == "blocks" && (!e.adapter.Caps.Blocks || !e.adapter.Caps.Digest) && !e.demo {
			return p, errors.New("Single-pass block comparison requires both a compatible CLI and the digest helper. Choose Automatic or build the tools.")
		}
		p.Commands = append(p.Commands, append([]string{e.adapter.CLI}, e.adapter.RestoreArgs(r, p.Targets)...))
	} else {
		p.Strategy, p.StrategyLabel, p.StrategyReason, p.Automatic = selectStrategy(r, len(p.Targets), e.adapter.Caps)
		p.Title = "Install a factory image"
		p.Verification = "Factory installation (no full-sector verification)"
		p.Explanation = "Prepare the factory image once, then install it on every selected board."
		if r.Image != "" {
			info, err := os.Stat(r.Image)
			if err != nil {
				return p, errors.New("Choose an existing factory archive or extracted image folder.")
			}
			if info.IsDir() {
				if _, err := findFlashDir(r.Image); err != nil {
					return p, err
				}
			}
		}
		if r.Image == "" {
			args := []string{e.adapter.CLI, "download", r.Board, "--dest-dir", "<shared preparation folder>"}
			if r.Version != "" {
				args = append(args, "--version", r.Version)
			}
			if r.OS != "" {
				args = append(args, "--os", r.OS)
			}
			p.Commands = append(p.Commands, args)
		}
		if r.Board == "ventunoq" && !e.demo && !executable(e.adapter.QDL) {
			return p, errors.New("The bundled QDL tool is required to install all VENTUNO Q factory LUNs. Run scripts/build-tools.sh.")
		}
		for _, s := range p.Targets {
			if r.Board == "ventunoq" {
				p.Commands = append(p.Commands, []string{e.adapter.QDL, "--serial", s[2:], "--storage", "emmc", "prog_firehose_ddr.elf", "<all numbered rawprogram and patch files>"})
			} else {
				p.Commands = append(p.Commands, []string{e.adapter.CLI, "flash", r.Board, "<private working image>", "--serial", s, "--yes"})
			}
		}
		p.Warnings = append(p.Warnings, "Factory images do not guarantee identical unused sectors or include your workshop customisations.")
	}
	p.Jobs = min(r.Jobs, len(p.Targets))
	p.Warnings = append(p.Warnings, "Connect only the selected model. EDL serials cannot identify a board model; matching storage capacity gives consistent layouts.")
	describeScale(r, &p)
	return p, nil
}
func (e *Engine) Start(r Request) (string, error) {
	if !r.Ready {
		return "", errors.New("Complete the power, cable and EDL setup checklist first.")
	}
	if !r.Confirmed && r.Mode != "capture" {
		return "", errors.New("Confirm replacing data on the selected boards.")
	}
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return "", errors.New("A job is already running.")
	}
	e.mu.Unlock()
	plan, err := e.Plan(r)
	if err != nil {
		return "", err
	}
	if !e.demo {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		devices, err := e.Devices(ctx)
		cancel()
		if err != nil {
			return "", err
		}
		connected := map[string]bool{}
		for _, d := range devices {
			connected[d.Serial] = true
		}
		for _, s := range plan.Targets {
			if !connected[s] {
				return "", fmt.Errorf("%s disconnected. Refresh the target list.", s)
			}
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return "", errors.New("A job is already running.")
	}
	job := time.Now().Format("20060102-150405.000000000")
	dir := filepath.Join(e.logs, job)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	log, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	e.done = make(chan struct{})
	e.running = true
	e.job = job
	e.log = log
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	if r.Mode == "golden" {
		r.Strategy = plan.Strategy
	}
	go e.run(ctx, r, plan, dir)
	return job, nil
}
func (e *Engine) Cancel() {
	e.mu.Lock()
	cancel := e.cancel
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Stop waits until active subprocesses and result logging have stopped.
func (e *Engine) Stop() bool {
	e.mu.Lock()
	cancel, done := e.cancel, e.done
	active := e.running
	e.mu.Unlock()
	if !active {
		return true
	}
	if cancel != nil {
		cancel()
	}
	select {
	case <-done:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}
func (e *Engine) run(ctx context.Context, r Request, p Plan, dir string) {
	e.event(Event{Kind: "started", Message: p.Title})
	var results []Result
	var err error
	if e.demo {
		results, err = e.runDemo(ctx, r, p)
	} else if r.Mode == "factory" {
		results, err = e.runFactory(ctx, r, p, dir)
	} else {
		results, err = e.runGolden(ctx, r, p)
	}
	if results == nil {
		for _, s := range p.Targets {
			status := "failed"
			message := "Operation failed"
			if err != nil {
				message = err.Error()
			}
			if ctx.Err() != nil {
				status = "cancelled"
			}
			results = append(results, Result{Serial: s, Status: status, Error: message})
		}
	}
	_ = writeJSON(filepath.Join(dir, "request.json"), r)
	_ = writeJSON(filepath.Join(dir, "results.json"), results)
	for _, result := range results {
		e.event(Event{Kind: "result", Serial: result.Serial, Status: result.Status, Elapsed: (time.Duration(result.Seconds) * time.Second).String(), Message: result.Error, BytesWritten: result.BytesWritten, BytesSkipped: result.BytesSkipped, Percent: 100})
	}
	message := "Finished. Remove EDL jumpers and power-cycle completed boards."
	status := "finished"
	if r.Mode == "capture" {
		message = "Golden image saved. Unplug the source board, then connect the new boards you want to restore with this image."
	}
	if err != nil {
		message = err.Error()
		status = "failed"
	}
	if ctx.Err() != nil {
		message = "Stopped. Interrupted boards may need to be flashed again."
		status = "cancelled"
	}
	e.event(Event{Kind: "done", Status: status, Message: message})
	e.mu.Lock()
	if e.log != nil {
		_ = e.log.Close()
		e.log = nil
	}
	close(e.done)
	e.running = false
	e.cancel = nil
	e.mu.Unlock()
}
func (e *Engine) runGolden(ctx context.Context, r Request, p Plan) ([]Result, error) {
	if r.Mode == "golden" {
		args := append(e.adapter.GoldenArgs("flash", r.Board, r.Image), "--serial", strings.Join(p.Targets, ","), "--dry-run")
		e.event(Event{Kind: "phase", Phase: "Checking image", Message: "Validating every source checksum before writing."})
		if _, err := runCommand(ctx, e.adapter.CLI, args, "", func(line string) { e.event(ProgressEvent(line, "image")) }); err != nil {
			return nil, err
		}
	}
	args := p.Commands[0][1:]
	if r.Mode == "golden" {
		args = append(args, "--yes")
	}
	started := time.Now()
	targetCtx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMinutes)*time.Minute)
	defer cancel()
	out, err := runCommand(targetCtx, e.adapter.CLI, args, "", func(line string) { e.event(ProgressEvent(line, r.Source)) })
	if r.Mode == "capture" && err != nil {
		return nil, err
	}
	if r.Mode == "capture" {
		return []Result{{Serial: p.Targets[0], Status: "captured", Seconds: time.Since(started).Seconds()}}, nil
	}
	results, parseErr := parseGoldenResults(out, p.Targets)
	return results, errors.Join(err, parseErr)
}
func parseGoldenResults(out []byte, targets []string) ([]Result, error) {
	var rows []struct {
		Serial  string  `json:"serial"`
		Status  string  `json:"status"`
		Seconds float64 `json:"duration_seconds"`
		Error   string  `json:"error"`
		Written uint64  `json:"bytes_written"`
		Skipped uint64  `json:"bytes_skipped"`
	}
	found := false
	for i := len(out) - 1; i >= 0; i-- {
		if out[i] != '[' {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(string(out[i:])))
		if decoder.Decode(&rows) == nil && len(rows) > 0 {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("The CLI exited without golden verification results.")
	}
	bySerial := map[string]Result{}
	for _, row := range rows {
		s, err := NormalizeSerial("0x" + strings.TrimPrefix(row.Serial, "0x"))
		if err != nil {
			return nil, err
		}
		if !slices.Contains(targets, s) {
			return nil, fmt.Errorf("Unexpected result serial %s", s)
		}
		if _, exists := bySerial[s]; exists {
			return nil, fmt.Errorf("Duplicate result serial %s", s)
		}
		if row.Status != "verified" && row.Status != "failed" && row.Status != "cancelled" {
			return nil, fmt.Errorf("Unexpected result status %s", row.Status)
		}
		bySerial[s] = Result{Serial: s, Status: row.Status, Seconds: row.Seconds, Error: row.Error, BytesWritten: row.Written, BytesSkipped: row.Skipped}
	}
	var results []Result
	var problem error
	for _, s := range targets {
		result, ok := bySerial[s]
		if !ok {
			result = Result{Serial: s, Status: "failed", Error: "Missing verification result"}
		}
		if result.Status != "verified" {
			problem = errors.New("Some boards did not verify. See individual results.")
		}
		results = append(results, result)
	}
	return results, problem
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
func (e *Engine) runDemo(ctx context.Context, r Request, p Plan) ([]Result, error) {
	phases := []string{"Check image", "Compare storage digests", "Write changed blocks", "Verify every sector"}
	if r.Mode == "capture" {
		phases = []string{"Probe storage", "Read Linux storage", "Checksum captured image"}
	}
	if r.Mode == "factory" {
		phases = []string{"Prepare factory image", "Write factory partitions", "Apply GPT patches"}
	}
	for _, phase := range phases {
		for percent := 0; percent <= 100; percent += 10 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(150 * time.Millisecond):
			}
			for i, s := range p.Targets {
				e.event(Event{Kind: "progress", Serial: s, Phase: phase, Percent: float64(max(0, percent-i*3)), Rate: "simulated", ETA: "Demo", Elapsed: "Demo", Message: "Demonstration — no hardware access"})
			}
		}
	}
	var results []Result
	status := "verified"
	if r.Mode == "capture" {
		status = "captured"
	}
	if r.Mode == "factory" {
		status = "flashed"
	}
	for _, s := range p.Targets {
		results = append(results, Result{Serial: s, Status: status, Seconds: float64(len(phases) * 2)})
	}
	return results, nil
}
