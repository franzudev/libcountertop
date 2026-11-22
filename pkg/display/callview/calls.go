package callview

import (
	"log/slog"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/franzudev/libcountertop/pkg/display/peerboard"
	"github.com/franzudev/libcountertop/pkg/display/theme"
	"github.com/franzudev/libcountertop/pkg/display/ui"
	"github.com/franzudev/libcountertop/pkg/network/peer"
	"github.com/franzudev/libcountertop/pkg/say"
)

func (cv *View) callsBoard(at ui.Rect, pal theme.Theme) gogui.View {
	return peerboard.Board{
		UI:      cv.o.UI,
		ID:      "peers",
		Empty:   say.T("call.empty"),
		Actions: cv.actions,
	}.View(at, cv.o.Peers.Peers(), pal)
}

func (cv *View) actions(p peer.Peer) []peerboard.Action {
	info := peerboard.Action{
		Name: "info",
		Icon: gogui.IconInfo,
		Do:   func(gogui.EventCtx) { cv.open(p) },
	}
	if !p.Standing.Trusted() {
		return []peerboard.Action{info}
	}
	out := []peerboard.Action{
		{Name: "audio", Icon: gogui.IconPhone, Do: func(gogui.EventCtx) { cv.dial(p, false) }},
		info,
	}
	if cv.video(p) {
		out = append(out, peerboard.Action{
			Name: "video",
			Icon: gogui.IconVideo,
			Do:   func(gogui.EventCtx) { cv.dial(p, true) },
		})
	}
	return out
}

func (cv *View) video(p peer.Peer) bool { return p.Video() && cv.o.Video }

func (cv *View) dial(p peer.Peer, video bool) {
	if err := cv.o.Calls.Start(p, video); err != nil {
		slog.Warn("call not placed", "to", p.Name, "err", err)
	}
}
