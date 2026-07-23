//go:build js && wasm

package attractor

import (
	"math"
	"strconv"
	"syscall/js"

	sg "github.com/0magnet/audioprism-go/pkg/spectrogram"
)

// Audio-feature analysis for modulating the attractors. Each frame (when
// "Audio reactive" is on) we snapshot the latest window from the shared
// audio source, run one FFT, and derive a small set of smoothed, roughly
// 0..1-normalized features. Phase 2 maps these onto ODE params, colors,
// motion, etc. This layer is independent of the spectrogram/skin path and
// works in any mode.

var (
	audioReactive bool

	afWindow  []float32 // reused sample window (sg.FFTSize)
	afPrevMag []float64 // previous frame magnitudes, for onset flux

	// Smoothed features in ~[0,1] (afBeat is a decaying pulse).
	afAmp      float32
	afBass     float32
	afMid      float32
	afTreble   float32
	afCentroid float32
	afBeat     float32

	// Adaptive per-feature peaks for level-independent normalization.
	afPeakAmp    float32
	afPeakBass   float32
	afPeakMid    float32
	afPeakTreble float32
	afPeakFlux   float32

	afOverlay   js.Value
	afMeterFill [6]js.Value // meter bar fill elements
	afFrameCnt  int
)

// afNorm scales x by an adaptive peak that rises instantly and decays
// slowly, yielding a level-independent 0..1 value.
func afNorm(peak *float32, x float32) float32 {
	if x > *peak {
		*peak = x
	} else {
		*peak *= 0.9995
	}
	if *peak < 1e-9 {
		return 0
	}
	r := x / *peak
	if r > 1 {
		r = 1
	}
	return r
}

// afSmooth applies asymmetric attack/decay smoothing (fast rise, slow
// fall) so modulated params respond quickly but settle gently.
func afSmooth(cur, target float32) float32 {
	const attack, release = 0.6, 0.12
	if target > cur {
		return cur + (target-cur)*attack
	}
	return cur + (target-cur)*release
}

// updateAudioFeatures refreshes the smoothed feature set. Cheap (one FFT,
// same as the spectrogram). No-op unless audio-reactive is enabled and the
// source is delivering samples.
func updateAudioFeatures() {
	if !audioReactive {
		return
	}
	src := ensureAudioSource()
	if src == nil || !src.Ready() {
		return
	}
	if afWindow == nil {
		afWindow = make([]float32, sg.FFTSize)
	}
	src.TimeDomain(afWindow)

	// RMS loudness.
	var sq float32
	for _, s := range afWindow {
		sq += s * s
	}
	rms := float32(math.Sqrt(float64(sq / float32(len(afWindow)))))

	mags := sg.ComputeFFT(afWindow)
	sr := 24000
	if src.SampleRate() > 0 {
		sr = src.SampleRate()
	}
	nyq := float64(sr) / 2

	var bass, mid, treble, total, cWeighted, flux float64
	for i, m := range mags {
		f := float64(i) / float64(len(mags)) * nyq
		switch {
		case f < 250:
			bass += m
		case f < 2000:
			mid += m
		default:
			treble += m
		}
		total += m
		cWeighted += f * m
		if afPrevMag != nil && i < len(afPrevMag) {
			if d := m - afPrevMag[i]; d > 0 {
				flux += d
			}
		}
	}
	afPrevMag = mags

	centroid := float32(0)
	if total > 0 {
		centroid = float32(cWeighted / total / nyq) // 0..1
	}

	afAmp = afSmooth(afAmp, afNorm(&afPeakAmp, rms))
	afBass = afSmooth(afBass, afNorm(&afPeakBass, float32(bass)))
	afMid = afSmooth(afMid, afNorm(&afPeakMid, float32(mid)))
	afTreble = afSmooth(afTreble, afNorm(&afPeakTreble, float32(treble)))
	afCentroid = afSmooth(afCentroid, centroid)

	// Onset → decaying beat pulse: fire when normalized flux crosses a
	// threshold and the pulse has mostly decayed (avoids re-triggering).
	fluxN := afNorm(&afPeakFlux, float32(flux))
	if fluxN > 0.55 && afBeat < 0.35 {
		afBeat = 1
	} else {
		afBeat *= 0.86
	}

	updateAudioMeters()
}

// setAudioReactive toggles the feature layer, ensuring the audio source
// exists (which prompts for the mic when using the default backend) and
// showing/hiding the meter overlay.
func setAudioReactive(on bool) {
	audioReactive = on
	if on {
		ensureAudioSource()
		showAudioMeters()
	} else {
		if afOverlay.Truthy() {
			afOverlay.Get("style").Set("display", "none")
		}
		restoreModColors() // undo any modulated colors / point size
	}
}

// showAudioMeters lazily builds a small overlay of labelled bars (amp,
// bass, mid, treble, centroid, beat) at the top-left of the canvas.
func showAudioMeters() {
	if !afOverlay.Truthy() {
		labels := [6]string{"amp", "bass", "mid", "treble", "cntr", "beat"}
		afOverlay = doc.Call("createElement", "div")
		afOverlay.Set("id", "audio-meters")
		st := afOverlay.Get("style")
		st.Set("position", "fixed")
		st.Set("top", "8px")
		st.Set("left", "8px")
		st.Set("padding", "6px 8px")
		st.Set("background", "rgba(0,0,0,0.6)")
		st.Set("font-family", "monospace")
		st.Set("font-size", "10px")
		st.Set("color", "#ccc")
		st.Set("z-index", "40")
		st.Set("pointer-events", "none")
		for i, lab := range labels {
			row := doc.Call("createElement", "div")
			row.Get("style").Set("display", "flex")
			row.Get("style").Set("alignItems", "center")
			row.Get("style").Set("margin", "1px 0")
			name := doc.Call("createElement", "span")
			name.Set("textContent", lab)
			name.Get("style").Set("width", "34px")
			track := doc.Call("createElement", "div")
			track.Get("style").Set("width", "80px")
			track.Get("style").Set("height", "6px")
			track.Get("style").Set("background", "#333")
			fill := doc.Call("createElement", "div")
			fill.Get("style").Set("height", "6px")
			fill.Get("style").Set("width", "0%")
			fill.Get("style").Set("background", "#4caf50")
			track.Call("appendChild", fill)
			row.Call("appendChild", name)
			row.Call("appendChild", track)
			afOverlay.Call("appendChild", row)
			afMeterFill[i] = fill
		}
		body.Call("appendChild", afOverlay)
	}
	afOverlay.Get("style").Set("display", "block")
}

// updateAudioMeters reflects current feature values in the overlay bars,
// throttled to a few updates per second.
func updateAudioMeters() {
	if !afOverlay.Truthy() {
		return
	}
	afFrameCnt++
	if afFrameCnt%6 != 0 {
		return
	}
	vals := [6]float32{afAmp, afBass, afMid, afTreble, afCentroid, afBeat}
	for i, v := range vals {
		if afMeterFill[i].Truthy() {
			afMeterFill[i].Get("style").Set("width", strconv.FormatFloat(float64(v*100), 'f', 0, 64)+"%")
		}
	}
}
