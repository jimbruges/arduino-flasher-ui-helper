package provision

import (
	"fmt"
	"math"
)

// Short UX estimates use recorded CLI runs; benchmark details live in the README.
func describeScale(r Request, p *Plan) {
	if r.Mode == "factory" {
		p.DataScale = "Partition sizes available after image preparation"
		p.Timing = "Step ETA shown during transfer"
		return
	}
	bytes := p.Bytes
	if r.Mode == "capture" {
		p.DataScale = "Read from board during capture"
		seconds := 164.0
		if r.Board == "ventunoq" {
			seconds = 1733
		}
		p.Timing = minuteRange(seconds) + " · typical capture"
		return
	}
	if bytes == 0 {
		p.DataScale = "Image size unavailable"
		p.Timing = "Step ETA shown during transfer"
		return
	}
	size := fmt.Sprintf("%.2f GiB", float64(bytes)/(1<<30))
	p.DataScale = size + " per board"
	if len(p.Targets) > 1 {
		p.DataScale += fmt.Sprintf(" · %.2f GiB total", float64(bytes)*float64(len(p.Targets))/(1<<30))
	}
	if p.Strategy != "full" {
		p.DataScale = "Up to " + p.DataScale
	}
	if len(p.Targets) > 1 {
		p.Timing = "No reliable batch estimate yet"
		return
	}
	baseline, low, high := 15675162624.0, 15.0, 35.0
	if r.Board == "ventunoq" {
		baseline, low, high = 63685787648, 35, 50
	}
	if r.TargetContents == "similar" && p.Strategy != "full" {
		low, high = 2, 20
		if r.Board == "ventunoq" {
			low, high = 7, 40
		}
	}
	scale := float64(bytes) / baseline
	p.Timing = fmt.Sprintf("About %.0f–%.0f min", math.Max(1, math.Floor(low*scale)), math.Max(2, math.Ceil(high*scale)))
}
func minuteRange(seconds float64) string {
	return fmt.Sprintf("About %.0f–%.0f min", math.Max(1, math.Floor(seconds*.8/60)), math.Max(2, math.Ceil(seconds*1.3/60)))
}
