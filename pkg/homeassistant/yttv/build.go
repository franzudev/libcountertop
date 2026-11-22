package yttv

import (
	"github.com/franzudev/libcountertop/pkg/homeassistant"
	"github.com/franzudev/libcountertop/pkg/runtime/product"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Build(p product.Context, settings storage.Section[schema.TV], self func() string) *Guide {
	g := New(Options{
		HA:        p.Controller,
		Connected: p.Controller.Connected,
		States:    p.States,
		Settings:  settings,
		Self:      self,
		Redraw:    p.Redraw,
		DeviceID:  p.Screen,
	})
	p.Settings.Add(g.Settings())
	p.Controller.Watch(func(access homeassistant.Access) {
		if access == homeassistant.Allowed {
			g.ask()
		}
	})
	return g
}
