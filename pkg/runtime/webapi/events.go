package webapi

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/franzudev/libcountertop/pkg/runtime/component"
)

const (
	queueMost = 256
	pingEvery = 30 * time.Second
	writeWait = 10 * time.Second
	readMost  = 4 << 10
)

type event struct {
	Type   string          `json:"type"`
	State  *esphome.State  `json:"state,omitempty"`
	States []esphome.State `json:"states,omitempty"`
	Event  *fired          `json:"event,omitempty"`
}

type fired struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

type Fired interface {
	Listen(fn func(component.Event)) (cancel func())
}

var upgrader = websocket.Upgrader{}

func (a *API) events(w http.ResponseWriter, r *http.Request) {
	if a.o.States == nil {
		http.Error(w, "no states yet", http.StatusServiceUnavailable)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	queue := make(chan event, queueMost)
	full := make(chan struct{})
	var overflow sync.Once
	push := func(e event) {
		select {
		case queue <- e:
		default:
			overflow.Do(func() { close(full) })
		}
	}
	defer a.o.States.Listen(func(s esphome.State) { push(event{Type: "state", State: &s}) })()
	if a.o.Fired != nil {
		defer a.o.Fired.Listen(func(f component.Event) {
			push(event{Type: "event", Event: &fired{Name: f.Name, Data: f.Data}})
		})()
	}

	gone := make(chan struct{})
	conn.SetReadLimit(readMost)
	go func() {
		defer close(gone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	if !a.send(conn, event{Type: "states", States: a.o.States.States()}) {
		return
	}
	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	for {
		select {
		case e := <-queue:
			if !a.send(conn, e) {
				return
			}
		case <-ping.C:
			if err := conn.WriteControl(
				websocket.PingMessage, nil, time.Now().Add(writeWait),
			); err != nil {
				return
			}
		case <-full:
			slog.Info("closing a slow events client", "from", remote(r))
			return
		case <-gone:
			return
		}
	}
}

func (a *API) send(conn *websocket.Conn, e event) bool {
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteJSON(e) == nil
}
