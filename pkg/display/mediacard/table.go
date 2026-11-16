package mediacard

import (
	"github.com/ygelfand/libcountertop/pkg/audio/volume"
	"github.com/ygelfand/libcountertop/pkg/say"
	setting "github.com/ygelfand/libcountertop/pkg/settings"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/settings/storage"
)

type Settings = setting.Controls[schema.Media]

func Table() *setting.Table[schema.Media] {
	return setting.NewTable(
		"media",
		[]setting.Group{volume.MediaGroup},
		[]setting.Setting[schema.Media]{
			setting.Pick(
				setting.Setting[schema.Media]{
					Name:  "player",
					Group: volume.MediaGroup,
					ID:    "player_on_start",
					Icon:  "mdi:play-box-outline",
				},
				schema.PlayerStarts(),
				func(m *schema.Media) *schema.PlayerStart { return &m.Player },
			),
		},
		setting.Messages{Text: say.T, Missing: say.Missing},
	)
}

func NewSettings(at storage.Section[schema.Media], device uint32) *Settings {
	return &Settings{
		Table:  Table(),
		Device: device,
		Read:   at.Read,
		Save: func(s setting.Setting[schema.Media], v string) error {
			return setting.Store(at, s, v)
		},
	}
}
