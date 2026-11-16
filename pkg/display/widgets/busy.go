package widgets

import (
	"math"
	"sync/atomic"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/libcountertop/pkg/display/icons"
	"github.com/ygelfand/libcountertop/pkg/display/interaction"
	"github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	"github.com/ygelfand/libcountertop/pkg/display/theme"
	"github.com/ygelfand/libcountertop/pkg/display/ui"
)

const (
	busyDots  = 12
	busyShare = 0.42
	busyMost  = 0.8
	busyInset = 0.12
)

var busySpinning atomic.Bool

func (kit *Toolkit) Busy(w *gogui.Window, b shell.Busy, pal theme.Theme) gogui.View {
	vw, vh := w.WindowSize()
	title := gogui.CurrentTheme().TextStyleTitle
	title.Color = Color(pal.Text)
	detail := gogui.CurrentTheme().Cfg.TextStyleDef
	detail.Color = Color(pal.Text)
	wide := max(w.TextWidth(b.Text, title), w.TextWidth(b.Detail, detail))
	d := min(
		max(float32(min(vw, vh))*busyShare, wide*1.3),
		float32(min(vw, vh))*busyMost,
	)
	box := ui.Rect{
		X: int((float32(vw) - d) / 2),
		Y: int((float32(vh) - d) / 2),
		W: int(d),
		H: int(d),
	}
	lead := int(time.Now().UnixNano()/int64(SpinStep)) % busyDots
	ring := busyRing(d, b.Outcome, lead, pal)
	title.Align = gogui.TextAlignCenter
	detail.Align = gogui.TextAlignCenter
	inner := d * (1 - 2*busyInset)
	lines := []gogui.View{}
	if glyph := outcomeGlyph(b.Outcome); glyph != "" {
		mark := IconStyle(Color(outcomeColor(b.Outcome, pal)))
		mark.Family = icons.Family
		mark.Size = style.Reach()
		lines = append(lines, gogui.Label(icons.Glyph(icons.Named(glyph)), mark))
	}
	lines = append(lines, gogui.Text(gogui.TextCfg{
		Text:      b.Text,
		TextStyle: title,
		Mode:      gogui.TextModeWrap,
	}))
	if b.Detail != "" {
		lines = append(lines, gogui.Text(gogui.TextCfg{
			Text:      b.Detail,
			TextStyle: detail,
			Mode:      gogui.TextModeWrap,
		}))
	}
	if len(b.Buttons) > 0 {
		lines = append(lines, kit.busyButtons(w, b.Buttons, inner, pal))
	}
	if b.Outcome == shell.Working {
		busyAgain(w)
	}
	return gogui.Column(gogui.ContainerCfg{
		Sizing:  gogui.FillFill,
		Padding: gogui.NoPadding,
		Color:   gogui.RGBA(0, 0, 0, 0x99),
		OnClick: func(e gogui.EventCtx) { e.Consume() },
		Content: []gogui.View{
			Placed(box, gogui.Column(gogui.ContainerCfg{
				Sizing:  gogui.FillFill,
				Padding: gogui.NoPadding,
				Radius:  gogui.RadiusPx(d / 2),
				Color:   Color(pal.Background),
			})),
			Placed(box, ring),
			Placed(box, gogui.Column(gogui.ContainerCfg{
				Sizing:  gogui.FillFill,
				Padding: gogui.NewPadding(0, d*busyInset, 0, d*busyInset),
				Spacing: gogui.SpacingSmall,
				HAlign:  gogui.HAlignCenter,
				VAlign:  gogui.VAlignMiddle,
				Content: lines,
			})),
		},
	})
}

func (kit *Toolkit) busyButtons(
	w *gogui.Window,
	buttons []shell.BusyButton,
	room float32,
	pal theme.Theme,
) gogui.View {
	st := gogui.CurrentTheme().TextStyleTitle
	tall := style.Reach()
	gap := gogui.CurrentTheme().SpacingLarge
	keys := make([]gogui.View, 0, len(buttons))
	var wide float32
	for _, b := range buttons {
		fill, ink := pal.Background.Blend(pal.Text, 0.12), pal.Text
		if b.Primary {
			fill, ink = pal.Accent, pal.Contrast(pal.Accent)
		}
		st.Color = Color(ink)
		width := KeyWidth(w, b.Label, st, tall) + tall
		wide += width
		keys = append(keys, kit.Pressable(gogui.Row, gogui.ContainerCfg{
			ID:      b.ID,
			Width:   width,
			Height:  tall,
			Sizing:  gogui.FixedFixed,
			Radius:  gogui.RadiusLarge,
			HAlign:  gogui.HAlignCenter,
			VAlign:  gogui.VAlignMiddle,
			Padding: gogui.NoPadding,
			Color:   Color(fill),
			Content: []gogui.View{gogui.Label(b.Label, st)},
		}, func(e gogui.EventCtx) {
			b.Do()
			e.Window.InvalidateLayout()
		}))
	}
	wide += gap * float32(len(keys)-1)
	lay := gogui.Row
	if wide > room {
		lay = gogui.Column
	}
	return lay(gogui.ContainerCfg{
		Sizing:  gogui.FitFit,
		Padding: gogui.NoPadding,
		Spacing: gogui.SpacingLarge,
		HAlign:  gogui.HAlignCenter,
		Content: keys,
	})
}

func busyRing(d float32, o shell.Outcome, lead int, pal theme.Theme) gogui.View {
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		Width: d, Height: d,
		Version: uint64(lead)<<2 | uint64(o),
		OnDraw: func(dc *gogui.DrawContext) {
			c, r, dot := d/2, d*0.44, max(d*0.03, 3)
			for i := range busyDots {
				a := 2 * math.Pi * float64(i) / busyDots
				fill := outcomeColor(o, pal)
				if o == shell.Working {
					age := (lead - i + busyDots) % busyDots
					fill = pal.Background.Blend(pal.Accent, 1-float64(age)/busyDots)
				}
				dc.FilledPolygon(
					interaction.Disc(c+r*float32(math.Sin(a)), c-r*float32(math.Cos(a)), dot),
					Color(fill),
				)
			}
		},
	})
}

func outcomeColor(o shell.Outcome, pal theme.Theme) theme.Color {
	switch o {
	case shell.Succeeded:
		return pal.Success
	case shell.Failed:
		return pal.Danger
	case shell.Warned:
		return pal.Warning
	}
	return pal.Accent
}

func outcomeGlyph(o shell.Outcome) string {
	switch o {
	case shell.Succeeded:
		return "mdi:check-circle-outline"
	case shell.Failed:
		return "mdi:alert-circle-outline"
	case shell.Warned:
		return "mdi:account-multiple-outline"
	}
	return ""
}

func busyAgain(w *gogui.Window) {
	if busySpinning.Swap(true) {
		return
	}
	time.AfterFunc(SpinStep, func() {
		busySpinning.Store(false)
		w.QueueCommand(func(w *gogui.Window) { w.InvalidateLayout() })
	})
}
