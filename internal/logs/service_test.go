package logs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error { b.closed = true; return nil }

type fakeEngine struct {
	tty                 bool
	driver              string
	inspectErr, logsErr error
	options             client.ContainerLogsOptions
	body                *trackingBody
}

func (e *fakeEngine) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{Container: container.InspectResponse{Config: &container.Config{Tty: e.tty}, HostConfig: &container.HostConfig{LogConfig: container.LogConfig{Type: e.driver}}}}, e.inspectErr
}
func (e *fakeEngine) ContainerLogs(_ context.Context, _ string, options client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	e.options = options
	return e.body, e.logsErr
}
func TestSnapshotUsesWindowAndClosesResponse(t *testing.T) {
	engine := &fakeEngine{driver: "json-file", body: &trackingBody{Reader: bytes.NewReader(frame(stdcopy.Stdout, "2026-10-24T12:00:00Z hello\n"))}}
	service := &Service{Engine: engine}
	to := time.Now().UTC()
	from := to.Add(-time.Hour)
	got, err := service.Snapshot(context.Background(), strings.Repeat("a", 64), from, to, 500)
	if err != nil || len(got.Entries) != 1 || got.Entries[0].Message != "hello" || !engine.body.closed {
		t.Fatalf("snapshot failed: %+v %v", got, err)
	}
	o := engine.options
	if o.Follow || !o.ShowStdout || !o.ShowStderr || !o.Timestamps || o.Tail != "500" || o.Since != from.Format(time.RFC3339Nano) || o.Until != to.Format(time.RFC3339Nano) {
		t.Fatalf("unsafe or mismatched Docker options: %+v", o)
	}
	if got.Source != "docker" || got.LoggingDriver != "json-file" || got.FetchedAt.IsZero() {
		t.Fatal("snapshot provenance missing")
	}
}
func TestSnapshotFailures(t *testing.T) {
	to := time.Now()
	from := to.Add(-time.Hour)
	for _, test := range []struct {
		name, code string
		engine     fakeEngine
	}{
		{"removed", "container_not_found", fakeEngine{inspectErr: errdefs.ErrNotFound}},
		{"disabled", "logs_unsupported", fakeEngine{driver: "none"}},
		{"unsupported", "logs_unsupported", fakeEngine{logsErr: errors.New("configured logging driver does not support reading")}},
		{"offline", "docker_unavailable", fakeEngine{inspectErr: errors.New("connection refused")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := (&Service{Engine: &test.engine}).Snapshot(context.Background(), "id", from, to, 500)
			var failure *Failure
			if !errors.As(err, &failure) || failure.Code != test.code {
				t.Fatalf("wrong classification: %v", err)
			}
		})
	}
}
func TestSnapshotCallerCancellationClosesBody(t *testing.T) {
	engine := &fakeEngine{body: &trackingBody{Reader: strings.NewReader("")}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&Service{Engine: engine}).Snapshot(ctx, "id", time.Now().Add(-time.Hour), time.Now(), 500)
	if !errors.Is(err, context.Canceled) || !engine.body.closed {
		t.Fatal("cancellation must close response", err)
	}
}
func TestExactTailLimitIsNotCertainTruncation(t *testing.T) {
	engine := &fakeEngine{tty: true, body: &trackingBody{Reader: strings.NewReader("one\ntwo\n")}}
	got, err := (&Service{Engine: engine}).Snapshot(context.Background(), "id", time.Now().Add(-time.Hour), time.Now(), 2)
	if err != nil || !got.LimitReached || got.Truncated {
		t.Fatal("exact tail limit is an indication, not certain truncation", err)
	}
}
