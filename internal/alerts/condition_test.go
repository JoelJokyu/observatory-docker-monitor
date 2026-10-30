package alerts

import (
	"testing"
	"time"
)

func TestSustainedEvidenceAndRepeatSuppression(t *testing.T) {
	now := time.Now()
	var c Condition
	for i := 0; i < 3; i++ {
		if c.Observe(now.Add(time.Duration(i)*10*time.Second), true, true, 30*time.Second, 15*time.Second).Fired {
			t.Fatal("fired without sustained evidence")
		}
	}
	if !c.Observe(now.Add(30*time.Second), true, true, 30*time.Second, 15*time.Second).Fired {
		t.Fatal("sustained condition did not fire")
	}
	if c.Observe(now.Add(40*time.Second), true, true, 30*time.Second, 15*time.Second).Fired {
		t.Fatal("repeated ongoing condition")
	}
	if !c.Observe(now.Add(50*time.Second), false, true, 30*time.Second, 15*time.Second).Recovered {
		t.Fatal("fresh low reading did not recover")
	}
}
func TestUnknownGapsAndReplay(t *testing.T) {
	now := time.Now()
	var c Condition
	c.Observe(now, true, true, 30*time.Second, 15*time.Second)
	c.Observe(now.Add(10*time.Second), true, false, 30*time.Second, 15*time.Second)
	if c.Observe(now.Add(30*time.Second), true, true, 30*time.Second, 15*time.Second).Fired {
		t.Fatal("missing data counted as sustained evidence")
	}
	if c.Observe(now.Add(30*time.Second), true, true, 30*time.Second, 15*time.Second).Fired {
		t.Fatal("same observation advanced condition")
	}
	c.Active = true
	c.Observe(now.Add(40*time.Second), false, false, 30*time.Second, 15*time.Second)
	if !c.Active || !c.Unknown {
		t.Fatal("unknown resolved active condition")
	}
	c.Observe(now.Add(20*time.Second), false, true, 30*time.Second, 15*time.Second)
	if !c.Active {
		t.Fatal("out-of-order replay resolved active condition")
	}
}
