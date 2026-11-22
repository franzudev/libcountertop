package doorbell

import (
	"github.com/franzudev/libcountertop/pkg/runtime/product"
	setting "github.com/franzudev/libcountertop/pkg/settings"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Build(
	p product.Context,
	settings storage.Section[schema.Doorbell],
	chimes func() []setting.Option,
) *Doorbell {
	d := New(Options{
		Settings: settings,
		Finder:   p.Controller,
		Actions:  p.Controller,
		States:   p.States,
		DeviceID: p.Camera,
		Chimes:   chimes,
	})
	p.Settings.Add(d.Settings())
	return d
}
