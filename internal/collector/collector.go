package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/client"
	"observatory.local/platform/internal/model"
)

var errInvalidStats = errors.New("invalid Docker statistics")

type Engine interface {
	ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerStats(context.Context, string, client.ContainerStatsOptions) (client.ContainerStatsResult, error)
}
type Storage interface {
	Inventory(context.Context, []model.Container) error
	Save(context.Context, model.Sample) error
	Prune(context.Context, time.Time) error
}
type Status struct {
	LastCycle     *time.Time `json:"last_cycle"`
	LastInventory *time.Time `json:"last_inventory"`
	LastSample    *time.Time `json:"last_sample"`
	Healthy       bool       `json:"healthy"`
	Errors        int        `json:"errors"`
	Message       string     `json:"message"`
}
type Collector struct {
	Engine   Engine
	Store    Storage
	Interval time.Duration
	Log      *slog.Logger
	mu       sync.RWMutex
	status   Status
	previous map[string]Stats
}

func (c *Collector) Status() Status { c.mu.RLock(); defer c.mu.RUnlock(); return c.status }
func (c *Collector) Run(ctx context.Context) {
	c.previous = make(map[string]Stats)
	tick := time.NewTicker(c.Interval)
	defer tick.Stop()
	prune := time.NewTicker(time.Minute)
	defer prune.Stop()
	c.cycle(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			c.cycle(ctx)
		case <-prune.C:
			pruneCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.Store.Prune(pruneCtx, time.Now().UTC().Add(-7*24*time.Hour))
			cancel()
			if err != nil {
				c.Log.Error("retention cleanup failed", "error", err)
			}
		}
	}
}
func (c *Collector) cycle(ctx context.Context) {
	now := time.Now().UTC()
	state := c.Status()
	state.LastCycle = &now
	state.Errors = 0
	state.Healthy = false
	state.Message = ""
	defer func() { c.mu.Lock(); c.status = state; c.mu.Unlock() }()
	listCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	listing, err := c.Engine.ContainerList(listCtx, client.ContainerListOptions{All: true})
	cancel()
	if err != nil {
		state.Errors = 1
		state.Message = "Docker inventory unavailable; retrying"
		c.Log.Warn(state.Message, "error", err)
		c.previous = make(map[string]Stats)
		return
	}
	containers := make([]model.Container, 0, len(listing.Items))
	for _, item := range listing.Items {
		name := item.ID[:min(12, len(item.ID))]
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		containers = append(containers, model.Container{ID: item.ID, Name: name, Image: item.Image, State: string(item.State), SeenAt: now, Present: true, Project: item.Labels["com.docker.compose.project"], Service: item.Labels["com.docker.compose.service"]})
	}
	storeCtx, storeCancel := context.WithTimeout(ctx, 5*time.Second)
	err = c.Store.Inventory(storeCtx, containers)
	storeCancel()
	if err != nil {
		state.Errors = 1
		state.Message = "Cannot persist Docker inventory"
		c.Log.Error(state.Message, "error", err)
		c.previous = make(map[string]Stats)
		return
	}
	state.LastInventory = &now
	type result struct {
		id    string
		stats Stats
		err   error
	}
	jobs := make(chan string, len(containers))
	results := make(chan result, len(containers))
	count := 0
	for _, container := range containers {
		if container.State == "running" {
			jobs <- container.ID
			count++
		}
	}
	close(jobs)
	var workers sync.WaitGroup
	for i := 0; i < min(4, count); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for id := range jobs {
				result := result{id: id}
				func() {
					statsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
					defer cancel()
					response, err := c.Engine.ContainerStats(statsCtx, id, client.ContainerStatsOptions{})
					if err != nil {
						result.err = err
						return
					}
					defer response.Body.Close()
					result.err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result.stats)
				}()
				results <- result
			}
		}()
	}
	workers.Wait()
	close(results)
	next := make(map[string]Stats)
	for r := range results {
		if r.err != nil {
			state.Errors++
			c.Log.Warn("container sample failed", "container", r.id, "error", r.err)
			continue
		}
		var previous *Stats
		if p, ok := c.previous[r.id]; ok && r.stats.Read.Sub(p.Read) <= 3*c.Interval {
			previous = &p
		}
		sample, err := Normalize(r.id, r.stats, previous)
		if err == nil && (sample.At.After(now.Add(time.Minute)) || sample.At.Before(now.Add(-3*c.Interval))) {
			err = fmt.Errorf("Docker statistics timestamp outside collection window")
		}
		if err == nil {
			writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = c.Store.Save(writeCtx, sample)
			cancel()
		}
		if err != nil {
			state.Errors++
			c.Log.Warn("sample rejected or not persisted", "container", r.id, "error", err)
			continue
		}
		next[r.id] = r.stats
		at := sample.At
		if state.LastSample == nil || at.After(*state.LastSample) {
			state.LastSample = &at
		}
	}
	c.previous = next
	state.Healthy = state.Errors == 0
	if state.Errors > 0 {
		state.Message = "Some container samples failed; historical values may be stale"
	}
}
