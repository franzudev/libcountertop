package settings

import (
	"slices"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/libcountertop/pkg/display/menu"
	"github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/say"
	config "github.com/ygelfand/libcountertop/pkg/settings/schema"
)

func (p *BasicPages) Media() *shell.Page {
	return &shell.Page{Title: say.T("settings.media"), Build: func() ([]menu.Row, []func(int)) {
		player, pick := p.playerRow()
		mini, flip := p.miniRow()
		paused, delay := p.pausedRow()
		return []menu.Row{player, mini, paused}, []func(int){pick, flip, delay}
	}}
}

func (p *BasicPages) playerRow() (menu.Row, func(int)) {
	player := p.options.Player
	return menu.Row{
		Glyph: gogui.IconPlay,
		Label: say.T("media.setting.player"),
		Kind:  menu.Chevron,
		Value: player.PlayerStart().Label(),
		Open: func() shell.View {
			return Choose(
				say.T("media.setting.player"),
				config.PlayerStarts(),
				player.PlayerStart,
				player.SetPlayerStart,
				nil,
			)
		},
	}, nil
}

func (p *BasicPages) miniRow() (menu.Row, func(int)) {
	player := p.options.Player
	on := player.Mini()
	return menu.Row{
		Glyph: gogui.IconMinus,
		Label: say.T("media.setting.mini"),
		Kind:  menu.Toggle,
		On:    on,
	}, func(int) { player.SetMini(!on) }
}

func (p *BasicPages) pausedRow() (menu.Row, func(int)) {
	steps := menu.Steps(len(config.MediaDelays()))
	delay := p.state().Idle.Media
	return menu.Row{
		Glyph: gogui.IconPause,
		Label: say.T("idle.media"),
		Hint:  say.T("idle.media.hint"),
		Kind:  menu.Slider,
		Level: steps.Level(slices.Index(config.MediaDelays(), delay)),
		Snap:  steps.Snap,
		Value: delay.Label(),
	}, func(level int) {
		p.options.Media.SetIdle(config.MediaDelays()[steps.Index(level)])
	}
}
