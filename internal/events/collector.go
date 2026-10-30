package events

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	dockerevents "github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"observatory.local/platform/internal/model"
)

type Engine interface {
	Events(context.Context, client.EventsListOptions) client.EventsResult
}
type Storage interface {
	SaveEvent(context.Context, model.Event) error
	LatestEvent(context.Context) (*time.Time, error)
}
type Status struct {
	Connected     bool       `json:"connected"`
	StartedAt     time.Time  `json:"started_at"`
	LastConnected *time.Time `json:"last_connected"`
	LastSaved     *time.Time `json:"last_saved"`
	Interrupted   bool       `json:"interrupted"`
	Message       string     `json:"message"`
}
type Collector struct {
	Engine Engine
	Store  Storage
	Log    *slog.Logger
	mu     sync.RWMutex
	status Status
}

func (c *Collector) Status() Status          { c.mu.RLock(); defer c.mu.RUnlock(); return c.status }
func (c *Collector) update(fn func(*Status)) { c.mu.Lock(); defer c.mu.Unlock(); fn(&c.status) }

// Normalize deliberately excludes command/exec output and arbitrary Docker labels.
func Normalize(m dockerevents.Message) (model.Event, bool) {
	action := string(m.Action)
	switch action {
	case "create", "start", "stop", "restart", "die", "destroy", "kill", "oom", "pause", "unpause", "rename", "health_status: healthy", "health_status: unhealthy", "health_status: starting":
	default:
		return model.Event{}, false
	}
	if m.Type != dockerevents.ContainerEventType || len(m.Actor.ID) != 64 {
		return model.Event{}, false
	}
	if _, err := hex.DecodeString(m.Actor.ID); err != nil {
		return model.Event{}, false
	}
	at := time.Unix(m.Time, 0).UTC()
	if m.TimeNano > 0 {
		at = time.Unix(0, m.TimeNano).UTC()
	}
	if at.Before(time.Now().Add(-7*24*time.Hour)) || at.After(time.Now().Add(time.Minute)) {
		return model.Event{}, false
	}
	attr := m.Actor.Attributes
	bounded := func(v string) string {
		if len(v) > 512 {
			v = v[:512]
		}
		return strings.ToValidUTF8(v, "�")
	}
	e := model.Event{ContainerID: m.Actor.ID, Action: action, At: at, Name: bounded(attr["name"]), Project: bounded(attr["com.docker.compose.project"]), Service: bounded(attr["com.docker.compose.service"]), ExitCode: bounded(attr["exitCode"]), Signal: bounded(attr["signal"])}
	// Hash before PostgreSQL's microsecond timestamp rounding. Event replay is idempotent.
	encoded, _ := json.Marshal(e)
	sum := sha256.Sum256(encoded)
	e.ID = hex.EncodeToString(sum[:])
	return e, true
}
func (c *Collector) Run(ctx context.Context) {
	start := time.Now().UTC()
	c.update(func(s *Status) { s.StartedAt = start; s.Message = "Connecting to Docker events" })
	// Replay overlaps the last durable observation. Docker's replay is limited,
	// so neither reconnection nor startup implies a complete historical record.
	var since time.Time
	for ctx.Err() == nil {
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		latest, err := c.Store.LatestEvent(readCtx)
		cancel()
		if err == nil {
			since = start.Add(-time.Minute)
			if latest != nil {
				since = latest.Add(-time.Second)
			}
			break
		}
		c.failed("Event storage unavailable; retrying", err)
		if !pause(ctx) {
			return
		}
	}
	for ctx.Err() == nil {
		// Periodic renewal also bounds a stalled Docker handshake/read.
		streamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		stream := c.Engine.Events(streamCtx, client.EventsListOptions{Since: since.Format(time.RFC3339Nano), Filters: client.Filters{"type": {"container": true}}})
		connected := time.Now().UTC()
		// The SDK returns after the HTTP handshake, including failed handshakes.
		select {
		case err := <-stream.Err:
			cancel()
			if ctx.Err() != nil {
				return
			}
			c.failed("Docker events unavailable; retrying", err)
			if !pause(ctx) {
				return
			}
			continue
		default:
		}
		c.update(func(s *Status) {
			s.Connected = true
			s.LastConnected = &connected
			s.Message = "Listening for container events"
		})
		failed := false
	streamLoop:
		for {
			select {
			case <-streamCtx.Done():
				break streamLoop
			case err := <-stream.Err:
				if streamCtx.Err() == nil {
					c.failed("Docker event stream interrupted; retrying", err)
					failed = true
				}
				break streamLoop
			case m, ok := <-stream.Messages:
				if !ok {
					c.failed("Docker event stream closed; retrying", nil)
					failed = true
					break streamLoop
				}
				e, ok := Normalize(m)
				if !ok {
					continue
				}
				writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
				err := c.Store.SaveEvent(writeCtx, e)
				writeCancel()
				if err != nil {
					c.failed("Event storage unavailable; retrying", err)
					failed = true
					break streamLoop
				}
				c.update(func(s *Status) { at := e.At; s.LastSaved = &at })
				if e.At.After(since.Add(time.Second)) {
					since = e.At.Add(-time.Second)
				}
			}
		}
		cancel()
		c.update(func(s *Status) { s.Connected = false })
		if failed && !pause(ctx) {
			return
		}
	}
}
func (c *Collector) failed(message string, err error) {
	c.update(func(s *Status) { s.Connected = false; s.Interrupted = true; s.Message = message })
	c.Log.Warn(message, "error", err)
}
func pause(ctx context.Context) bool {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
