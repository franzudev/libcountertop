package doorbellview

import (
	"testing"

	"github.com/franzudev/libcountertop/pkg/camera/doorbell"
	"github.com/franzudev/libcountertop/pkg/say"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
)

type memorySection struct{ cfg schema.Doorbell }

func (m *memorySection) Read() schema.Doorbell { return m.cfg }

func (m *memorySection) Update(change func(*schema.Doorbell)) error {
	change(&m.cfg)
	return nil
}

func formRows(t *testing.T, door schema.DoorbellDoor) []string {
	t.Helper()
	var cfg schema.Doorbell
	cfg.Doors[0] = door
	bell := doorbell.New(doorbell.Options{Settings: &memorySection{cfg: cfg}})
	f := &form{v: &View{o: Options{Bell: bell}}, fresh: true, cfg: cfg}
	rows, _ := f.build()
	labels := make([]string, len(rows))
	for i, r := range rows {
		labels[i] = r.Label
	}
	return labels
}

func TestTheLockIsTheDoorFormsLastRow(t *testing.T) {
	for _, door := range []schema.DoorbellDoor{
		{Video: schema.VideoRTSP, Events: schema.FeedAssistant},
		{Video: schema.VideoONVIF, Events: schema.FeedONVIF},
		{Video: schema.VideoAssistant, Events: schema.FeedAssistant},
	} {
		labels := formRows(t, door)
		if len(labels) == 0 || labels[len(labels)-1] != say.T("doorbell.lock") {
			t.Errorf(
				"video %s, events %s: rows %q, want the lock last",
				door.Video,
				door.Events,
				labels,
			)
		}
	}
}
