package calendar

import (
	"github.com/franzudev/libcountertop/pkg/runtime/product"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Build(p product.Context, settings storage.Section[schema.Calendar]) *Calendar {
	c := New(Options{
		Controller: p.Controller,
		States:     p.States,
		Settings:   settings,
		Redraw:     p.Redraw,
		DeviceID:   p.Screen,
	})
	p.Settings.Add(c.Settings())
	return c
}
