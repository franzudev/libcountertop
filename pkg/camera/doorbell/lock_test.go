package doorbell

import (
	"context"
	"errors"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/homeassistant"
	"github.com/ygelfand/libcountertop/pkg/homeassistant/states"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

type memorySection struct{ cfg schema.Doorbell }

func (m *memorySection) Read() schema.Doorbell { return m.cfg }

func (m *memorySection) Update(change func(*schema.Doorbell)) error {
	change(&m.cfg)
	return nil
}

type call struct {
	action string
	data   map[string]string
}

type recordingCaller struct{ calls []call }

func (r *recordingCaller) Call(_ context.Context, action string, data map[string]string) error {
	r.calls = append(r.calls, call{action, data})
	return nil
}

type recordingFollower struct{ followed []string }

func (r *recordingFollower) Follow(entity, _ string) *states.Value {
	r.followed = append(r.followed, entity)
	return &states.Value{Entity: entity}
}

type domainFinder struct{ asked []string }

func (f *domainFinder) ReolinkDevices(context.Context) ([]homeassistant.ReolinkDevice, error) {
	return nil, nil
}

func (f *domainFinder) Entities(
	_ context.Context,
	filter homeassistant.Filter,
) ([]homeassistant.Entity, error) {
	f.asked = append(f.asked, filter.Domains...)
	return []homeassistant.Entity{
		{ID: "lock.side_door", Name: "Side door"},
		{ID: "lock.front_door", Name: "Front door"},
	}, nil
}

func lockedDoorbell(lock string, o Options) *Doorbell {
	section := &memorySection{}
	section.cfg.Doors[0] = schema.DoorbellDoor{Name: "Front", Camera: "camera.front", Lock: lock}
	o.Settings = section
	return New(o)
}

func TestSetLockUnlocksTheDoorsLock(t *testing.T) {
	caller := &recordingCaller{}
	d := lockedDoorbell("lock.front_door", Options{Actions: caller})

	if err := d.SetLock(context.Background(), 0, false); err != nil {
		t.Fatalf("SetLock: %v", err)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(caller.calls))
	}
	got := caller.calls[0]
	if got.action != "lock.unlock" || got.data["entity_id"] != "lock.front_door" {
		t.Errorf("called %s %v, want lock.unlock on lock.front_door", got.action, got.data)
	}
}

func TestSetLockLocksTheDoorsLock(t *testing.T) {
	caller := &recordingCaller{}
	d := lockedDoorbell("lock.front_door", Options{Actions: caller})

	if err := d.SetLock(context.Background(), 0, true); err != nil {
		t.Fatalf("SetLock: %v", err)
	}
	if got := caller.calls[0].action; got != "lock.lock" {
		t.Errorf("called %s, want lock.lock", got)
	}
}

func TestSetLockRefusesADoorWithoutALock(t *testing.T) {
	caller := &recordingCaller{}
	d := lockedDoorbell("", Options{Actions: caller})

	if err := d.SetLock(context.Background(), 0, false); !errors.Is(err, errNoLock) {
		t.Errorf("err = %v, want errNoLock", err)
	}
	if len(caller.calls) != 0 {
		t.Errorf("a door without a lock called %v", caller.calls)
	}
}

func TestSetLockNeedsHomeAssistantActions(t *testing.T) {
	d := lockedDoorbell("lock.front_door", Options{})

	if err := d.SetLock(context.Background(), 0, false); !errors.Is(err, errNoAssistant) {
		t.Errorf("err = %v, want errNoAssistant", err)
	}
}

func TestSetLockIgnoresASlotOutOfRange(t *testing.T) {
	caller := &recordingCaller{}
	d := lockedDoorbell("lock.front_door", Options{Actions: caller})

	err := d.SetLock(context.Background(), schema.DoorbellSlots, false)
	if !errors.Is(err, errNoLock) {
		t.Errorf("err = %v, want errNoLock", err)
	}
	if len(caller.calls) != 0 {
		t.Errorf("a slot out of range called %v", caller.calls)
	}
}

func TestLockStateFollowsTheDoorsLock(t *testing.T) {
	follower := &recordingFollower{}
	d := lockedDoorbell("lock.front_door", Options{States: follower})

	v := d.LockState(0)
	if v == nil || v.Entity != "lock.front_door" {
		t.Fatalf("LockState = %+v, want a value for lock.front_door", v)
	}
	if len(follower.followed) != 1 || follower.followed[0] != "lock.front_door" {
		t.Errorf("followed %v, want [lock.front_door]", follower.followed)
	}
}

func TestLockStateIsNilWithoutALock(t *testing.T) {
	follower := &recordingFollower{}
	d := lockedDoorbell("", Options{States: follower})

	if v := d.LockState(0); v != nil {
		t.Errorf("LockState = %+v, want nil", v)
	}
	if len(follower.followed) != 0 {
		t.Errorf("a door without a lock followed %v", follower.followed)
	}
}

func TestLocksListsHomeAssistantLocksByName(t *testing.T) {
	finder := &domainFinder{}
	d := lockedDoorbell("", Options{Finder: finder})

	got, err := d.Locks(context.Background())
	if err != nil {
		t.Fatalf("Locks: %v", err)
	}
	if len(finder.asked) != 1 || finder.asked[0] != "lock" {
		t.Errorf("asked Home Assistant for %v, want [lock]", finder.asked)
	}
	if len(got) != 2 || got[0].Name != "Front door" || got[1].Name != "Side door" {
		t.Errorf("Locks = %+v, want Front door then Side door", got)
	}
}
