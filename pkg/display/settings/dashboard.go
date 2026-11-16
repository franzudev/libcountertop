package settings

import (
	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/libcountertop/pkg/display/menu"
	"github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/say"
)

func (p *BasicPages) LocalRows() ([]menu.Row, []func(int)) {
	cfg := p.state()
	rows := []menu.Row{
		{
			Glyph: gogui.IconClock,
			Label: say.T("display.clock"),
			Kind:  menu.Chevron,
			Value: cfg.Clock.Face.Label(),
			Open:  func() shell.View { return p.options.Clocks.Page() },
		},
	}
	acts := []func(int){nil}
	return rows, acts
}

func (p *BasicPages) CompactClockRow() (menu.Row, func(int)) {
	return menu.Row{
		Glyph: gogui.IconClock,
		Label: say.T("display.clock"),
		Kind:  menu.Chevron,
		Value: p.state().Clock.Face.Label(),
		Open:  func() shell.View { return p.options.Clocks.Compact() },
	}, nil
}

func (p *BasicPages) WeatherRow() (menu.Row, func(int)) {
	weather := p.options.Preferences.IdleState().Weather
	return menu.Row{
		Glyph: gogui.IconSunnyO,
		Label: say.T("weather.title"),
		Kind:  menu.Chevron,
		Value: weatherShown(weather.Dashboard, weather.WeatherLayout),
		Open:  func() shell.View { return p.options.Idle.DashboardWeather() },
	}, nil
}
