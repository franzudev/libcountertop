package doorbellview

import (
	"context"
	"log/slog"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/libcountertop/pkg/camera/doorbell"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
	"github.com/ygelfand/libcountertop/pkg/say"
)

// lockMove is what a tap on the live view's lock button does.
type lockMove int

const (
	lockNothing lockMove = iota // the lock is moving: wait for it
	lockArm                     // locked: the first tap asks for a second one
	lockUnlock                  // locked, and armed by a tap moments ago
	lockLock                    // unlocked, open, jammed or unknown: locking is always safe
)

// lockTap decides what a tap does. Unlocking takes two taps, so a stray touch can't open the door;
// locking takes one.
func lockTap(state string, armed bool) lockMove {
	switch state {
	case doorbell.LockLocking, doorbell.LockUnlocking, doorbell.LockOpening:
		return lockNothing
	case doorbell.LockLocked:
		if armed {
			return lockUnlock
		}
		return lockArm
	}
	return lockLock
}

// lockLook is how the lock button looks. asking puts "tap again to unlock" just above it.
type lockLook struct {
	shown  bool
	glyph  string
	tint   gogui.Color
	ink    gogui.Color
	asking bool
}

var (
	lockPlain = gogui.RGBA(0, 0, 0, 0x99)
	lockDim   = gogui.RGBA(0, 0, 0, 0x55)
	lockWarn  = gogui.RGB(0xd3, 0x2f, 0x2f)
	lockArmed = gogui.RGB(0xff, 0xb3, 0x00)
	inkLight  = gogui.RGB(0xff, 0xff, 0xff)
	inkDark   = gogui.RGB(0x21, 0x21, 0x21)
)

func lookOf(state string, known, armed, pending bool) lockLook {
	switch {
	case pending, state == doorbell.LockLocking, state == doorbell.LockUnlocking,
		state == doorbell.LockOpening:
		return lockLook{shown: true, glyph: "mdi:dots-horizontal", tint: lockDim, ink: inkLight}
	case !known:
		return lockLook{shown: true, glyph: "mdi:lock-question", tint: lockPlain, ink: inkLight}
	case state == doorbell.LockLocked && armed:
		return lockLook{
			shown:  true,
			glyph:  "mdi:lock-open-variant",
			tint:   lockArmed,
			ink:    inkDark,
			asking: true,
		}
	case state == doorbell.LockLocked:
		return lockLook{shown: true, glyph: "mdi:lock", tint: lockPlain, ink: inkLight}
	case state == doorbell.LockUnlocked, state == doorbell.LockOpen:
		// Where the door rests is shown by the icon alone; colour is kept for "tap again" and faults.
		return lockLook{shown: true, glyph: "mdi:lock-open-variant", tint: lockPlain, ink: inkLight}
	}
	return lockLook{shown: true, glyph: "mdi:lock-alert", tint: lockWarn, ink: inkLight}
}

// lockButton is called with l.mu held.
func (l *session) lockButton(now time.Time) lockLook {
	if l.lock == nil {
		return lockLook{}
	}
	state, known := l.lock.Get()
	return lookOf(state, known, now.Before(l.armed), l.pending)
}

func (l *session) pressLock() {
	l.mu.Lock()
	if l.lock == nil || l.pending {
		l.mu.Unlock()
		return
	}
	state, _ := l.lock.Get()
	now := time.Now()
	move := lockTap(state, now.Before(l.armed))
	if move == lockNothing {
		l.mu.Unlock()
		return
	}
	l.press++
	press := l.press
	stopTimer(l.armTimer)
	stopTimer(l.waitTimer)
	if move == lockArm {
		l.armed = now.Add(unlockArm)
		l.armTimer = time.AfterFunc(unlockArm, func() { l.disarm(press) })
		l.mu.Unlock()
		l.v.o.Shell.Redraw()
		return
	}
	l.armed, l.pending = time.Time{}, true
	// The lock's own state change clears pending; this covers a lock that never reports back.
	l.waitTimer = time.AfterFunc(lockWait, func() { l.settle(press) })
	l.mu.Unlock()
	l.v.o.Shell.Redraw()
	locked := move == lockLock
	safe.Go("doorbell lock", func() {
		ctx, cancel := context.WithTimeout(context.Background(), lockWait)
		defer cancel()
		err := l.v.o.Bell.SetLock(ctx, l.slot, locked)
		if err == nil {
			return
		}
		slog.Warn("doorbell lock", "slot", l.slot, "locked", locked, "err", err)
		if l.settle(press) {
			l.explain(say.F("doorbell.why.lock", map[string]any{"Why": failure(err)}))
		}
	})
}

// disarm ends the wait for a second tap, which also takes the "tap again" note away, unless a
// newer tap has happened since.
func (l *session) disarm(press int) {
	l.mu.Lock()
	if l.press != press {
		l.mu.Unlock()
		return
	}
	l.armed = time.Time{}
	l.mu.Unlock()
	l.v.o.Shell.Redraw()
}

// settle stops waiting on the lock for tap press, and says whether that tap is still the latest.
func (l *session) settle(press int) bool {
	l.mu.Lock()
	if l.press != press {
		l.mu.Unlock()
		return false
	}
	l.pending = false
	l.mu.Unlock()
	l.v.o.Shell.Redraw()
	return true
}

func stopTimer(t *time.Timer) {
	if t != nil {
		t.Stop()
	}
}
