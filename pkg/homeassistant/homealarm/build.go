package homealarm

import (
	"github.com/franzudev/libcountertop/pkg/runtime/product"
	setting "github.com/franzudev/libcountertop/pkg/settings"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Build(
	p product.Context,
	settings storage.Section[schema.HomeAlarm],
	chimes func() []setting.Option,
) *Alarm {
	a := New(Options{
		Controller: p.Controller,
		States:     p.States,
		Settings:   settings,
		DeviceID:   p.Screen,
		Chimes:     chimes,
	})
	p.Settings.Add(a.Settings())
	return a
}
