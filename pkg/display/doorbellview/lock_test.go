package doorbellview

import (
	"testing"
	"time"

	"github.com/franzudev/libcountertop/pkg/camera/doorbell"
	"github.com/franzudev/libcountertop/pkg/display/icons"
	"github.com/franzudev/libcountertop/pkg/display/shell"
)

func TestUnlockingTakesTwoTaps(t *testing.T) {
	if got := lockTap(doorbell.LockLocked, false); got != lockArm {
		t.Errorf("first tap on a locked lock = %v, want lockArm", got)
	}
	if got := lockTap(doorbell.LockLocked, true); got != lockUnlock {
		t.Errorf("second tap on a locked lock = %v, want lockUnlock", got)
	}
}

func TestLockingTakesOneTap(t *testing.T) {
	for _, state := range []string{
		doorbell.LockUnlocked,
		doorbell.LockOpen,
		doorbell.LockJammed,
		"unavailable",
		"unknown",
		"",
	} {
		for _, armed := range []bool{false, true} {
			if got := lockTap(state, armed); got != lockLock {
				t.Errorf("tap on %q (armed %v) = %v, want lockLock", state, armed, got)
			}
		}
	}
}

func TestAMovingLockIgnoresTaps(t *testing.T) {
	for _, state := range []string{doorbell.LockLocking, doorbell.LockUnlocking, doorbell.LockOpening} {
		for _, armed := range []bool{false, true} {
			if got := lockTap(state, armed); got != lockNothing {
				t.Errorf("tap on %q (armed %v) = %v, want lockNothing", state, armed, got)
			}
		}
	}
}

func TestTheButtonShowsTheLocksState(t *testing.T) {
	cases := []struct {
		name    string
		state   string
		known   bool
		armed   bool
		pending bool
		glyph   string
	}{
		{"locked", doorbell.LockLocked, true, false, false, "mdi:lock"},
		{"armed to unlock", doorbell.LockLocked, true, true, false, "mdi:lock-open-variant"},
		{"unlocked", doorbell.LockUnlocked, true, false, false, "mdi:lock-open-variant"},
		{"open", doorbell.LockOpen, true, false, false, "mdi:lock-open-variant"},
		{"locking", doorbell.LockLocking, true, false, false, "mdi:dots-horizontal"},
		{"waiting for a reply", doorbell.LockLocked, true, false, true, "mdi:dots-horizontal"},
		{"jammed", doorbell.LockJammed, true, false, false, "mdi:lock-alert"},
		{"unavailable", "unavailable", true, false, false, "mdi:lock-alert"},
		{"not heard from yet", "", false, false, false, "mdi:lock-question"},
	}
	for _, c := range cases {
		got := lookOf(c.state, c.known, c.armed, c.pending)
		if !got.shown || got.glyph != c.glyph {
			t.Errorf("%s: look = %+v, want glyph %s", c.name, got, c.glyph)
		}
	}
}

func TestArmingTurnsAmberAndAsksForASecondTap(t *testing.T) {
	armed := lookOf(doorbell.LockLocked, true, true, false)
	locked := lookOf(doorbell.LockLocked, true, false, false)
	if armed.tint != lockArmed || armed.ink != inkDark {
		t.Errorf("armed = %+v, want amber with a dark icon", armed)
	}
	if !armed.asking || locked.asking {
		t.Errorf(
			"asking: armed %v, locked %v; only the armed button asks",
			armed.asking,
			locked.asking,
		)
	}
}

func TestLockedAndUnlockedLookAlikeExceptTheIcon(t *testing.T) {
	locked := lookOf(doorbell.LockLocked, true, false, false)
	for _, state := range []string{doorbell.LockUnlocked, doorbell.LockOpen} {
		got := lookOf(state, true, false, false)
		if got.tint != locked.tint || got.ink != locked.ink || got.asking {
			t.Errorf("%q = %+v, want the locked button's colours (%+v)", state, got, locked)
		}
		if got.glyph == locked.glyph {
			t.Errorf("%q shows the locked icon", state)
		}
	}
}

func TestOnlyAFaultIsRed(t *testing.T) {
	for _, state := range []string{doorbell.LockLocked, doorbell.LockUnlocked, doorbell.LockLocking} {
		for _, armed := range []bool{false, true} {
			if got := lookOf(state, true, armed, false); got.tint == lockWarn {
				t.Errorf("%q (armed %v) is red", state, armed)
			}
		}
	}
	if got := lookOf(doorbell.LockJammed, true, false, false); got.tint != lockWarn {
		t.Errorf("jammed tint %v, want red", got.tint)
	}
}

func TestTheLockIconsAreInTheFont(t *testing.T) {
	missing := icons.Named("mdi:no-such-icon-anywhere")
	for _, g := range []string{
		"mdi:lock",
		"mdi:lock-open-variant",
		"mdi:lock-alert",
		"mdi:lock-question",
		"mdi:dots-horizontal",
	} {
		if icons.Named(g) == missing {
			t.Errorf("%s is not in the icon font", g)
		}
	}
}

func lockSession() *session {
	return &session{v: &View{o: Options{Shell: shell.New()}}}
}

// Tap Lock (press 1), the lock reports back, then start an unlock (press 2): press 1's
// 15 s timer must not end the wait of press 2.
func TestAnOldWaitTimerLeavesANewerPressWaiting(t *testing.T) {
	l := lockSession()
	l.press, l.pending = 2, true
	if l.settle(1) {
		t.Error("settle(1) said press 1 is still the latest")
	}
	if !l.pending {
		t.Error("press 1's timer ended press 2's wait")
	}
	if !l.settle(2) || l.pending {
		t.Error("press 2's own timer did not end its wait")
	}
}

// Arm (press 1), unlock, re-lock, arm again (press 4) within 4 s: press 1's timer must not
// disarm press 4.
func TestAnOldArmTimerLeavesANewerArm(t *testing.T) {
	l := lockSession()
	armed := time.Now().Add(unlockArm)
	l.press, l.armed = 4, armed
	l.disarm(1)
	if !l.armed.Equal(armed) {
		t.Error("press 1's timer disarmed press 4")
	}
	l.disarm(4)
	if !l.armed.IsZero() {
		t.Error("press 4's own timer did not disarm it")
	}
}

func TestClosingTheLiveViewStopsTheLockTimers(t *testing.T) {
	l := lockSession()
	fired := make(chan struct{}, 2)
	l.armTimer = time.AfterFunc(time.Hour, func() { fired <- struct{}{} })
	l.waitTimer = time.AfterFunc(time.Hour, func() { fired <- struct{}{} })
	l.end()
	if l.armTimer.Stop() || l.waitTimer.Stop() {
		t.Error("a lock timer was still running after the live view closed")
	}
}
