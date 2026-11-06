package provision

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var serialPrefix = regexp.MustCompile(`^\[(0x[0-9A-Fa-f]+|image)\]\s*(.*)$`)
var percentPattern = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)%`)
var ratePattern = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?\s*(?:MiB|KiB|GiB)/s)`)
var elapsedPattern = regexp.MustCompile(`elapsed\s+([^|]+)`)
var etaPattern = regexp.MustCompile(`ETA\s*~?([^|]+)`)

func ProgressEvent(line, serial string) Event {
	e := Event{Kind: "log", Serial: serial, Message: line, Time: time.Now()}
	if m := serialPrefix.FindStringSubmatch(line); len(m) > 0 {
		e.Serial = m[1]
		line = m[2]
	}
	if m := percentPattern.FindStringSubmatch(line); len(m) > 0 {
		e.Kind = "progress"
		e.Percent, _ = strconv.ParseFloat(m[1], 64)
	}
	if m := ratePattern.FindStringSubmatch(line); len(m) > 0 {
		e.Rate = m[1]
	}
	if m := elapsedPattern.FindStringSubmatch(line); len(m) > 0 {
		e.Elapsed = strings.TrimSpace(m[1])
	}
	if m := etaPattern.FindStringSubmatch(line); len(m) > 0 {
		e.ETA = strings.TrimSpace(m[1])
	}
	phase := strings.SplitN(line, "—", 2)[0]
	e.Phase = strings.TrimSpace(phase)
	if strings.Contains(line, "VERIFIED") {
		e.Kind = "progress"
		e.Status = "verified"
		e.Phase = "Verified"
		e.Percent = 100
	}
	if strings.Contains(line, "FAILED:") {
		e.Kind = "progress"
		e.Status = "failed"
		e.Phase = "Failed"
	}
	if strings.Contains(line, "CAPTURED") {
		e.Kind = "progress"
		e.Status = "captured"
		e.Percent = 100
		e.Phase = "Captured"
	}
	if strings.HasPrefix(line, "flashed ") {
		e.Kind = "progress"
		e.Phase = "Writing factory partitions"
	}
	if strings.Contains(line, "waiting for transfer progress") || strings.HasPrefix(strings.ToLower(line), "waiting for edl") {
		e.Kind = "progress"
	}
	return e
}
