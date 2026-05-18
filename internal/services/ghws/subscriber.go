// Package ghws implements a WebSocket-based webhook transport for live-actions.
//
// It mirrors the protocol used by https://github.com/cli/gh-webhook:
//
//  1. Create a webhook on a repo or org via the REST API with name="cli"
//     and active=false. The response includes a one-time ws_url.
//  2. Dial that ws_url over TLS with Authorization: <token>.
//  3. Activate the webhook so GitHub starts emitting deliveries.
//  4. Read JSON frames of the form {"Header": http.Header, "Body": []byte}
//     from the relay, hand the body off to the local Ingester, and write
//     back a JSON ack frame describing the outcome.
//  5. On disconnect (CloseAbnormalClosure / network error) redial with
//     exponential backoff up to maxBackoff.
//
// The relay endpoint is GitHub-managed and undocumented; this transport is
// best suited to development or to single-instance deployments behind NAT
// where a public HTTP endpoint isn't available. Production-scale ingest
// should still use the HTTP path.
package ghws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gateixeira/live-actions/pkg/metrics"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Ingester is the seam between this transport and the rest of the system.
// In production it is satisfied by *handlers.WebhookHandler.Ingest, but the
// package itself only depends on the function signature so it can be tested
// against a fake.
type Ingester interface {
	Ingest(eventType, deliveryID string, body []byte) IngestResult
}

// IngestResult mirrors handlers.IngestResult to avoid an import cycle.
type IngestResult struct {
	Status  int
	Message string
}

// Config describes a single relay subscription. Either Repo or Org must be
// non-empty; the rest have sensible defaults.
type Config struct {
	// Token is a GitHub PAT or OAuth token with admin:repo_hook (Repo) or
	// admin:org_hook (Org) scope.
	Token string
	// Host is the GitHub API host. Use "github.com" for github.com, or a
	// GHES hostname.
	Host string
	// Repo is "owner/repo". Mutually exclusive with Org.
	Repo string
	// Org is "org-name". Mutually exclusive with Repo.
	Org string
	// Events are the webhook event types to subscribe to. Use ["*"] for all.
	Events []string
	// Secret is the optional webhook signing secret. Currently informational;
	// the transport does not verify per-frame signatures because the relay
	// is already authenticated by Token.
	Secret string
}

// Validate returns an error if the configuration is unusable.
func (c *Config) Validate() error {
	if c.Token == "" {
		return errors.New("GitHub token is required")
	}
	if c.Repo == "" && c.Org == "" {
		return errors.New("either Repo or Org is required")
	}
	if c.Repo != "" && c.Org != "" {
		return errors.New("only one of Repo or Org may be set")
	}
	if len(c.Events) == 0 {
		return errors.New("at least one event type is required")
	}
	if c.Host == "" {
		c.Host = "github.com"
	}
	return nil
}

// apiBase returns the REST API base for the configured host. Tests can
// override this by setting Subscriber.apiBaseOverride.
func (s *Subscriber) apiBase() string {
	if s.apiBaseOverride != "" {
		return s.apiBaseOverride
	}
	return s.cfg.apiBase()
}

// apiBase returns the REST API base for the configured host.
func (c *Config) apiBase() string {
	if c.Host == "github.com" {
		return "https://api.github.com"
	}
	return "https://" + c.Host + "/api/v3"
}

// Subscriber owns the lifecycle of one relay subscription.
type Subscriber struct {
	cfg      Config
	ingester Ingester

	httpClient *http.Client
	dialer     *websocket.Dialer

	// apiBaseOverride, when non-empty, replaces the GitHub REST base URL.
	// Tests use this to point at an httptest server.
	apiBaseOverride string

	// Tunable retry/backoff knobs for tests.
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

// NewSubscriber returns a ready-to-Run subscriber. cfg is validated and
// normalised; an error is returned only if cfg is invalid.
func NewSubscriber(cfg Config, ingester Ingester) (*Subscriber, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Subscriber{
		cfg:            cfg,
		ingester:       ingester,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		dialer:         websocket.DefaultDialer,
		initialBackoff: 1 * time.Second,
		maxBackoff:     60 * time.Second,
	}, nil
}

// Run blocks until ctx is cancelled. It creates a hook, opens the WebSocket,
// reads frames, and dispatches them to Ingester. On disconnect it retries
// with exponential backoff. Each redial creates a fresh hook (the relay
// ws_url is single-use) and the previous hook is best-effort deleted.
func (s *Subscriber) Run(ctx context.Context) error {
	backoff := s.initialBackoff
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := s.runOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			// runOnce only returns nil on a clean close; treat as exit.
			return nil
		}

		logger.Logger.Warn("WebSocket subscriber disconnected; will retry",
			zap.Duration("backoff", backoff),
			zap.Error(err))

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
		if backoff > s.maxBackoff {
			backoff = s.maxBackoff
		}
	}
}

// runOnce performs a single create-hook → dial → read-loop cycle. It returns
// nil only on a websocket.CloseNormalClosure; any other condition is wrapped
// and returned for the caller to back off and retry.
func (s *Subscriber) runOnce(ctx context.Context) error {
	hook, err := s.createHook(ctx)
	if err != nil {
		return fmt.Errorf("create hook: %w", err)
	}
	defer s.deleteHook(context.Background(), hook)

	conn, _, err := s.dialer.DialContext(ctx, hook.WsURL, http.Header{
		"Authorization": []string{s.cfg.Token},
	})
	if err != nil {
		return fmt.Errorf("dial relay: %w", err)
	}
	defer conn.Close()

	if err := s.activateHook(ctx, hook); err != nil {
		return fmt.Errorf("activate hook: %w", err)
	}

	logger.Logger.Info("WebSocket subscriber connected to GitHub relay",
		zap.String("repo", s.cfg.Repo),
		zap.String("org", s.cfg.Org),
		zap.Int("hook_id", hook.ID))

	// Tear the connection down when ctx is cancelled.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()

	for {
		var frame wsFrame
		if err := conn.ReadJSON(&frame); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return nil
			}
			return fmt.Errorf("read frame: %w", err)
		}

		eventType := http.Header(frame.Header).Get("X-GitHub-Event")
		deliveryID := http.Header(frame.Header).Get("X-GitHub-Delivery")
		result := s.ingester.Ingest(eventType, deliveryID, frame.Body)

		metrics.GetRegistry().WebhookEventsTotal.
			WithLabelValues(eventType, "ws_"+wsOutcome(result.Status)).Inc()

		ack := wsAck{
			Status: result.Status,
			Body:   []byte(result.Message),
		}
		if err := conn.WriteJSON(ack); err != nil {
			return fmt.Errorf("write ack: %w", err)
		}
	}
}

// wsFrame matches the JSON shape pushed down by the relay. The relay encodes
// http.Header as a map[string][]string, which decodes cleanly into this type.
type wsFrame struct {
	Header map[string][]string `json:"Header"`
	Body   []byte              `json:"Body"`
}

// wsAck is what we write back per frame. The relay forwards Status to GitHub
// so deliveries appear in the repo's hook log with the right status code.
type wsAck struct {
	Status int                 `json:"Status"`
	Header map[string][]string `json:"Header,omitempty"`
	Body   []byte              `json:"Body,omitempty"`
}

func wsOutcome(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "accepted"
	case status == http.StatusServiceUnavailable:
		return "queue_full"
	default:
		return "rejected"
	}
}

// hookResponse is the subset of the create-hook response we need.
type hookResponse struct {
	ID    int    `json:"id"`
	URL   string `json:"url"`
	WsURL string `json:"ws_url"`
}

// hookConfig is the JSON sub-object for the GitHub hook config.
type hookConfig struct {
	ContentType string `json:"content_type"`
	InsecureSSL string `json:"insecure_ssl"`
	URL         string `json:"url,omitempty"`
	Secret      string `json:"secret,omitempty"`
}

type createHookRequest struct {
	Name   string     `json:"name"`
	Events []string   `json:"events"`
	Active bool       `json:"active"`
	Config hookConfig `json:"config"`
}

func (s *Subscriber) hookPath() string {
	if s.cfg.Org != "" {
		return "/orgs/" + s.cfg.Org + "/hooks"
	}
	return "/repos/" + s.cfg.Repo + "/hooks"
}

// createHook posts a new dev webhook and returns the relay coordinates.
// The hook is created with active=false so we can finish wiring up the
// WebSocket before GitHub starts delivering.
func (s *Subscriber) createHook(ctx context.Context) (*hookResponse, error) {
	body, _ := json.Marshal(createHookRequest{
		Name:   "cli",
		Events: s.cfg.Events,
		Active: false,
		Config: hookConfig{
			ContentType: "json",
			InsecureSSL: "0",
			Secret:      s.cfg.Secret,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.apiBase()+s.hookPath(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	s.applyAuth(req)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create hook: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var hr hookResponse
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		return nil, fmt.Errorf("decode hook response: %w", err)
	}
	if hr.WsURL == "" {
		return nil, errors.New("create hook: response missing ws_url; the relay feature may not be enabled for this account")
	}
	return &hr, nil
}

// activateHook flips the hook's active flag once the WebSocket is connected.
func (s *Subscriber) activateHook(ctx context.Context, hook *hookResponse) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, hook.URL,
		strings.NewReader(`{"active": true}`))
	if err != nil {
		return err
	}
	s.applyAuth(req)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("activate hook: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// deleteHook is best-effort; failures only result in a stale hook in the
// repo's settings, not a delivery problem. Errors are logged at debug level.
func (s *Subscriber) deleteHook(ctx context.Context, hook *hookResponse) {
	if hook == nil || hook.URL == "" {
		return
	}
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(dctx, http.MethodDelete, hook.URL, nil)
	if err != nil {
		logger.Logger.Debug("delete hook: build request", zap.Error(err))
		return
	}
	s.applyAuth(req)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Logger.Debug("delete hook: request failed", zap.Error(err))
		return
	}
	resp.Body.Close()
}

func (s *Subscriber) applyAuth(req *http.Request) {
	// GitHub accepts either Bearer or the bare token; the relay expects the
	// bare token in its Authorization header, so we use the same form here
	// for consistency.
	req.Header.Set("Authorization", s.cfg.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}
