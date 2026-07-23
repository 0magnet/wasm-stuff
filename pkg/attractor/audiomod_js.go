//go:build js && wasm

package attractor

// Per-parameter audio modulation. Any attractor-specific parameter can be
// individually routed from an audio feature (a channel: L / R / mono, and
// a band) with a signed level. Nothing is modulated by default — the user
// enables it per parameter via the controls that appear under each param
// when "Audio mod" is on.
//
// A routed parameter's value for the integration step is:
//
//	value = base + level * feature * (max - min)
//
// where base is the slider value (still authoritative — audio swings
// around it), level is the signed per-parameter depth, and feature is the
// smoothed 0..1 source. Keep level small for the "very low level" nudges
// these delicate attractors want; negative inverts. Applied only for the
// integration step, then restored, so the sliders never drift.

// paramMod is the per-parameter routing config, keyed by paramDef.ID.
type paramMod struct {
	source string // feature name (audiofeatures_js.go); "" = off
	level  float32
}

var paramMods = map[string]paramMod{}

// modSources is the ordered list offered in the per-parameter source
// dropdown (label, feature-name). "" is the off entry.
var modSources = []struct{ label, name string }{
	{"— off —", ""},
	{"amp", "amp"}, {"bass", "bass"}, {"mid", "mid"}, {"treble", "treble"},
	{"centroid", "centroid"}, {"beat", "beat"},
	{"L amp", "L-amp"}, {"L bass", "L-bass"}, {"L mid", "L-mid"}, {"L treble", "L-treble"}, {"L centroid", "L-centroid"},
	{"R amp", "R-amp"}, {"R bass", "R-bass"}, {"R mid", "R-mid"}, {"R treble", "R-treble"}, {"R centroid", "R-centroid"},
}

type savedParam struct {
	p *float32
	v float32
}

// applyAudioModulation overrides each routed parameter of the current
// attractor for this integration step and returns the saved originals.
func applyAudioModulation(mode string) []savedParam {
	if !audioMod || !isAttractorMode(mode) {
		return nil
	}
	var saved []savedParam
	for _, pd := range attractorParams[mode] {
		m, ok := paramMods[pd.ID]
		if !ok || m.source == "" || m.level == 0 {
			continue
		}
		f := featureByName(m.source)
		base := *pd.Value
		saved = append(saved, savedParam{pd.Value, base})
		*pd.Value = clampF(base+m.level*f*(pd.Max-pd.Min), pd.Min, pd.Max)
	}
	return saved
}

// restoreAudioModulation restores base parameter values after the step.
func restoreAudioModulation(saved []savedParam) {
	for _, s := range saved {
		*s.p = s.v
	}
}

func clampF(x, lo, hi float32) float32 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
