package screens

import (
	"github.com/franzudev/libcountertop/pkg/display/menu"
	"github.com/franzudev/libcountertop/pkg/homeassistant/lovelaceview"
	"github.com/franzudev/libcountertop/pkg/tessera/firmwareview"
)

func standby(v *firmwareview.View) func() bool {
	if v == nil {
		return nil
	}
	return v.Rests
}

func lovelaceRows(f *lovelaceview.Feature) func(home, saver bool) ([]menu.Row, []func(int)) {
	if f == nil {
		return nil
	}
	return f.Rows
}

func lovelaceReload(f *lovelaceview.Feature) func() {
	if f == nil {
		return nil
	}
	return f.Refresh
}
