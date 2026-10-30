package logs

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

const (
	DefaultLimit = 500
	MaxLimit     = 1000
	MaxTextBytes = 512 * 1024
	MaxWireBytes = 1024 * 1024
	Timeout      = 8 * time.Second
)

type Entry struct {
	Timestamp *time.Time `json:"timestamp"`
	Stream    string     `json:"stream"`
	Message   string     `json:"message"`
	Partial   bool       `json:"partial"`
}
type Snapshot struct {
	ContainerID      string    `json:"container_id"`
	From             time.Time `json:"from"`
	To               time.Time `json:"to"`
	FetchedAt        time.Time `json:"fetched_at"`
	Entries          []Entry   `json:"entries"`
	Limit            int       `json:"limit"`
	LimitReached     bool      `json:"limit_reached"`
	Truncated        bool      `json:"truncated"`
	TruncationReason string    `json:"truncation_reason,omitempty"`
	TTY              bool      `json:"tty"`
	TextBytes        int       `json:"text_bytes"`
	MaxTextBytes     int       `json:"max_text_bytes"`
	Source           string    `json:"source"`
	LoggingDriver    string    `json:"logging_driver"`
}
type Engine interface {
	ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	ContainerLogs(context.Context, string, client.ContainerLogsOptions) (client.ContainerLogsResult, error)
}
type Service struct{ Engine Engine }
type Failure struct {
	Code string
	Err  error
}

func (e *Failure) Error() string { return e.Code }
func (e *Failure) Unwrap() error { return e.Err }
func classify(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errdefs.IsNotFound(err) {
		return &Failure{Code: "container_not_found", Err: err}
	}
	lower := strings.ToLower(err.Error())
	if errdefs.IsNotImplemented(err) || strings.Contains(lower, "does not support reading") || strings.Contains(lower, "configured logging driver does not support") {
		return &Failure{Code: "logs_unsupported", Err: err}
	}
	return &Failure{Code: "docker_unavailable", Err: err}
}
func (s *Service) Snapshot(ctx context.Context, id string, from, to time.Time, limit int) (Snapshot, error) {
	result := Snapshot{ContainerID: id, From: from, To: to, Limit: limit, Source: "docker", MaxTextBytes: MaxTextBytes, Entries: make([]Entry, 0)}
	if limit < 1 || limit > MaxLimit || !from.Before(to) {
		return result, &Failure{Code: "invalid_query", Err: errors.New("invalid snapshot query")}
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	inspection, err := s.Engine.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return result, classify(err)
	}
	if inspection.Container.Config == nil {
		return result, &Failure{Code: "invalid_docker_response", Err: errors.New("missing container configuration")}
	}
	result.TTY = inspection.Container.Config.Tty
	if inspection.Container.HostConfig != nil {
		result.LoggingDriver = inspection.Container.HostConfig.LogConfig.Type
	}
	if result.LoggingDriver == "none" {
		return result, &Failure{Code: "logs_unsupported", Err: errors.New("logging driver is none")}
	}
	response, err := s.Engine.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true, ShowStderr: true, Since: from.Format(time.RFC3339Nano), Until: to.Format(time.RFC3339Nano),
		Timestamps: true, Follow: false, Tail: itoa(limit),
	})
	if err != nil {
		return result, classify(err)
	}
	defer response.Close()
	decoded, err := decode(ctx, response, result.TTY, limit)
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, &Failure{Code: "invalid_docker_response", Err: err}
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	result.Entries = decoded.entries
	result.TextBytes = decoded.textBytes
	result.Truncated = decoded.truncated
	result.TruncationReason = decoded.reason
	result.LimitReached = len(result.Entries) >= limit
	result.FetchedAt = time.Now().UTC()
	return result, nil
}
