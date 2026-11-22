package photos

import (
	"fmt"

	"github.com/franzudev/libcountertop/pkg/say"
	setting "github.com/franzudev/libcountertop/pkg/settings"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
)

const (
	Group           setting.Group = "Photos"
	GoogleGroup     setting.Group = "Google"
	PhotoprismGroup setting.Group = "Photoprism"
)

const (
	textMost = 1024
	listMost = 4096
)

type (
	Setting  = setting.Setting[schema.Photos]
	Table    = setting.Table[schema.Photos]
	Controls = setting.Controls[schema.Photos]
)

func NewTable() *Table {
	rows := []Setting{
		setting.Pick(
			Setting{Name: "app.every", Group: Group, Icon: "mdi:image-sync"},
			schema.PhotoAppSpans(),
			func(c *schema.Photos) *schema.Delay { return &c.AppEvery },
		),
		setting.Pick(
			Setting{Name: "app.fit", Group: Group, Icon: "mdi:fit-to-screen"},
			schema.PhotoFits(),
			func(c *schema.Photos) *schema.PhotoFit { return &c.AppFit },
		),
		setting.Pick(
			Setting{Name: "app.transition", Group: Group, Icon: "mdi:transition"},
			schema.PhotoTransitions(),
			func(c *schema.Photos) *schema.PhotoTransition { return &c.AppChange },
		),
		setting.Flag(
			Setting{Name: "photoprism.enabled", Group: PhotoprismGroup, Icon: "mdi:image-multiple"},
			func(c *schema.Photos) *bool { return &c.Photoprism.Enabled },
		),
		setting.Words(
			Setting{Name: "photoprism.server", Group: PhotoprismGroup, Icon: "mdi:server"},
			textMost,
			func(c *schema.Photos) *string { return &c.Photoprism.Server },
		),
		setting.Words(
			Setting{Name: "photoprism.key", Group: PhotoprismGroup, Icon: "mdi:key"},
			textMost,
			func(c *schema.Photos) *string { return &c.Photoprism.Key },
		),
		setting.Words(
			Setting{Name: "photoprism.albums", Group: PhotoprismGroup, Icon: "mdi:image-album"},
			listMost,
			func(c *schema.Photos) *string { return &c.Photoprism.Albums },
		),
		setting.Flag(
			Setting{Name: "google.enabled", Group: GoogleGroup, Icon: "mdi:google-photos"},
			func(c *schema.Photos) *bool { return &c.Google.Enabled },
		),
	}
	for i := range schema.GoogleAlbums {
		album := func(c *schema.Photos) *schema.GoogleAlbum { return &c.Google.Albums[i] }
		name := fmt.Sprintf("google.album%d", i+1)
		rows = append(rows,
			setting.Words(
				Setting{Name: name, Group: GoogleGroup, Icon: "mdi:link-variant"},
				textMost,
				func(c *schema.Photos) *string { return &album(c).URL },
			),
			setting.Count(Setting{
				Name: name + ".weight", Group: GoogleGroup, Icon: "mdi:scale-balance",
				Min: 0, Max: schema.GoogleWeightMost, Slider: true,
			}, func(c *schema.Photos) *int { return &album(c).Weight }),
		)
	}
	photoprismKey := rows[len(rows)-2]
	photoprismKey.Kind = setting.Secret
	return setting.NewTable("photos",
		[]setting.Group{Group, GoogleGroup, PhotoprismGroup}, rows,
		setting.Messages{Text: say.T, Missing: say.Missing})
}
