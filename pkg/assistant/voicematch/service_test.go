package voicematch

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/franzudev/libcountertop/pkg/audio/capture"
	"github.com/franzudev/libcountertop/pkg/inference/voiceprint"
)

func newService(t *testing.T) *Service {
	t.Helper()
	return NewService(Options{
		Speech:   capture.NewHistory(voiceprint.SampleRate, Span),
		ModelDir: t.TempDir(),
		Profiles: filepath.Join(t.TempDir(), "voices.json"),
	})
}

func TestTheClosestTakeDecides(t *testing.T) {
	ann := Profile{ID: "a", Name: "Ann", Threshold: Default, Prints: [][]float32{{1, 0}, {0, 1}}}
	bo := Profile{ID: "b", Name: "Bo", Threshold: Default, Prints: [][]float32{{0.7, 0.7}}}
	m, ok := best([]float32{0, 1}, []Profile{bo, ann})
	if !ok || m.Profile.ID != "a" || m.Score < 0.99 {
		t.Errorf("a clip matching one of Ann's takes gave %+v %v", m, ok)
	}
}

func TestProfilesSurviveARestart(t *testing.T) {
	s := newService(t)
	want := []Profile{
		{
			ID:        "a",
			Name:      "Ann",
			Person:    "person.ann",
			Threshold: Default,
			Prints:    [][]float32{{1, 0}, {0, 1}},
		},
	}
	if err := s.SetProfiles(want); err != nil {
		t.Fatal(err)
	}
	again := NewService(s.o)
	if got := again.Profiles(); !reflect.DeepEqual(got, want) {
		t.Errorf("after reloading: %+v", got)
	}
}

func TestProfilesActionReplacesTheSet(t *testing.T) {
	s := newService(t)
	_ = s.SetProfiles(
		[]Profile{{ID: "old", Name: "Old", Threshold: Default, Prints: [][]float32{{1}}}},
	)
	action := named(t, s, "voice_profiles")
	call, err := esphome.NewCall(map[string]any{
		"profiles": `[{"id":"a","name":"Ann","prints":[[1,0]],"threshold":0.5},` +
			`{"id":"b","name":"Bo","prints":[[0,1]],"threshold":0.6}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := action.Run(call); err != nil {
		t.Fatal(err)
	}
	if got := s.Profiles(); len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("profiles after the action: %+v", got)
	}
	bad, err := esphome.NewCall(map[string]any{"profiles": "not json"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := action.Run(bad); err == nil {
		t.Error("malformed profiles should be refused")
	}
}

func TestLastSpeakerNamesTheClosestAboveTheThreshold(t *testing.T) {
	s := newService(t)
	_ = s.SetProfiles([]Profile{
		{ID: "a", Name: "Ann", Threshold: Default, Prints: [][]float32{{1, 0}}},
		{ID: "b", Name: "Bo", Threshold: Default, Prints: [][]float32{{0, 1}}},
	})
	at := time.Now()
	var seen []Identified
	s.Identified.Listen(func(i Identified) { seen = append(seen, i) })
	s.heard(at, []int16{7}, []float32{0.1, 1})
	if len(seen) != 1 || !seen[0].At.Equal(at) || !seen[0].Known ||
		seen[0].Match.Profile.ID != "b" {
		t.Errorf("identified %+v, want Bo known at the wake time", seen)
	}
	if got := s.speaker.Get(); got != "Bo" {
		t.Errorf("speaker %q, want Bo", got)
	}
	if got := s.Last(); len(got) != 1 || got[0] != 7 {
		t.Errorf("last clip %v, want the one just matched", got)
	}
	s.heard(at, nil, []float32{1, 1})
	if got := s.speaker.Get(); got != "Ann" && got != "Bo" {
		t.Errorf("an even split above the threshold should still name someone, got %q", got)
	}
	s.heard(at, nil, []float32{-1, -1})
	if got := s.speaker.Get(); got != unknown {
		t.Errorf("speaker %q, want %q", got, unknown)
	}
}

func TestWakeClipKeepsOnlyTheSpeechAndRepeatsIt(t *testing.T) {
	voice := speech(t)[voiceprint.SampleRate*6/10 : voiceprint.SampleRate*16/10]
	window := slices.Concat(
		make([]int16, voiceprint.SampleRate/2),
		voice,
		make([]int16, voiceprint.SampleRate*3/10),
	)
	clip, took, err := wakeClip(window)
	if err != nil {
		t.Fatal(err)
	}
	if len(clip) != voiceprint.ClipSamples {
		t.Fatalf("clip has %d samples, want %d", len(clip), voiceprint.ClipSamples)
	}
	if took < 600*time.Millisecond || took > 1300*time.Millisecond {
		t.Errorf("kept %v of speech from a window with 1 s of it", took)
	}
	kept := int(took * voiceprint.SampleRate / time.Second)
	if !slices.Equal(clip[:kept], clip[kept:2*kept]) {
		t.Error("the speech should repeat to fill the clip")
	}
}

func TestWakeClipRefusesSilence(t *testing.T) {
	if _, _, err := wakeClip(make([]int16, voiceprint.SampleRate*2)); !errors.Is(err, ErrNoSpeech) {
		t.Errorf("silence gave %v, want ErrNoSpeech", err)
	}
}

func TestClaimOnlyWhileEnrolling(t *testing.T) {
	s := newService(t)
	if s.Claim(time.Now()) {
		t.Error("a wake word should start a turn when nobody is enrolling")
	}
	s.claims = make(chan time.Time, 1)
	first, second := s.Claim(time.Now()), s.Claim(time.Now())
	if !first || !second {
		t.Error("while enrolling every wake word belongs to the enrolment")
	}
}

func TestANewVoiceCloseToAnotherIsNoticed(t *testing.T) {
	ann := Profile{ID: "a", Name: "Ann", Threshold: Default, Prints: [][]float32{{1, 0}}}
	bo := Profile{ID: "b", Name: "Bo", Threshold: Default, Prints: [][]float32{{1, 0.2}}}
	cy := Profile{ID: "c", Name: "Cy", Threshold: Default, Prints: [][]float32{{0, 1}}}
	if m, ok := resembles([]float32{1, 0.1}, ann, []Profile{bo, cy}); !ok || m.Profile.ID != "b" {
		t.Errorf("a clip that also clears Bo's threshold %+v %v, want Bo", m, ok)
	}
	if _, ok := resembles([]float32{1, 0}, ann, []Profile{cy}); ok {
		t.Error("a clip far from every other voice should not be noticed")
	}
	if _, ok := resembles([]float32{1, 0}, ann, []Profile{ann}); ok {
		t.Error("recording a voice again should not be compared with its old copy")
	}
}

func TestTheConfirmationTakeDecidesTheOutcome(t *testing.T) {
	ann := Profile{ID: "a", Name: "Ann", Threshold: Default, Prints: [][]float32{{1, 0}}}
	far := Profile{ID: "b", Name: "Bo", Threshold: Default, Prints: [][]float32{{0, 1}}}
	near := Profile{ID: "c", Name: "Cy", Threshold: Default, Prints: [][]float32{{1, 0.05}}}
	oldAnn := Profile{ID: "a", Name: "Ann", Threshold: Default, Prints: [][]float32{{0, 1}}}
	for _, c := range []struct {
		name     string
		print    []float32
		existing []Profile
		want     error
		matched  string
	}{
		{"close to the new voice only", []float32{1, 0.1}, []Profile{far}, nil, "a"},
		{"below its own threshold", []float32{-1, 0}, nil, ErrNotConfirmed, "a"},
		{"another voice is closer", []float32{1, 0.06}, []Profile{near}, ErrSimilar, "c"},
		{"the old copy of the same voice", []float32{1, 0}, []Profile{oldAnn}, nil, "a"},
	} {
		m, err := judge(c.print, ann, c.existing)
		if !errors.Is(err, c.want) || m.Profile.ID != c.matched {
			t.Errorf(
				"%s: %v matched %q, want %v and %q",
				c.name,
				err,
				m.Profile.ID,
				c.want,
				c.matched,
			)
		}
	}
}
