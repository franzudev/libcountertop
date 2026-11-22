package radio

import (
	"github.com/franzudev/libcountertop/pkg/runtime/product"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Build(p product.Context, settings storage.Section[schema.Radio], mac func() string) *Radio {
	return New(Options{
		Controller: p.Controller,
		MAC:        mac,
		Settings:   settings,
		Redraw:     p.Redraw,
	})
}
