//go:build js && wasm

package attractor

import (
	"strconv"
	"strings"
	"syscall/js"

	"github.com/go-gl/mathgl/mgl32"
)

// Permalink: serialize the live state into the URL hash and restore it on
// load, so any tuned view is shareable. The hash is
//
//	#<mode>[&<control>=<v>]...[&p.<param>=<v>]...[&q=x,y,z,w][&m.<param>=<src>~<lvl>]...
//
// Only values that differ from their pristine defaults are written, so a
// clean load stays short. The hash is refreshed via history.replaceState
// (no reload, no history spam). The orientation quaternion is included
// only when the model is held still (auto-rotate off, spin rates 0) so it
// doesn't churn the URL while rotating.

// permaCtl maps a hash key to a DOM control. check = checkbox (else the
// element's .value is used; color inputs are stored without the leading #).
type permaCtl struct {
	key   string
	id    string
	check bool
}

// Ordered so "am" (audio-mod) is applied last — after params and mod
// routing are in place — so its panel rebuild reflects them.
var permaCtls = []permaCtl{
	{"z", "camera-zoom", false},
	{"rx", "rotation-controls-x", false},
	{"ry", "rotation-controls-y", false},
	{"rz", "rotation-controls-z", false},
	{"sp", "speed-slider", false},
	{"tr", "trail-slider", false},
	{"lw", "line-width", false},
	{"g", "gradient-type", false},
	{"cb", "color-base", false},
	{"cm", "color-mid", false},
	{"ct", "color-top", false},
	{"cg", "color-bg", false},
	{"ar", "auto-rotate", true},
	{"pt", "use-points", true},
	{"ps", "persist-trail", true},
	{"gr", "gradient-reverse", true},
	{"am", "audio-mod", true},
}

var (
	permaDefaults = map[string]string{}
	lastPermaHash string
)

func isColorKey(k string) bool { return k == "cb" || k == "cm" || k == "ct" || k == "cg" }

func permaFmt(v float32) string { return strconv.FormatFloat(float64(v), 'g', 6, 32) }

func paramKey(id string) string { return strings.TrimPrefix(id, selectedMode+"-") }

// hashModeToken returns the mode portion of the URL hash (the token before
// the first '&'), or "" if there's no hash.
func hashModeToken() string {
	h := js.Global().Get("location").Get("hash").String()
	if len(h) < 2 {
		return ""
	}
	h = h[1:]
	if i := strings.IndexByte(h, '&'); i >= 0 {
		h = h[:i]
	}
	return h
}

func ctlValue(c permaCtl, el js.Value) string {
	if c.check {
		if el.Get("checked").Bool() {
			return "1"
		}
		return "0"
	}
	v := el.Get("value").String()
	if isColorKey(c.key) {
		v = strings.TrimPrefix(v, "#")
	}
	return v
}

// capturePermaDefaults records the pristine value of each tracked control
// so serialization can omit anything left at its default. Must run before
// applyStateFromHash.
func capturePermaDefaults() {
	permaDefaults = map[string]string{}
	for _, c := range permaCtls {
		el := doc.Call("getElementById", c.id)
		if el.Truthy() {
			permaDefaults[c.key] = ctlValue(c, el)
		}
	}
}

// serializeState builds the hash content (without the leading '#') from the
// current live state.
func serializeState() string {
	var b strings.Builder
	b.WriteString(selectedMode)

	// Controls that differ from their captured defaults.
	for _, c := range permaCtls {
		el := doc.Call("getElementById", c.id)
		if !el.Truthy() {
			continue
		}
		v := ctlValue(c, el)
		if v != permaDefaults[c.key] {
			b.WriteString("&")
			b.WriteString(c.key)
			b.WriteString("=")
			b.WriteString(v)
		}
	}

	// Attractor parameters that differ from their default.
	for _, p := range attractorParams[selectedMode] {
		if *p.Value != p.Def {
			b.WriteString("&p.")
			b.WriteString(paramKey(p.ID))
			b.WriteString("=")
			b.WriteString(permaFmt(*p.Value))
		}
	}

	// Orientation — only when the model is held still, to avoid churn.
	if !autoRotate && cachedRotX == 0 && cachedRotY == 0 && cachedRotZ == 0 {
		q := mgl32.Mat4ToQuat(movMatrix)
		b.WriteString("&q=")
		b.WriteString(permaFmt(q.V[0]) + "," + permaFmt(q.V[1]) + "," + permaFmt(q.V[2]) + "," + permaFmt(q.W))
	}

	// Per-parameter audio-mod routing.
	for _, p := range attractorParams[selectedMode] {
		m := paramMods[p.ID]
		if m.source != "" && m.level != 0 {
			b.WriteString("&m.")
			b.WriteString(paramKey(p.ID))
			b.WriteString("=")
			b.WriteString(m.source + "~" + permaFmt(m.level))
		}
	}
	return b.String()
}

// startPermalinkSync refreshes the URL hash from the live state on a timer,
// writing only when it actually changed.
func startPermalinkSync() {
	lastPermaHash = serializeState()
	js.Global().Call("setInterval", js.FuncOf(func(js.Value, []js.Value) interface{} {
		syncPermalinkNow()
		return nil
	}), 700)
}

// syncPermalinkNow updates the URL hash immediately if the state changed.
func syncPermalinkNow() {
	s := serializeState()
	if s == lastPermaHash {
		return
	}
	lastPermaHash = s
	js.Global().Get("history").Call("replaceState", js.Null(), "", "#"+s)
}

func permaEvent(name string) js.Value { return js.Global().Get("Event").New(name) }

func eventFor(c permaCtl) string {
	if c.check || c.key == "g" {
		return "change"
	}
	return "input"
}

func applyControl(key, val string) {
	for _, c := range permaCtls {
		if c.key != key {
			continue
		}
		el := doc.Call("getElementById", c.id)
		if !el.Truthy() {
			return
		}
		if c.check {
			el.Set("checked", val == "1")
		} else if isColorKey(key) {
			el.Set("value", "#"+val)
		} else {
			el.Set("value", val)
		}
		el.Call("dispatchEvent", permaEvent(eventFor(c)))
		return
	}
}

func applyParam(suffix, val string) {
	el := doc.Call("getElementById", selectedMode+"-"+suffix)
	if !el.Truthy() {
		return
	}
	el.Set("value", val)
	el.Call("dispatchEvent", permaEvent("input"))
}

func applyPose(val string) {
	f := strings.Split(val, ",")
	if len(f) != 4 {
		return
	}
	n := make([]float32, 4)
	for i := 0; i < 4; i++ {
		v, err := strconv.ParseFloat(f[i], 32)
		if err != nil {
			return
		}
		n[i] = float32(v)
	}
	q := mgl32.Quat{W: n[3], V: mgl32.Vec3{n[0], n[1], n[2]}}
	movMatrix = q.Mat4()
	updateModelMatrix()
}

// applyStateFromHash parses the URL hash and applies everything after the
// mode token. Params first, then the mod routing map, then controls (with
// audio-mod last so its panel rebuild reflects the params + routing), then
// the held pose.
func applyStateFromHash() {
	h := js.Global().Get("location").Get("hash").String()
	if len(h) < 2 {
		return
	}
	parts := strings.Split(h[1:], "&")
	if len(parts) < 2 {
		return
	}

	var poseVal, amVal string
	for _, part := range parts[1:] {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch {
		case key == "q":
			poseVal = val
		case key == "am":
			amVal = val
		case strings.HasPrefix(key, "p."):
			applyParam(strings.TrimPrefix(key, "p."), val)
		case strings.HasPrefix(key, "m."):
			sv := strings.SplitN(val, "~", 2)
			if len(sv) == 2 {
				lvl, err := strconv.ParseFloat(sv[1], 32)
				if err == nil {
					paramMods[selectedMode+"-"+strings.TrimPrefix(key, "m.")] = paramMod{source: sv[0], level: float32(lvl)}
				}
			}
		default:
			applyControl(key, val)
		}
	}
	if amVal != "" {
		applyControl("am", amVal) // triggers setAudioMod → panel rebuild
	}
	if poseVal != "" {
		applyPose(poseVal)
	}
}
