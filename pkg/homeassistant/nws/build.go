package nws

import (
	"github.com/franzudev/libcountertop/pkg/runtime/product"
	"github.com/franzudev/libcountertop/pkg/runtime/remember"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Build(
	p product.Context,
	settings storage.Section[schema.Weather],
	dismissed remember.Slot[[]string],
) *NWS {
	return New(Options{
		Controller: p.Controller,
		Agent:      p.Agent(),
		Settings:   settings,
		Dismissed:  dismissed,
		Redraw:     p.Redraw,
	})
}
