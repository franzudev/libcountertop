package radar

import (
	"github.com/franzudev/libcountertop/pkg/runtime/product"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Build(p product.Context, settings storage.Section[schema.Weather]) *Radar {
	return New(Options{
		Home:     p.Controller.Home,
		Agent:    p.Agent(),
		Settings: settings,
		Redraw:   p.Redraw,
		CacheDir: p.Path("tiles"),
	})
}
