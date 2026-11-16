package livevisual

import (
	"time"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	"github.com/ygelfand/libcountertop/pkg/display/gpu"
	"github.com/ygelfand/libcountertop/pkg/display/ui"
	"github.com/ygelfand/libcountertop/pkg/display/visual"
)

type Layers interface {
	OpenAt(w, h, depth int) (*gpu.Layer, error)
}

type Oriented interface {
	Orientation() geometry.Orientation
}

type Slot struct {
	Kind  string
	Base  int
	Depth int

	layer  *gpu.Layer
	vis    visual.Visual
	placed ui.Rect
	rot    geometry.Orientation
	fresh  bool
	last   time.Time
}

func Open(layers Layers, kind string, size ui.Rect, base, depth int) (*Slot, error) {
	l, err := layers.OpenAt(size.W, size.H, depth)
	if err != nil {
		return nil, err
	}
	return &Slot{
		Kind:  kind,
		Base:  base,
		Depth: depth,
		layer: l,
		vis:   visual.New(visual.Kind(kind)),
		fresh: true,
	}, nil
}

func (s *Slot) Close() {
	if s != nil && s.layer != nil {
		_ = s.layer.Close()
		s.layer = nil
	}
}

func (s *Slot) Fits(at ui.Rect, base int) bool {
	return s.layer != nil && s.layer.W == at.W && s.layer.H == at.H && s.Base == base &&
		s.layer.Alive()
}

func (s *Slot) Alive() bool { return s.layer != nil && s.layer.Alive() }

func (s *Slot) Place(d Oriented, at ui.Rect) error {
	rot := d.Orientation()
	if s.placed == at && s.rot == rot {
		return nil
	}
	if err := s.layer.Place(at); err != nil {
		return err
	}
	s.placed, s.rot = at, rot
	return nil
}

func (s *Slot) Shade(d Oriented, placed, area ui.Rect, in visual.Input) error {
	if err := s.Place(d, placed); err != nil {
		return err
	}
	now := time.Now()
	if !s.last.IsZero() {
		in.Dt = now.Sub(s.last)
	}
	s.last = now
	if err := s.vis.Shade(s.layer, s.fresh, area, in); err != nil {
		return err
	}
	s.fresh = false
	return nil
}
