package alerts

import "time"

type Rule struct {
	Project         string    `json:"project"`
	Enabled         bool      `json:"enabled"`
	CPUEnabled      bool      `json:"cpu_enabled"`
	MemoryEnabled   bool      `json:"memory_enabled"`
	CPUThreshold    float64   `json:"cpu_threshold"`
	MemoryThreshold float64   `json:"memory_threshold"`
	NonzeroExit     bool      `json:"nonzero_exit"`
	OOM             bool      `json:"oom"`
	Unhealthy       bool      `json:"unhealthy"`
	EnabledSince    time.Time `json:"enabled_since"`
}

func DefaultRule(project string) Rule {
	return Rule{Project: project, CPUEnabled: true, MemoryEnabled: true, CPUThreshold: 80, MemoryThreshold: 80, NonzeroExit: true, OOM: true, Unhealthy: true}
}

type Alert struct {
	ID           string     `json:"id"`
	Project      string     `json:"project"`
	ContainerID  string     `json:"container_id"`
	Name         string     `json:"name"`
	Service      string     `json:"service"`
	Kind         string     `json:"kind"`
	EvidenceFrom time.Time  `json:"evidence_from"`
	At           time.Time  `json:"at"`
	LastObserved time.Time  `json:"last_observed_at"`
	Value        *float64   `json:"value"`
	Threshold    *float64   `json:"threshold"`
	Detail       string     `json:"detail"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	Resolution   string     `json:"resolution"`
	DismissedAt  *time.Time `json:"dismissed_at"`
}
