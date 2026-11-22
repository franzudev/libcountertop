package livevisual

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/franzudev/libcountertop/pkg/display/gpu"
	"github.com/franzudev/libcountertop/pkg/display/ui"
	"github.com/franzudev/libcountertop/pkg/display/visual"
	"github.com/franzudev/libcountertop/pkg/display/visualview"
	"github.com/franzudev/libcountertop/pkg/runtime/safe"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
)

const every = time.Second / 60

type Visuals interface {
	Hold() (release func())
	Input() visual.Input
}

type Options struct {
	GPU     Layers
	Display Oriented
	Visuals Visuals
	Kind    string
	Source  schema.Source
	Depth   int
	Showing func() bool
}

type Panel struct {
	o Options

	mu   sync.Mutex
	at   ui.Rect
	stop context.CancelFunc
}

func NewPanel(o Options) *Panel { return &Panel{o: o} }

func (p *Panel) Place(at ui.Rect) {
	if at.W <= 0 || at.H <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.at = at
	if p.stop != nil {
		return
	}
	ctx, stop := context.WithCancel(context.Background())
	p.stop = stop
	safe.Go("live visual "+p.o.Kind, func() { p.run(ctx) })
}

func (p *Panel) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stop != nil {
		p.stop()
		p.stop = nil
	}
}

func (p *Panel) run(ctx context.Context) {
	release := p.o.Visuals.Hold()
	defer release()
	var slot *Slot
	defer func() { slot.Close() }()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		p.mu.Lock()
		at := p.at
		p.mu.Unlock()
		if !p.o.Showing() {
			slot.Close()
			slot = nil
			continue
		}
		if slot != nil && !slot.Fits(at, p.o.Depth) {
			slot.Close()
			slot = nil
		}
		if slot == nil {
			s, err := Open(p.o.GPU, p.o.Kind, at, p.o.Depth, p.o.Depth)
			if errors.Is(err, gpu.ErrNoHelper) {
				continue
			}
			if err != nil {
				slog.Error("live visual: opening the GPU layer", "kind", p.o.Kind, "err", err)
				return
			}
			slot = s
		}
		in := visualview.From(p.o.Visuals.Input(), p.o.Source)
		if err := slot.Shade(p.o.Display, at, at, in); err != nil {
			slog.Error("live visual failed", "kind", p.o.Kind, "alive", slot.Alive(), "err", err)
			if slot.Alive() {
				return
			}
			slot.Close()
			slot = nil
		}
	}
}
