package doorbell

import (
	"context"
	"errors"

	"github.com/franzudev/libcountertop/pkg/homeassistant/states"
)

// Lock states Home Assistant reports for a lock entity.
const (
	LockLocked    = "locked"
	LockUnlocked  = "unlocked"
	LockLocking   = "locking"
	LockUnlocking = "unlocking"
	LockOpen      = "open"
	LockOpening   = "opening"
	LockJammed    = "jammed"
)

var (
	errNoLock      = errors.New("doorbell: no lock for this door")
	errNoAssistant = errors.New("doorbell: Home Assistant actions are not available")
)

// LockOf is the lock entity linked to a door, or "" for none.
func (d *Doorbell) LockOf(slot int) string {
	if slot < 0 || slot >= len(d.o.Settings.Read().Doors) {
		return ""
	}
	return d.o.Settings.Read().Doors[slot].Lock
}

// LockState follows the state of the lock linked to a door. It is nil when the door has no
// lock or the device does not follow Home Assistant states.
func (d *Doorbell) LockState(slot int) *states.Value {
	entity := d.LockOf(slot)
	if entity == "" || d.o.States == nil {
		return nil
	}
	return d.o.States.Follow(entity, "")
}

// SetLock locks or unlocks the lock linked to a door through Home Assistant.
func (d *Doorbell) SetLock(ctx context.Context, slot int, locked bool) error {
	entity := d.LockOf(slot)
	if entity == "" {
		return errNoLock
	}
	if d.o.Actions == nil {
		return errNoAssistant
	}
	action := "lock.unlock"
	if locked {
		action = "lock.lock"
	}
	return d.o.Actions.Call(ctx, action, map[string]string{"entity_id": entity})
}
