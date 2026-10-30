package logs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/moby/moby/api/pkg/stdcopy"
)

func frame(stream stdcopy.StdType, text string) []byte {
	wire := make([]byte, 8+len(text))
	wire[0] = byte(stream)
	binary.BigEndian.PutUint32(wire[4:8], uint32(len(text)))
	copy(wire[8:], text)
	return wire
}
func TestFrameFixtureMatchesDockerDecoder(t *testing.T) {
	var stdout, stderr bytes.Buffer
	wire := append(frame(stdcopy.Stdout, "out\n"), frame(stdcopy.Stderr, "err\n")...)
	if _, err := stdcopy.StdCopy(&stdout, &stderr, bytes.NewReader(wire)); err != nil || stdout.String() != "out\n" || stderr.String() != "err\n" {
		t.Fatal("fixture does not match Docker framing", err)
	}
}

func TestDecodeDockerFramesAndPartialWrites(t *testing.T) {
	raw := append(frame(stdcopy.Stdout, "2026-10-24T12:00:00Z hel"), frame(stdcopy.Stdout, "lo\ncontinuation <script>alert(1)</script>\n")...)
	raw = append(raw, frame(stdcopy.Stderr, "2026-10-24T12:00:01Z normal stderr output\n")...)
	got, err := decode(context.Background(), &shortReader{Reader: bytes.NewReader(raw)}, false, 500)
	if err != nil || len(got.entries) != 3 {
		t.Fatalf("decode failed: %+v %v", got, err)
	}
	if got.entries[0].Message != "hello" || got.entries[0].Timestamp == nil || got.entries[0].Stream != "stdout" {
		t.Fatalf("split record lost: %+v", got.entries[0])
	}
	if got.entries[1].Timestamp != nil || got.entries[1].Message != "continuation <script>alert(1)</script>" {
		t.Fatal("continuation altered")
	}
	if got.entries[2].Stream != "stderr" || got.truncated {
		t.Fatal("stderr provenance or completion lost")
	}
}

type shortReader struct{ io.Reader }

func (r *shortReader) Read(p []byte) (int, error) { return r.Reader.Read(p[:min(len(p), 3)]) }
func TestTTYAndEmptyLogs(t *testing.T) {
	got, err := decode(context.Background(), strings.NewReader("2026-10-24T12:00:00Z hello\nlast line"), true, 500)
	if err != nil || len(got.entries) != 2 || got.entries[0].Stream != "combined" || got.entries[1].Partial {
		t.Fatalf("TTY decode failed: %+v %v", got, err)
	}
	got, err = decode(context.Background(), strings.NewReader(""), false, 500)
	if err != nil || got.entries == nil || len(got.entries) != 0 {
		t.Fatal("empty snapshot must be an empty array")
	}
}
func TestLineAndByteBudgets(t *testing.T) {
	got, err := decode(context.Background(), bytes.NewReader(frame(stdcopy.Stdout, "one\ntwo\nthree\n")), false, 2)
	if err != nil || len(got.entries) != 2 || !got.truncated || got.reason != "line_limit" {
		t.Fatalf("line cap failed: %+v %v", got, err)
	}
	got, err = decode(context.Background(), bytes.NewReader(frame(stdcopy.Stdout, strings.Repeat("界", MaxTextBytes))), false, 500)
	if err != nil || !got.truncated || got.reason != "text_bytes" || len(got.entries) != 1 || !got.entries[0].Partial || !utf8.ValidString(got.entries[0].Message) || got.textBytes > MaxTextBytes {
		t.Fatalf("text cap failed: entries=%d bytes=%d reason=%s err=%v", len(got.entries), got.textBytes, got.reason, err)
	}
}
func TestWireBudgetAndHugeDeclaredFrame(t *testing.T) {
	header := make([]byte, 8)
	header[0] = 1
	binary.BigEndian.PutUint32(header[4:], ^uint32(0))
	raw := io.MultiReader(bytes.NewReader(header), strings.NewReader(strings.Repeat("x", MaxTextBytes+4096)))
	got, err := decode(context.Background(), raw, false, 500)
	if err != nil || !got.truncated || got.reason != "text_bytes" {
		t.Fatal("declared frame size must not drive allocation", err)
	}
	got, err = decode(context.Background(), bytes.NewReader(bytes.Repeat([]byte{1, 0, 0, 0, 0, 0, 0, 0}, MaxWireBytes/8+1)), false, 500)
	if err != nil || !got.truncated || got.reason != "wire_bytes" {
		t.Fatal("wire budget failed", err)
	}
}
func TestInvalidFrameAndCancellation(t *testing.T) {
	for _, raw := range [][]byte{{1, 0, 0}, {9, 0, 0, 0, 0, 0, 0, 0}, {1, 0, 0, 0, 0, 0, 0, 5, 'x'}} {
		if _, err := decode(context.Background(), bytes.NewReader(raw), false, 500); err == nil {
			t.Fatal("invalid frame accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := decode(ctx, strings.NewReader("hello"), true, 500); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored", err)
	}
}
