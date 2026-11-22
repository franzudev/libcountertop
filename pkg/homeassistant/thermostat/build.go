package thermostat

import (
	"github.com/franzudev/libcountertop/pkg/runtime/product"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Build(
	p product.Context,
	settings storage.Section[schema.Thermostat],
	unit func() string,
) *Thermostats {
	t := New(Options{
		Controller: p.Controller,
		States:     p.States,
		Settings:   settings,
		DeviceID:   p.Screen,
		Unit:       unit,
	})
	p.Settings.Add(t.Settings())
	return t
}
