package weather

import (
	"github.com/franzudev/libcountertop/pkg/runtime/product"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Build(
	p product.Context,
	settings storage.Section[schema.Weather],
	home storage.Section[schema.Home],
) *Weather {
	w := New(Options{
		Controller: p.Controller,
		States:     p.States,
		Settings:   settings,
		Home:       home,
		Redraw:     p.Redraw,
		DeviceID:   p.Screen,
	})
	p.Settings.Add(w.Settings())
	return w
}
