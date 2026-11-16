package voicematchview

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/assistant/voicematch"
	"github.com/ygelfand/libcountertop/pkg/display/livevisual"
	"github.com/ygelfand/libcountertop/pkg/display/menu"
	"github.com/ygelfand/libcountertop/pkg/display/navigation"
	"github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
	"github.com/ygelfand/libcountertop/pkg/say"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

const (
	halo        = "halo"
	belowUI     = 1
	peopleLimit = 5 * time.Second
	step        = 0.01
)

var stops = int(math.Round((voicematch.Strictest-voicematch.Loosest)/step)) + 1

type Voices interface {
	Profiles() []voicematch.Profile
	Begin(id, name, person string) error
	Progress() voicematch.Progress
	Cancel()
	Save(voicematch.Profile) error
	Remove(id string) error
}

type Person struct {
	ID   string
	Name string
}

type Options struct {
	UI      *widgets.UI
	Shell   *shell.Shell
	Voices  Voices
	People  func(context.Context) ([]Person, error)
	Wake    func() string
	GPU     livevisual.Layers
	Display livevisual.Oriented
	Visuals livevisual.Visuals
}

type Enrolling struct{ view *View }

func (*Enrolling) Covers() bool { return true }

func (*Enrolling) Wakes() bool { return true }

func (*Enrolling) Timeout() time.Duration { return shell.AppTimeout }

func (e *Enrolling) Close() { e.view.leave() }

type View struct {
	o     Options
	halo  *livevisual.Panel
	enrol Enrolling

	mu     sync.Mutex
	person string
	people []Person
}

func New(o Options) *View {
	v := &View{o: o}
	v.enrol = Enrolling{view: v}
	v.halo = livevisual.NewPanel(livevisual.Options{
		GPU:     o.GPU,
		Display: o.Display,
		Visuals: o.Visuals,
		Kind:    halo,
		Source:  schema.StageListening.Source(),
		Depth:   belowUI,
		Showing: func() bool { return o.Shell.Visible(&v.enrol) },
	})
	return v
}

func (v *View) Screen(sv shell.View) *navigation.Screen {
	if sv != &v.enrol {
		return nil
	}
	return &navigation.Screen{Fixed: true, Clear: true, View: sv, Build: v.build}
}

func (v *View) Page() *shell.Page {
	return &shell.Page{Title: say.T("voicematch.title"), Build: func() ([]menu.Row, []func(int)) {
		var (
			rows []menu.Row
			taps []func(int)
		)
		for _, p := range v.o.Voices.Profiles() {
			rows = append(
				rows,
				menu.Row{Glyph: "mdi:account-voice", Label: p.Name, Kind: menu.Chevron},
			)
			taps = append(taps, func(int) { v.o.Shell.Push(v.voice(p)) })
		}
		rows = append(
			rows,
			menu.Row{Glyph: "mdi:plus", Label: say.T("voicematch.add"), Kind: menu.Chevron},
		)
		taps = append(taps, func(int) { v.o.Shell.Push(v.pick()) })
		return rows, taps
	}}
}

func (v *View) voice(p voicematch.Profile) *shell.Page {
	return &shell.Page{Title: p.Name, Build: func() ([]menu.Row, []func(int)) {
		all := v.o.Voices.Profiles()
		if i := slices.IndexFunc(
			all,
			func(q voicematch.Profile) bool { return q.ID == p.ID },
		); i >= 0 {
			p = all[i]
		}
		steps := menu.Steps(stops)
		return []menu.Row{
			{Glyph: "mdi:microphone", Label: say.T("voicematch.redo"), Kind: menu.Chevron},
			{
				Glyph: "mdi:tune-vertical",
				Label: say.T("voicematch.threshold"),
				Kind:  menu.Slider,
				Level: steps.Level(int(math.Round((p.Threshold - voicematch.Loosest) / step))),
				Snap:  steps.Snap,
				Value: fmt.Sprintf("%.2f", p.Threshold),
			},
			{Glyph: "mdi:delete-outline", Label: say.T("voicematch.remove")},
		}, []func(int){
			func(int) { v.start(p.ID, p.Name, p.Person) },
			func(level int) {
				p.Threshold = voicematch.Loosest + float64(steps.Index(level))*step
				if err := v.o.Voices.Save(p); err != nil {
					slog.Error("saving a voice threshold failed", "voice", p.ID, "err", err)
				}
			},
			func(int) {
				if err := v.o.Voices.Remove(p.ID); err != nil {
					slog.Error("removing a voice failed", "voice", p.ID, "err", err)
					return
				}
				v.o.Shell.Pop()
			},
		}
	}}
}

func (v *View) pick() *shell.Page {
	return &shell.Page{
		Title:  say.T("voicematch.pick"),
		OnOpen: v.fetch,
		Build: func() ([]menu.Row, []func(int)) {
			v.mu.Lock()
			people := v.people
			v.mu.Unlock()
			var (
				rows []menu.Row
				taps []func(int)
			)
			for _, p := range people {
				rows = append(
					rows,
					menu.Row{Glyph: "mdi:account", Label: p.Name, Kind: menu.Chevron},
				)
				taps = append(taps, func(int) { v.start(p.ID, p.Name, p.ID) })
			}
			rows = append(rows, menu.Row{
				Glyph: "mdi:account-plus",
				Label: say.T("voicematch.someone"),
				Kind:  menu.Field,
				Save: func(name string) {
					if name = strings.TrimSpace(name); name != "" {
						v.start(slug(name), name, "")
					}
				},
			})
			taps = append(taps, nil)
			return rows, taps
		},
	}
}

func (v *View) fetch() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), peopleLimit)
		defer cancel()
		people, err := v.o.People(ctx)
		if err != nil {
			slog.Warn("listing people for voice match failed", "err", err)
			return
		}
		v.mu.Lock()
		v.people = people
		v.mu.Unlock()
		v.o.Shell.Redraw()
	}()
}

func (v *View) start(id, name, person string) {
	if err := v.o.Voices.Begin(id, name, person); err != nil {
		slog.Warn("voice enrolment not started", "voice", id, "err", err)
		return
	}
	v.mu.Lock()
	v.person = person
	v.mu.Unlock()
	from := v.o.Shell.Top()
	v.o.Shell.Push(&v.enrol)
	if from != nil {
		v.o.Shell.Remove(from)
	}
}

func (v *View) again() {
	p := v.o.Voices.Progress()
	v.mu.Lock()
	person := v.person
	v.mu.Unlock()
	if err := v.o.Voices.Begin(p.ID, p.Name, person); err != nil {
		slog.Warn("voice enrolment not restarted", "voice", p.ID, "err", err)
	}
}

func (v *View) leave() {
	v.halo.Close()
	if !slices.Contains(v.o.Shell.Views(), shell.View(&v.enrol)) {
		v.o.Voices.Cancel()
	}
}

func slug(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), "_")
}
