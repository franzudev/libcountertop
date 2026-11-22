package mediacard

import (
	"testing"

	"github.com/franzudev/libcountertop/pkg/display/shell"
	"github.com/franzudev/libcountertop/pkg/media/nowplaying"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
)

type testSource struct {
	now nowplaying.Now
	own bool
}

func (*testSource) Play()                 {}
func (*testSource) Pause()                {}
func (*testSource) Stop()                 {}
func (*testSource) Next()                 {}
func (*testSource) Previous()             {}
func (s *testSource) Now() nowplaying.Now { return s.now }
func (*testSource) Kind() nowplaying.Kind { return nowplaying.FromCast }
func (*testSource) Label() string         { return "" }
func (s *testSource) Open() bool          { return s.own }

type testPlayer struct{ source *testSource }

func (p testPlayer) Now() nowplaying.Now       { return p.source.Now() }
func (p testPlayer) Holder() nowplaying.Source { return p.source }
func (testPlayer) ResetSession()               {}

type testIdle struct{}

func (testIdle) Read() schema.Idle               { return schema.Idle{} }
func (testIdle) Update(func(*schema.Idle)) error { return nil }

type testMedia struct{ player schema.PlayerStart }

func (m testMedia) Read() schema.Media             { return schema.Media{Player: m.player} }
func (testMedia) Update(func(*schema.Media)) error { return nil }

type started struct {
	card  *Card
	stack *shell.Shell
	src   *testSource
}

func start(t *testing.T, player schema.PlayerStart, video bool) *started {
	t.Helper()
	s := &started{
		stack: shell.New(),
		src:   &testSource{now: nowplaying.Now{Playing: true}, own: video},
	}
	t.Cleanup(s.stack.Close)
	s.card = New(Dependencies{
		Player: testPlayer{s.src},
		Idle:   testIdle{},
		Media:  testMedia{player},
		Shell:  s.stack,
	})
	s.card.Began(s.src)
	s.card.Follow()
	return s
}

func TestAudioStartFollowsThePlayerSetting(t *testing.T) {
	cases := []struct {
		player schema.PlayerStart
		card   bool
	}{
		{schema.PlayerFull, true},
		{"", true},
		{schema.PlayerNone, false},
	}
	for _, tc := range cases {
		s := start(t, tc.player, false)
		if got := s.stack.Top() == s.card.Page(); got != tc.card {
			t.Errorf("%q: card shown %v, want %v", tc.player, got, tc.card)
		}
	}
}

func TestVideoStartIgnoresThePlayerSetting(t *testing.T) {
	for _, player := range schema.PlayerStarts() {
		s := start(t, player, true)
		if s.stack.Open() {
			t.Errorf("%q: video start held the card", player)
		}
	}
}

func TestOpenShowsTheFullCardWhateverTheSetting(t *testing.T) {
	s := start(t, schema.PlayerNone, false)
	s.card.Open()
	if s.stack.Top() != s.card.Page() {
		t.Fatal("Open did not show the card")
	}
}

func TestSourcePageReleasesTheCardAndCloseReleasesTheHold(t *testing.T) {
	stack := shell.New()
	defer stack.Close()
	src := &testSource{now: nowplaying.Now{Playing: true}}
	c := New(
		Dependencies{
			Player: testPlayer{src},
			Idle:   testIdle{},
			Media:  testMedia{schema.PlayerFull},
			Shell:  stack,
		},
	)
	c.Began(src)
	c.Follow()
	if stack.Top() != c.Page() {
		t.Fatal("new source did not open its card")
	}
	src.own = true
	c.Open()
	if stack.Open() {
		t.Fatal("source page left the old card held")
	}
	src.own = false
	c.Open()
	c.Close()
	if stack.Open() {
		t.Fatal("card remained held after Close")
	}
}
