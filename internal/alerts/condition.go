package alerts

import "time"

// Condition advances only on distinct, ordered observations. Missing data can
// interrupt evidence, but must never masquerade as a recovery.
type Condition struct {
	Since   *time.Time `json:"since"`
	Last    *time.Time `json:"last"`
	Active  bool       `json:"active"`
	Unknown bool       `json:"unknown"`
}
type Transition struct {
	Fired     bool
	Recovered bool
}

func (c *Condition) Observe(at time.Time, high, known bool, duration, maxGap time.Duration) Transition {
	if c.Last != nil && !at.After(*c.Last) {
		return Transition{}
	}
	gap := c.Last != nil && at.Sub(*c.Last) > maxGap
	c.Last = &at
	if !known {
		c.Since = nil
		c.Unknown = true
		return Transition{}
	}
	c.Unknown = false
	if !high {
		was := c.Active
		c.Active = false
		c.Since = nil
		return Transition{Recovered: was}
	}
	if c.Active {
		return Transition{}
	}
	if gap || c.Since == nil {
		c.Since = &at
	}
	if at.Sub(*c.Since) >= duration {
		c.Active = true
		return Transition{Fired: true}
	}
	return Transition{}
}
