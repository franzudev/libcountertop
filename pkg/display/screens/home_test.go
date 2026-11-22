package screens

import (
	"testing"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/franzudev/libcountertop/pkg/display/navigation"
	"github.com/franzudev/libcountertop/pkg/display/shell"
	"github.com/franzudev/libcountertop/pkg/homeassistant/lovelaceview"
)

func homeApp(drawsHome *bool) (*App, *lovelaceview.View) {
	home := lovelaceview.NewView(lovelaceview.Options{Name: "home"})
	a := &App{o: Options{Shell: shell.New(), Lovelace: &lovelaceview.Feature{Home: home}}}
	a.nav = &navigation.Nav{For: func(v shell.View) *Screen { return &Screen{View: v} }}
	a.nav.Show(&navigation.Overlay{
		Priority: 1,
		Covers:   func() bool { return true },
		Build: func(*gogui.Window) gogui.View {
			a.homeDrawn = *drawsHome
			return blank()
		},
	})
	return a, home
}

func TestAFrameThatDoesNotDrawTheHomePageHidesIt(t *testing.T) {
	drawsHome := false
	a, home := homeApp(&drawsHome)
	a.o.Shell.Push(&view{covers: true})

	a.root(nil)
	if home.Shown() {
		t.Error("the home page is shown under a screen that covers it")
	}

	drawsHome = true
	a.root(nil)
	if !home.Shown() {
		t.Error("the home page stayed hidden in a frame that draws it")
	}
}

func TestTheHomePageStaysShownWhenItBecomesTheScreenSaver(t *testing.T) {
	drawsHome := true
	a, home := homeApp(&drawsHome)

	a.o.Shell.Push(&saver{view: view{covers: true}})
	if !home.Shown() {
		t.Fatal("the shell change hid the home page before the saver was built")
	}
	a.root(nil)
	if !home.Shown() {
		t.Error("the home page was hidden while it is the screen saver")
	}
}
