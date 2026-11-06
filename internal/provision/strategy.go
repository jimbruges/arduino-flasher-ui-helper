package provision

import "fmt"

// Strategy selection is a benchmark-informed policy, not a hardware speed probe.
// Batch size means simultaneously active targets, after source exclusion.
func selectStrategy(r Request, targets int, caps Capabilities) (key, label, reason string, automatic bool) {
	automatic = r.Strategy == "auto" || r.Strategy == "recommended"
	if r.Mode == "factory" {
		return "factory", "Factory partition installation", "Prepare the official image once and write its declared partitions to each target; golden comparison does not apply to factory archives.", true
	}
	if r.Mode == "capture" {
		return "capture", "Complete storage capture", "Read the source storage once and calculate the captured image checksums.", true
	}
	if !automatic {
		key = r.Strategy
		reason = "Manual override. The automatic recommendation is not used."
	} else {
		active := min(r.Jobs, targets)
		similar := r.TargetContents == "similar"
		switch {
		case !caps.Digest && similar:
			key = "readback"
			reason = "These boards are expected to match much of the image. Without the digest helper, compare using USB readback and skip unchanged regions."
		case !caps.Digest:
			key = "full"
			reason = "The digest helper is unavailable. Write once and verify by readback, avoiding a second full USB read solely to compare factory or unknown contents."
		case similar:
			key = "hierarchical"
			reason = "These boards are expected to be close to the golden image. Hierarchical digests can match large ranges without requesting every small block."
		case r.Board == "ventunoq" && active == 1:
			key = "full"
			reason = "For one active VENTUNO Q with factory or unknown contents, the recorded full-write digest run took 36m47s versus 45m46s for a factory partial restore. Avoid the pre-comparison passes."
		case r.Board == "ventunoq" && !caps.Blocks:
			key = "full"
			reason = "This CLI lacks single-pass block comparison. Use a full write with digest verification to avoid the costly hierarchical factory comparison observed on VENTUNO Q."
		case active > 1 && caps.Blocks:
			key = "blocks"
			reason = fmt.Sprintf("For %d active targets with factory or unknown contents, compare each 64 MiB block in one pass, then share USB bandwidth only among changed writes. This avoids hierarchical refinement passes; the fastest batch policy still needs a representative hardware benchmark.", active)
		default:
			key = "hierarchical"
			reason = "Use the tested UNO Q hierarchical digest path to skip matching ranges, then verify every captured sector. Single-pass comparison is unavailable or only one board is active."
		}
	}
	switch key {
	case "full":
		label = "Full write + digest verification"
		if !caps.Digest {
			label = "Full write + readback verification"
		}
	case "blocks":
		label = "Single-pass block digests + changed writes"
	case "hierarchical":
		label = "Hierarchical digests + changed writes"
	case "readback":
		label = "USB readback comparison + changed writes"
	}
	return
}
