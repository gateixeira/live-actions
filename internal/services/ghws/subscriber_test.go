package ghws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gorilla/websocket"
)

func init() {
	// The package logs at Info on connect/disconnect; initialise the global
	// logger so tests don't NPE.
	logger.InitLogger("error")
}

// fakeIngester captures dispatched frames and returns a canned result.
type fakeIngester struct {
	mu     sync.Mutex
	calls  []ingestCall
	result IngestResult
}

type ingestCall struct {
	eventType  string
	deliveryID string
	body       []byte
}

func (f *fakeIngester) Ingest(eventType, deliveryID string, body []byte) IngestResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, ingestCall{eventType, deliveryID, append([]byte(nil), body...)})
	return f.result
}

// TestSubscriber_HappyPath exercises the full create-hook → dial →
// activate → frame → ack → close cycle against an httptest server that
// upgrades to WebSocket and speaks the relay protocol.
func TestSubscriber_HappyPath(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{}
	var (
		wsConnReady = make(chan struct{})
		activated   = make(chan struct{})
		ackCh       = make(chan wsAck, 1)
		deletedCh   = make(chan struct{}, 1)
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/hooks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s on create-hook", r.Method)
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("create-hook Authorization = %q, want %q", got, "Bearer tok")
		}
		// We don't know srv.URL until later; the test patches the response
		// before serving by closing over a pointer to it.
		writeHookResponse(t, w, r.Host)
	})
	mux.HandleFunc("/repos/owner/repo/hooks/1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			close(activated)
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			select {
			case deletedCh <- struct{}{}:
			default:
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "bad", http.StatusBadRequest)
		}
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "tok" {
			t.Errorf("ws Authorization = %q, want %q", got, "tok")
		}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("ws upgrade: %v", err)
			return
		}
		defer c.Close()
		close(wsConnReady)

		// Wait until activate has happened, then push a frame.
		<-activated

		body, _ := json.Marshal(map[string]string{"hello": "world"})
		if err := c.WriteJSON(wsFrame{
			Header: map[string][]string{
				"X-Github-Event":    {"push"},
				"X-Github-Delivery": {"deliv-1"},
			},
			Body: body,
		}); err != nil {
			t.Errorf("write frame: %v", err)
			return
		}

		var ack wsAck
		if err := c.ReadJSON(&ack); err != nil {
			t.Errorf("read ack: %v", err)
			return
		}
		ackCh <- ack

		// Close cleanly so runOnce returns nil and Run exits.
		_ = c.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ing := &fakeIngester{result: IngestResult{Status: 200, Message: "queued"}}
	sub, err := NewSubscriber(Config{
		Token:  "tok",
		Repo:   "owner/repo",
		Events: []string{"*"},
	}, ing)
	if err != nil {
		t.Fatalf("NewSubscriber: %v", err)
	}
	sub.apiBaseOverride = srv.URL
	sub.httpClient = srv.Client()
	sub.initialBackoff = 10 * time.Millisecond
	sub.maxBackoff = 10 * time.Millisecond

	// The handler responds with ws_url pointing at the same host but with
	// the ws:// scheme; patch it in by overriding the response writer.
	hookHostHolder.set(srv.Listener.Addr().String())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- sub.Run(ctx) }()

	select {
	case <-wsConnReady:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ws connection")
	}

	select {
	case ack := <-ackCh:
		if ack.Status != 200 {
			t.Errorf("ack.Status = %d, want 200", ack.Status)
		}
		if string(ack.Body) != "queued" {
			t.Errorf("ack.Body = %q, want %q", string(ack.Body), "queued")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ack")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v, want nil after normal close", err)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("timed out waiting for Run to exit")
	}

	select {
	case <-deletedCh:
	case <-time.After(time.Second):
		t.Error("hook was not deleted on shutdown")
	}

	ing.mu.Lock()
	defer ing.mu.Unlock()
	if len(ing.calls) != 1 {
		t.Fatalf("Ingest called %d times, want 1", len(ing.calls))
	}
	if ing.calls[0].eventType != "push" || ing.calls[0].deliveryID != "deliv-1" {
		t.Errorf("Ingest call = %+v, want push/deliv-1", ing.calls[0])
	}
}

// TestSubscriber_CreateHookMissingWsURL ensures a confused relay response is
// surfaced as a clear error rather than dialing an empty URL.
func TestSubscriber_CreateHookMissingWsURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 7, "url": "https://api.example/hooks/7"}`))
	}))
	defer srv.Close()

	sub, err := NewSubscriber(Config{
		Token:  "tok",
		Repo:   "owner/repo",
		Events: []string{"*"},
	}, &fakeIngester{})
	if err != nil {
		t.Fatalf("NewSubscriber: %v", err)
	}
	sub.apiBaseOverride = srv.URL
	sub.httpClient = srv.Client()

	_, err = sub.createHook(context.Background())
	if err == nil || !strings.Contains(err.Error(), "ws_url") {
		t.Fatalf("createHook err = %v, want message about ws_url", err)
	}
}

// TestConfig_apiBase covers the three GitHub deployment shapes the
// subscriber needs to talk to: public github.com, GHE.com data residency
// subdomains, and GHES instances on arbitrary hostnames.
func TestConfig_apiBase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		host string
		want string
	}{
		{"github.com", "https://api.github.com"},
		{"octocorp.ghe.com", "https://api.octocorp.ghe.com"},
		{"another-tenant.ghe.com", "https://api.another-tenant.ghe.com"},
		{"ghes.example.com", "https://ghes.example.com/api/v3"},
	}
	for _, tc := range cases {
		c := Config{Host: tc.host}
		if got := c.apiBase(); got != tc.want {
			t.Errorf("apiBase(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}

// hookHostHolder lets the create-hook handler embed the test server's host

// in its ws_url response without a package-level race.
type hostHolder struct {
	mu   sync.Mutex
	host string
}

func (h *hostHolder) set(s string) { h.mu.Lock(); h.host = s; h.mu.Unlock() }
func (h *hostHolder) get() string  { h.mu.Lock(); defer h.mu.Unlock(); return h.host }

var hookHostHolder = &hostHolder{}

func writeHookResponse(t *testing.T, w http.ResponseWriter, fallbackHost string) {
	t.Helper()
	host := hookHostHolder.get()
	if host == "" {
		host = fallbackHost
	}
	u := url.URL{Scheme: "ws", Host: host, Path: "/ws"}
	hookURL := "http://" + host + "/repos/owner/repo/hooks/1"
	resp := map[string]any{
		"id":     1,
		"url":    hookURL,
		"ws_url": u.String(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
