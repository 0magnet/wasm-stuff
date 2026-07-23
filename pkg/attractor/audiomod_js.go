//go:build js && wasm

package attractor

import (
	"math"
	"syscall/js"

	"github.com/go-gl/mathgl/mgl32"
)

// Phase-2 audio → attractor modulation: a fixed set of mappings from the
// smoothed features (audiofeatures_js.go) onto attractor parameters, each
// with a 0..1 depth. The parameter sliders still set the base value; audio
// swings around it. Everything is a no-op unless audio-reactive is on.
//
//	modSpeed : dt              <- amp      (integration speed)
//	modShape : primary ODE p   <- bass     (shape morph — the "history")
//	modColor : hue/brightness  <- centroid/amp
//	modBeat  : rotation kick + point-size pop <- onset
//	modPump  : per-axis scale  <- bass/mid/treble (anisotropic breathing)
var (
	modSpeed float32 = 0.5
	modShape float32 = 0.5
	modColor float32 = 0.5
	modBeat  float32 = 0.5
	modPump  float32 = 0.4
)

type savedParam struct {
	p *float32
	v float32
}

// applyAudioModulation overrides the current attractor's dt and primary
// chaos param for this integration step and sets modulated colors + point
// size for this frame. Returns the saved originals for restore.
func applyAudioModulation(mode string) []savedParam {
	if !audioReactive || !isAttractorMode(mode) {
		return nil
	}
	params := attractorParams[mode]
	var saved []savedParam
	// dt (params[0]) <- amp: multiplicative, up to ~3.5x when loud.
	if modSpeed > 0 && len(params) > 0 {
		pd := params[0]
		base := *pd.Value
		saved = append(saved, savedParam{pd.Value, base})
		*pd.Value = clampF(base*(1+modSpeed*afAmp*2.5), pd.Min, pd.Max)
	}
	// primary chaos param (params[1]) <- bass: additive toward its max.
	if modShape > 0 && len(params) > 1 {
		pd := params[1]
		base := *pd.Value
		saved = append(saved, savedParam{pd.Value, base})
		*pd.Value = clampF(base+modShape*afBass*(pd.Max-base), pd.Min, pd.Max)
	}
	applyModColors()
	if !uPointSizeLoc.IsUndefined() {
		gl.Call("uniform1f", uPointSizeLoc, float64(2.0+modBeat*afBeat*10.0))
	}
	return saved
}

// restoreAudioModulation puts the overridden parameter vars back to their
// slider (base) values after the integration step.
func restoreAudioModulation(saved []savedParam) {
	for _, s := range saved {
		*s.p = s.v
	}
}

// audioModelMatrix returns the model matrix to upload: movMatrix with an
// audio-driven anisotropic scale (bass=X, mid=Y, treble=Z) folded in when
// the pump is active on an attractor. movMatrix itself stays pure rotation.
func audioModelMatrix() mgl32.Mat4 {
	if !audioReactive || modPump <= 0 || !isAttractorMode(selectedMode) {
		return movMatrix
	}
	sx := 1 + modPump*afBass*0.6
	sy := 1 + modPump*afMid*0.6
	sz := 1 + modPump*afTreble*0.6
	return movMatrix.Mul4(mgl32.Scale3D(sx, sy, sz))
}

// audioBeatSpin returns an extra Y-rotation (radians) to apply this frame
// from the onset pulse, so beats give the model a rotational kick.
func audioBeatSpin() float32 {
	if !audioReactive || modBeat <= 0 {
		return 0
	}
	return modBeat * afBeat * 0.12
}

// applyModColors uploads hue-rotated, amp-brightened versions of the base
// gradient colors for this frame (relative to the picker values).
func applyModColors() {
	if modColor <= 0 {
		return
	}
	hue := modColor * (afCentroid - 0.5) * 0.5  // up to ±0.25 hue turn
	bright := 1 - modColor*0.5 + modColor*afAmp // dim when quiet, up when loud
	setModColor(uBaseColorLoc, baseColor, hue, bright)
	setModColor(uMidColorLoc, midColor, hue, bright)
	setModColor(uTopColorLoc, topColor, hue, bright)
}

// restoreModColors re-uploads the unmodulated picker colors and default
// point size — called when audio-reactive is switched off.
func restoreModColors() {
	if uBaseColorLoc.IsUndefined() {
		return
	}
	gl.Call("useProgram", shaderProgram)
	gl.Call("uniform3f", uBaseColorLoc, float64(baseColor[0]), float64(baseColor[1]), float64(baseColor[2]))
	gl.Call("uniform3f", uMidColorLoc, float64(midColor[0]), float64(midColor[1]), float64(midColor[2]))
	gl.Call("uniform3f", uTopColorLoc, float64(topColor[0]), float64(topColor[1]), float64(topColor[2]))
	if !uPointSizeLoc.IsUndefined() {
		gl.Call("uniform1f", uPointSizeLoc, 2.0)
	}
}

// setModColor uploads one gradient color to its uniform after a hue
// rotation and brightness scale.
func setModColor(loc js.Value, c [3]float32, hueShift, bright float32) {
	h, s, v := rgb2hsv(c[0], c[1], c[2])
	h = mod1(h + hueShift)
	v = clampF(v*bright, 0, 1)
	r, g, b := hsv2rgb(h, s, v)
	gl.Call("uniform3f", loc, float64(r), float64(g), float64(b))
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

func mod1(x float32) float32 {
	x = float32(math.Mod(float64(x), 1))
	if x < 0 {
		x += 1
	}
	return x
}

func rgb2hsv(r, g, b float32) (float32, float32, float32) {
	max := float32(math.Max(float64(r), math.Max(float64(g), float64(b))))
	min := float32(math.Min(float64(r), math.Min(float64(g), float64(b))))
	v := max
	d := max - min
	var s float32
	if max > 0 {
		s = d / max
	}
	var h float32
	if d > 0 {
		switch max {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = (b-r)/d + 2
		default:
			h = (r-g)/d + 4
		}
		h /= 6
	}
	return h, s, v
}

func hsv2rgb(h, s, v float32) (float32, float32, float32) {
	i := float32(math.Floor(float64(h * 6)))
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)
	switch int(i) % 6 {
	case 0:
		return v, t, p
	case 1:
		return q, v, p
	case 2:
		return p, v, t
	case 3:
		return p, q, v
	case 4:
		return t, p, v
	default:
		return v, p, q
	}
}
