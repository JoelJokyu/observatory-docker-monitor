package collector

import (
	"testing"
	"time"
)

func stats(total, system uint64) Stats {
	var s Stats
	s.Read = time.Date(2026, 1, 1, 0, 0, 10, 0, time.UTC)
	s.CPU.Usage.Total = total
	s.CPU.System = system
	s.CPU.Online = 4
	s.Memory.Usage = 1000
	s.Memory.Limit = 2000
	s.Memory.Stats = map[string]uint64{"inactive_file": 200}
	return s
}
func TestNormalizeCPUAndMemory(t *testing.T) {
	prev := stats(100, 1000)
	prev.Read = prev.Read.Add(-10 * time.Second)
	current := stats(150, 1200)
	got, err := Normalize("id", current, &prev)
	if err != nil || got.CPU == nil || *got.CPU != 100 || got.Memory != 800 || got.MemoryLimit != 2000 {
		t.Fatalf("unexpected sample: %+v, %v", got, err)
	}
}
func TestUnknownAndResetCPU(t *testing.T) {
	current := stats(150, 1200)
	for name, previous := range map[string]*Stats{"initial": nil, "container reset": ptrStats(200, 1000), "system reset": ptrStats(100, 1400), "same timestamp": ptrStats(100, 1000)} {
		t.Run(name, func(t *testing.T) {
			if previous != nil && name != "same timestamp" {
				previous.Read = previous.Read.Add(-time.Second)
			}
			got, err := Normalize("id", current, previous)
			if err != nil || got.CPU != nil {
				t.Fatalf("expected unknown CPU: %+v %v", got, err)
			}
		})
	}
}
func ptrStats(total, system uint64) *Stats { s := stats(total, system); return &s }
func TestMemoryCgroupV1AndInvalidCache(t *testing.T) {
	s := stats(0, 0)
	s.Memory.Stats = map[string]uint64{"total_inactive_file": 300}
	got, err := Normalize("id", s, nil)
	if err != nil || got.Memory != 700 {
		t.Fatalf("v1 memory: %+v %v", got, err)
	}
	s.Memory.Stats = map[string]uint64{"inactive_file": 1200}
	got, _ = Normalize("id", s, nil)
	if got.Memory != 1000 {
		t.Fatal("cache must not underflow memory")
	}
	s.Read = time.Time{}
	if _, err = Normalize("id", s, nil); err == nil {
		t.Fatal("zero timestamp accepted")
	}
}
