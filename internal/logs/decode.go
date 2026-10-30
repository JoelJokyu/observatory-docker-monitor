package logs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var errOutputLimit = errors.New("snapshot output limit")
var errInvalidStream = errors.New("invalid Docker log stream")

func itoa(n int) string { return strconv.Itoa(n) }

type pendingLine struct {
	data  []byte
	order int
}
type decoded struct {
	entries                              []Entry
	pending                              map[string]*pendingLine
	rawBytes, textBytes, sequence, limit int
	truncated                            bool
	reason                               string
}

func (d *decoded) cut(reason string) {
	d.truncated = true
	if d.reason == "" {
		d.reason = reason
	}
}
func (d *decoded) emit(stream string, raw []byte, partial bool) error {
	if len(d.entries) >= d.limit {
		d.cut("line_limit")
		return errOutputLimit
	}
	text := strings.ToValidUTF8(string(raw), "\uFFFD")
	var timestamp *time.Time
	if prefix, rest, ok := strings.Cut(text, " "); ok {
		if at, err := time.Parse(time.RFC3339Nano, prefix); err == nil {
			timestamp = &at
			text = rest
		}
	}
	remaining := MaxTextBytes - d.textBytes
	if len(text) > remaining {
		n := remaining
		for n > 0 && !utf8.ValidString(text[:n]) {
			n--
		}
		text = text[:n]
		partial = true
		d.cut("text_bytes")
	}
	d.textBytes += len(text)
	d.entries = append(d.entries, Entry{Timestamp: timestamp, Stream: stream, Message: text, Partial: partial})
	return nil
}
func (d *decoded) feed(stream string, data []byte) error {
	line := d.pending[stream]
	if line == nil {
		line = &pendingLine{}
		d.pending[stream] = line
	}
	for len(data) > 0 {
		if len(d.entries) >= d.limit {
			d.cut("line_limit")
			return errOutputLimit
		}
		if d.rawBytes >= MaxTextBytes {
			d.cut("text_bytes")
			return errOutputLimit
		}
		if len(line.data) == 0 {
			d.sequence++
			line.order = d.sequence
		}
		end := bytes.IndexByte(data, '\n')
		n := len(data)
		if end >= 0 {
			n = end + 1
		}
		allowed := min(n, MaxTextBytes-d.rawBytes)
		segment := data[:allowed]
		d.rawBytes += allowed
		newline := len(segment) > 0 && segment[len(segment)-1] == '\n'
		if newline {
			segment = segment[:len(segment)-1]
		}
		line.data = append(line.data, segment...)
		data = data[allowed:]
		if newline {
			if err := d.emit(stream, line.data, false); err != nil {
				return err
			}
			line.data = nil
		}
		if allowed < n {
			d.cut("text_bytes")
			return errOutputLimit
		}
	}
	return nil
}
func (d *decoded) flush(partial bool) {
	// There are at most three pending streams. Preserve their first-byte order.
	for {
		name := ""
		order := int(^uint(0) >> 1)
		for stream, line := range d.pending {
			if len(line.data) > 0 && line.order < order {
				name = stream
				order = line.order
			}
		}
		if name == "" {
			return
		}
		line := d.pending[name]
		_ = d.emit(name, line.data, partial)
		line.data = nil
	}
}

// Docker's non-TTY stream has an 8-byte header followed by payload bytes.
// Read payload in fixed chunks: a declared frame size never drives allocation.
// TTY output is a combined stream and cannot recover stdout/stderr provenance.
func decode(ctx context.Context, reader io.Reader, tty bool, limit int) (decoded, error) {
	d := decoded{entries: make([]Entry, 0), pending: map[string]*pendingLine{}, limit: limit}
	wire := &io.LimitedReader{R: reader, N: MaxWireBytes}
	chunk := make([]byte, 4096)
	finish := func() (decoded, error) { d.flush(d.truncated); return d, nil }
	if tty {
		for {
			if err := ctx.Err(); err != nil {
				return d, err
			}
			n, err := wire.Read(chunk)
			if n > 0 {
				if feedErr := d.feed("combined", chunk[:n]); feedErr != nil {
					return finish()
				}
			}
			if err != nil {
				if err == io.EOF {
					if wire.N == 0 {
						d.cut("wire_bytes")
					}
					return finish()
				}
				return d, err
			}
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			return d, err
		}
		var header [8]byte
		n, err := io.ReadFull(wire, header[:])
		if err != nil {
			if wire.N == 0 {
				d.cut("wire_bytes")
				return finish()
			}
			if err == io.EOF && n == 0 {
				return finish()
			}
			return d, errInvalidStream
		}
		if header[1] != 0 || header[2] != 0 || header[3] != 0 {
			return d, errInvalidStream
		}
		stream := ""
		switch header[0] {
		case 1:
			stream = "stdout"
		case 2:
			stream = "stderr"
		default:
			return d, errInvalidStream
		}
		remaining := int64(binary.BigEndian.Uint32(header[4:]))
		for remaining > 0 {
			if err := ctx.Err(); err != nil {
				return d, err
			}
			n, err := io.ReadFull(wire, chunk[:min(int64(len(chunk)), remaining)])
			if n > 0 {
				remaining -= int64(n)
				if feedErr := d.feed(stream, chunk[:n]); feedErr != nil {
					return finish()
				}
			}
			if err != nil {
				if wire.N == 0 {
					d.cut("wire_bytes")
					return finish()
				}
				return d, errInvalidStream
			}
		}
	}
}
