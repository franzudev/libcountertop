package voicematch

import (
	"testing"
)

func TestSaveReplacesTheSameVoiceAndRemoveDropsIt(t *testing.T) {
	s := newService(t)
	ann := Profile{ID: "a", Name: "Ann", Threshold: Default, Prints: [][]float32{{1, 0}}}
	bo := Profile{ID: "b", Name: "Bo", Threshold: Default, Prints: [][]float32{{0, 1}}}
	for _, p := range []Profile{ann, bo, {ID: "a", Name: "Ann", Threshold: Default, Prints: [][]float32{{0.5, 0.5}}}} {
		if err := s.Save(p); err != nil {
			t.Fatal(err)
		}
	}
	got := s.Profiles()
	if len(got) != 2 {
		t.Fatalf("profiles %+v, want Ann once and Bo", got)
	}
	for _, p := range got {
		if p.ID == "a" && p.Prints[0][0] != 0.5 {
			t.Errorf("Ann kept the old takes: %+v", p)
		}
	}
	if err := s.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if got := s.Profiles(); len(got) != 1 || got[0].ID != "b" {
		t.Errorf("after removing Ann: %+v", got)
	}
}

func TestThresholdsOutsideTheRangeAreRefused(t *testing.T) {
	s := newService(t)
	for _, th := range []float64{0, Loosest - 0.01, Strictest + 0.01} {
		p := Profile{ID: "a", Name: "Ann", Threshold: th, Prints: [][]float32{{1, 0}}}
		if err := s.SetProfiles([]Profile{p}); err == nil {
			t.Errorf("threshold %.2f was accepted", th)
		}
	}
}

func TestEachVoiceIsJudgedByItsOwnThreshold(t *testing.T) {
	s := newService(t)
	_ = s.SetProfiles([]Profile{
		{ID: "a", Name: "Ann", Threshold: Loosest, Prints: [][]float32{{1, 0, 0}}},
		{ID: "b", Name: "Bo", Threshold: Strictest, Prints: [][]float32{{0, 1, 0}}},
	})
	s.heard(nil, []float32{0.35, 0.3, 0.89})
	if got := s.speaker.Get(); got != "Ann" {
		t.Errorf("a 0.35 match against Ann at %.2f gave %q, want Ann", Loosest, got)
	}
	s.heard(nil, []float32{0.3, 0.6, 0.74})
	if got := s.speaker.Get(); got != unknown {
		t.Errorf("a 0.60 match against Bo at %.2f gave %q, want %q", Strictest, got, unknown)
	}
	s.heard(nil, []float32{0, 0.8, 0.6})
	if got := s.speaker.Get(); got != "Bo" {
		t.Errorf("a 0.80 match against Bo at %.2f gave %q, want Bo", Strictest, got)
	}
}

func TestCancelWithoutAnEnrolmentClearsTheResult(t *testing.T) {
	s := newService(t)
	var seen []Progress
	s.Changed.Listen(func(p Progress) { seen = append(seen, p) })
	s.update(func(p *Progress) { *p = Progress{ID: "a", Outcome: Unconfirmed} })
	s.Cancel()
	if got := s.Progress(); got.Outcome != 0 {
		t.Errorf("progress after cancel %+v, want none", got)
	}
	if len(seen) != 2 || seen[1].Outcome != 0 {
		t.Errorf("changes %+v, want the result then the clear", seen)
	}
}
