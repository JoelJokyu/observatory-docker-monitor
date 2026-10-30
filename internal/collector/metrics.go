package collector

import (
	"math"
	"strings"
	"time"

	"observatory.local/platform/internal/model"
)

type CPUStats struct {
	Usage struct {
		Total uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	System uint64 `json:"system_cpu_usage"`
	Online uint32 `json:"online_cpus"`
}
type NetworkStats struct {
	Rx uint64 `json:"rx_bytes"`
	Tx uint64 `json:"tx_bytes"`
}
type BlockEntry struct {
	Major uint64 `json:"major"`
	Minor uint64 `json:"minor"`
	Op    string `json:"op"`
	Value uint64 `json:"value"`
}
type Stats struct {
	Read   time.Time `json:"read"`
	CPU    CPUStats  `json:"cpu_stats"`
	Memory struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]NetworkStats `json:"networks"`
	Block    struct {
		Entries []BlockEntry `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
	PIDs struct {
		Current *uint64 `json:"current"`
	} `json:"pids_stats"`
}

func integer(v uint64) (*int64, error) {
	if v > math.MaxInt64 {
		return nil, errInvalidStats
	}
	n := int64(v)
	return &n, nil
}
func add(total, value uint64) (uint64, error) {
	if value > math.MaxInt64 || total > math.MaxInt64-value {
		return 0, errInvalidStats
	}
	return total + value, nil
}
func networkTotals(s Stats) (*int64, *int64, error) {
	if s.Networks == nil {
		return nil, nil, nil
	}
	var rx, tx uint64
	var err error
	for _, n := range s.Networks {
		rx, err = add(rx, n.Rx)
		if err != nil {
			return nil, nil, err
		}
		tx, err = add(tx, n.Tx)
		if err != nil {
			return nil, nil, err
		}
	}
	r, _ := integer(rx)
	t, _ := integer(tx)
	return r, t, nil
}
func blockTotals(s Stats) (*int64, *int64, error) {
	if s.Block.Entries == nil {
		return nil, nil, nil
	}
	var read, write uint64
	var err error
	for _, e := range s.Block.Entries {
		switch strings.ToLower(e.Op) {
		case "read":
			read, err = add(read, e.Value)
		case "write":
			write, err = add(write, e.Value)
		}
		if err != nil {
			return nil, nil, err
		}
	}
	r, _ := integer(read)
	w, _ := integer(write)
	return r, w, nil
}
func rate(current, previous *int64, seconds float64) *float64 {
	if current == nil || previous == nil || seconds <= 0 || *current < *previous {
		return nil
	}
	r := float64(*current-*previous) / seconds
	return &r
}
func sameInterfaces(a, b Stats) bool {
	if a.Networks == nil || b.Networks == nil || len(a.Networks) != len(b.Networks) {
		return false
	}
	for name, n := range a.Networks {
		p, ok := b.Networks[name]
		if !ok || n.Rx < p.Rx || n.Tx < p.Tx {
			return false
		}
	}
	return true
}
func sameDevices(a, b Stats) bool {
	if a.Block.Entries == nil || b.Block.Entries == nil {
		return false
	}
	type key struct {
		major, minor uint64
		op           string
	}
	counters := func(s Stats) map[key]uint64 {
		m := map[key]uint64{}
		for _, e := range s.Block.Entries {
			op := strings.ToLower(e.Op)
			if op == "read" || op == "write" {
				m[key{e.Major, e.Minor, op}] = e.Value
			}
		}
		return m
	}
	am, bm := counters(a), counters(b)
	if len(am) != len(bm) {
		return false
	}
	for k, v := range am {
		p, ok := bm[k]
		if !ok || v < p {
			return false
		}
	}
	return true
}

// CPU is measured in core equivalents (100% = one core). I/O totals describe
// the current container run; charts use deltas per second, never cumulative sums.
// Missing support stays null. First samples and counter resets have no rate.
func Normalize(id string, current Stats, previous *Stats) (model.Sample, error) {
	v := model.Sample{ContainerID: id, At: current.Read}
	if current.Read.IsZero() {
		return v, errInvalidStats
	}
	usage := current.Memory.Usage
	cache, ok := current.Memory.Stats["inactive_file"]
	if !ok {
		cache = current.Memory.Stats["total_inactive_file"]
	}
	if cache <= usage {
		usage -= cache
	}
	if usage > math.MaxInt64 || current.Memory.Limit > math.MaxInt64 {
		return v, errInvalidStats
	}
	v.Memory = int64(usage)
	v.MemoryLimit = int64(current.Memory.Limit)
	var err error
	v.NetworkRx, v.NetworkTx, err = networkTotals(current)
	if err != nil {
		return v, err
	}
	v.BlockRead, v.BlockWrite, err = blockTotals(current)
	if err != nil {
		return v, err
	}
	if current.PIDs.Current != nil {
		v.PIDs, err = integer(*current.PIDs.Current)
		if err != nil {
			return v, err
		}
	}
	if previous == nil || previous.Read.IsZero() || !current.Read.After(previous.Read) {
		return v, nil
	}
	seconds := current.Read.Sub(previous.Read).Seconds()
	if sameInterfaces(current, *previous) {
		rx, tx, e := networkTotals(*previous)
		if e != nil {
			return v, e
		}
		v.NetworkRxRate = rate(v.NetworkRx, rx, seconds)
		v.NetworkTxRate = rate(v.NetworkTx, tx, seconds)
	}
	if sameDevices(current, *previous) {
		read, write, e := blockTotals(*previous)
		if e != nil {
			return v, e
		}
		v.BlockReadRate = rate(v.BlockRead, read, seconds)
		v.BlockWriteRate = rate(v.BlockWrite, write, seconds)
	}
	if current.CPU.Online == 0 || current.CPU.Usage.Total < previous.CPU.Usage.Total || current.CPU.System <= previous.CPU.System {
		return v, nil
	}
	percent := float64(current.CPU.Usage.Total-previous.CPU.Usage.Total) / float64(current.CPU.System-previous.CPU.System) * float64(current.CPU.Online) * 100
	v.CPU = &percent
	return v, nil
}
