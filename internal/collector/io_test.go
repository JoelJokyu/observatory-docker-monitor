package collector

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func ioStats(total uint64) Stats {
	s := stats(100, 1000)
	s.Networks = map[string]NetworkStats{"eth0": {Rx: total, Tx: total * 2}, "eth1": {Rx: total, Tx: total}}
	s.Block.Entries = []BlockEntry{{Major: 8, Minor: 0, Op: "Read", Value: total}, {Major: 8, Minor: 0, Op: "Write", Value: total * 3}, {Major: 8, Minor: 0, Op: "Total", Value: total * 4}}
	pid := uint64(7)
	s.PIDs.Current = &pid
	return s
}
func TestIOTotalsAndRates(t *testing.T) {
	before := ioStats(100)
	before.Read = before.Read.Add(-10 * time.Second)
	after := ioStats(200)
	v, err := Normalize("id", after, &before)
	if err != nil {
		t.Fatal(err)
	}
	if *v.NetworkRx != 400 || *v.NetworkTx != 600 || *v.BlockRead != 200 || *v.BlockWrite != 600 || *v.PIDs != 7 {
		t.Fatalf("incorrect totals: %+v", v)
	}
	if *v.NetworkRxRate != 20 || *v.NetworkTxRate != 30 || *v.BlockReadRate != 10 || *v.BlockWriteRate != 30 {
		t.Fatalf("incorrect rates: %+v", v)
	}
	initial, err := Normalize("id", after, nil)
	if err != nil || initial.NetworkRx == nil || initial.NetworkRxRate != nil || initial.BlockReadRate != nil {
		t.Fatal("first observation must contain totals but no rates")
	}
}
func TestMissingZeroAndUnsupportedCounters(t *testing.T) {
	missing := stats(0, 0)
	v, err := Normalize("id", missing, nil)
	if err != nil || v.NetworkRx != nil || v.BlockRead != nil || v.PIDs != nil {
		t.Fatal("missing counters became zeros")
	}
	var s Stats
	if err = json.Unmarshal([]byte(`{"read":"2026-01-01T00:00:10Z","networks":{},"blkio_stats":{"io_service_bytes_recursive":[]},"pids_stats":{"current":0}}`), &s); err != nil {
		t.Fatal(err)
	}
	v, err = Normalize("id", s, nil)
	if err != nil || v.NetworkRx == nil || *v.NetworkRx != 0 || v.BlockRead == nil || *v.BlockRead != 0 || v.PIDs == nil || *v.PIDs != 0 {
		t.Fatal("explicit zeros were lost")
	}
}
func TestResetAndTopologyChangesHaveUnknownRates(t *testing.T) {
	before := ioStats(200)
	before.Read = before.Read.Add(-10 * time.Second)
	after := ioStats(100)
	v, err := Normalize("id", after, &before)
	if err != nil || v.NetworkRxRate != nil || v.BlockReadRate != nil {
		t.Fatal("counter reset produced a rate")
	}
	after = ioStats(300)
	delete(after.Networks, "eth1")
	after.Block.Entries[0].Minor = 1
	v, err = Normalize("id", after, &before)
	if err != nil || v.NetworkRxRate != nil || v.BlockReadRate != nil {
		t.Fatal("topology change produced a rate")
	}
}
func TestRejectCounterOverflow(t *testing.T) {
	s := ioStats(1)
	s.Networks["eth0"] = NetworkStats{Rx: math.MaxInt64, Tx: 1}
	if _, err := Normalize("id", s, nil); err == nil {
		t.Fatal("overflowing sum accepted")
	}
	s = ioStats(1)
	s.Block.Entries[0].Value = math.MaxUint64
	if _, err := Normalize("id", s, nil); err == nil {
		t.Fatal("overflowing block counter accepted")
	}
	s = ioStats(1)
	pid := uint64(math.MaxUint64)
	s.PIDs.Current = &pid
	if _, err := Normalize("id", s, nil); err == nil {
		t.Fatal("overflowing PID count accepted")
	}
}
