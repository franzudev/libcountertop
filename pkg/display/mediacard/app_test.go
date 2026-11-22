package mediacard

import (
	"testing"

	"github.com/franzudev/libcountertop/pkg/display/shell"
	"github.com/franzudev/libcountertop/pkg/media/nowplaying"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
)

func TestTheAppOpensTheCardWithNothingPlaying(t *testing.T) {
	stack := shell.New()
	defer stack.Close()
	c := New(
		Dependencies{
			Player: testPlayer{&testSource{}},
			Idle:   testIdle{},
			Media:  testMedia{},
			Shell:  stack,
		},
	)

	app := c.App()
	if app.ID != schema.AppPlayer || app.Label() == "" || app.Icon == "" {
		t.Fatalf("app %+v", app)
	}
	app.Open()
	if stack.Top() != c.Page() {
		t.Error("the app did not put the card up")
	}
}

func TestTheAppOpensWhatIsPlaying(t *testing.T) {
	stack := shell.New()
	defer stack.Close()
	src := &testSource{now: nowplaying.Now{Playing: true}}
	c := New(
		Dependencies{
			Player: testPlayer{src},
			Idle:   testIdle{},
			Media:  testMedia{schema.PlayerNone},
			Shell:  stack,
		},
	)
	c.Began(src)

	c.App().Open()
	if stack.Top() != c.Page() {
		t.Error("the app did not open the playing card")
	}
}
