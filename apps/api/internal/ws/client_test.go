package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// The DApp keepalive is application-level because browser JavaScript cannot
// send WebSocket protocol ping frames. These tests pin the server half of
// that contract: {"action":"ping"} must come back as an EventPong inside the
// client's heartbeat timeout, otherwise every client tears down a perfectly
// healthy connection once per heartbeat interval.

func TestClient_AppLevelPingIsAnswered(t *testing.T) {
	hub := newTestHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=valid-token"
	conn, _, err := websocket.Dialer{}.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(ClientMessage{Action: ActionPing}); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received Event
	if err := conn.ReadJSON(&received); err != nil {
		t.Fatalf("read pong: %v", err)
	}

	if received.Type != EventPong {
		t.Errorf("expected event %q, got %q", EventPong, received.Type)
	}
}

// A ping must not disturb the client's subscriptions — the heartbeat shares
// the message channel with domain traffic, so it has to be inert.
func TestClient_PingDoesNotAffectSubscriptions(t *testing.T) {
	hub := newTestHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=valid-token"
	conn, _, err := websocket.Dialer{}.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(ClientMessage{Action: ActionSubscribe, Channels: []string{"vault:9"}}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if err := conn.WriteJSON(ClientMessage{Action: ActionPing}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var pong Event
	if err := conn.ReadJSON(&pong); err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if pong.Type != EventPong {
		t.Fatalf("expected pong first, got %q", pong.Type)
	}

	hub.BroadcastEvent(Event{
		Channel: "vault:9",
		Type:    EventBalanceUpdated,
		Data:    map[string]interface{}{"balance": "10.00"},
	})

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received Event
	if err := conn.ReadJSON(&received); err != nil {
		t.Fatalf("read broadcast after ping: %v", err)
	}
	if received.Type != EventBalanceUpdated || received.Channel != "vault:9" {
		t.Errorf("subscription broken by ping: got %q on %q", received.Type, received.Channel)
	}
}

// An unknown action must be ignored rather than closing the connection: a
// newer client speaking a verb this server does not know yet should degrade,
// not disconnect.
func TestClient_UnknownActionIsIgnored(t *testing.T) {
	hub := newTestHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=valid-token"
	conn, _, err := websocket.Dialer{}.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(ClientMessage{Action: "not-a-real-action"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := conn.WriteJSON(ClientMessage{Action: ActionPing}); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received Event
	if err := conn.ReadJSON(&received); err != nil {
		t.Fatalf("connection dropped after unknown action: %v", err)
	}
	if received.Type != EventPong {
		t.Errorf("expected pong, got %q", received.Type)
	}
}
