package castreceiver

import (
	"github.com/franzudev/libcountertop/pkg/media/cast/preferences"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
	"github.com/franzudev/libcountertop/pkg/settings/storage"
)

func Preferences(requestIDPrefix string, s storage.Section[schema.Cast]) preferences.Provider {
	return preferences.Provider{
		Prefix: requestIDPrefix,
		Read: func() preferences.Settings {
			return preferences.Settings{YouTube: preferences.YouTube{Skip: s.Read().YouTube.Skip}}
		},
	}
}
