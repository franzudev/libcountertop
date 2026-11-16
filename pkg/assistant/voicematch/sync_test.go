package voicematch

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type home struct {
	mu      sync.Mutex
	stored  map[string]Profile
	sent    [][]change
	offline bool
	refused int
}

func (h *home) online(after int) {
	for {
		h.mu.Lock()
		if h.refused >= after {
			h.offline = false
			h.mu.Unlock()
			return
		}
		h.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
}

func (h *home) Ask(
	_ context.Context,
	action string,
	data, _ map[string]string,
	_ string,
) (json.RawMessage, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.offline {
		h.refused++
		return nil, errors.New("not connected")
	}
	if action != exchange {
		return nil, errors.New("unknown action " + action)
	}
	if data["changes"] == "null" {
		return nil, errors.New("changes sent as null, want a list")
	}
	var changes []change
	if err := json.Unmarshal([]byte(data["changes"]), &changes); err != nil {
		return nil, err
	}
	h.sent = append(h.sent, changes)
	for _, c := range changes {
		if c.Remove != "" {
			delete(h.stored, c.Remove)
		}
		if c.Save != nil {
			h.stored[c.Save.ID] = *c.Save
		}
	}
	var out struct {
		Profiles []Profile `json:"profiles"`
	}
	for _, p := range h.stored {
		out.Profiles = append(out.Profiles, p)
	}
	return json.Marshal(out)
}

func synced(t *testing.T, h *home) *Service {
	t.Helper()
	dir := t.TempDir()
	return settled(t, NewService(Options{
		ModelDir: dir,
		Profiles: filepath.Join(dir, "voices.json"),
		Outbox:   filepath.Join(dir, "outbox.json"),
		Home:     h,
	}))
}

func settled(t *testing.T, s *Service) *Service {
	t.Cleanup(s.syncs.Wait)
	return s
}

func ann() Profile {
	return Profile{ID: "a", Name: "Ann", Threshold: Default, Prints: [][]float32{{1, 0}}}
}

func TestChangesMadeOfflineAreSentOnceHomeAssistantAnswers(t *testing.T) {
	h := &home{stored: map[string]Profile{}, offline: true}
	s := synced(t, h)
	if err := s.Save(ann()); err != nil {
		t.Fatal(err)
	}
	if err := s.Sync(context.Background()); err == nil {
		t.Fatal("an offline sync should fail")
	}
	h.online(2)
	again := settled(t, NewService(s.o))
	if err := again.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.stored["a"]; !ok {
		t.Error("a voice saved while offline never reached Home Assistant, even after a restart")
	}
	if err := again.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if last := h.sent[len(h.sent)-1]; len(last) != 0 {
		t.Errorf("changes already delivered were sent again: %+v", last)
	}
}

func TestTheHouseholdListReplacesTheLocalOne(t *testing.T) {
	bo := Profile{ID: "b", Name: "Bo", Threshold: Default, Prints: [][]float32{{0, 1}}}
	h := &home{stored: map[string]Profile{"b": bo}}
	s := synced(t, h)
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := s.Profiles(); len(got) != 1 || got[0].ID != "b" {
		t.Errorf("profiles after sync %+v, want Bo from Home Assistant", got)
	}
}

func TestVoicesEnrolledBeforeSyncingAreSentUpFirst(t *testing.T) {
	dir := t.TempDir()
	o := Options{ModelDir: dir, Profiles: filepath.Join(dir, "voices.json")}
	if err := NewService(o).SetProfiles([]Profile{ann()}); err != nil {
		t.Fatal(err)
	}
	h := &home{stored: map[string]Profile{}}
	o.Outbox, o.Home = filepath.Join(dir, "outbox.json"), h
	s := settled(t, NewService(o))
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := s.Profiles(); len(got) != 1 || got[0].ID != "a" {
		t.Errorf("an older local voice was lost on the first sync: %+v", got)
	}
}

func TestARemovalTravelsToo(t *testing.T) {
	h := &home{stored: map[string]Profile{}}
	s := synced(t, h)
	_ = s.Save(ann())
	_ = s.Sync(context.Background())
	if err := s.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.stored["a"]; ok {
		t.Error("the removal never reached Home Assistant")
	}
}
