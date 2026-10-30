// Package routeknob is the soft takeover of one of the deck's route knobs, a
// copy of the rule in deck's engine (deck/engine.go, absolute): the knob
// changes the Hive's route only once it has caught up with the route the
// Hive holds, so a knob left somewhere else never jumps the setting.
package routeknob

import "fmt"

// Knob is one knob's takeover state. Its zero value is a knob not yet moved.
type Knob struct {
	prev int  // the stop the knob was last seen at
	seen bool // the knob has been seen at a stop
	hold int  // the Hive's stop the knob last picked up
	held bool // the knob has picked up a stop
	told bool // the knob has said it needs a pickup since it last picked one up
}

// Move is the knob moving to stop pos while the Hive holds stop cur, or -1
// when the Hive's route isn't one of the knob's stops. It says whether the
// move sets the Hive's route to pos, and a prompt for the person, or "".
func (k *Knob) Move(pos, cur int) (act bool, prompt string) {
	prev, had := k.prev, k.seen
	k.prev, k.seen = pos, true
	if !k.held || k.hold != cur {
		if cur < 0 {
			// Hive's route is off the knob's stops: act on first move, no prompt.
			// This is a pickup like any other.
			k.hold, k.held, k.told = pos, true, false
			return true, ""
		}
		crossed := had && (prev-cur)*(pos-cur) < 0
		if pos != cur && !crossed {
			if !k.told {
				k.told = true
				prompt = fmt.Sprintf("the knob is at %d and the Hive has %d: move it there to pick it up", pos, cur)
			}
			return false, prompt
		}
		k.hold, k.held, k.told = cur, true, false
	}
	return pos != cur, ""
}
