// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/spf13/viper"
)

func TestBuildFrame(t *testing.T) {
	t.Log("Testing buildFrame")

	t.Log("valid")
	{
		viper.Set(config.KeyReverse, false)
		c, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		expect := []byte{0x0, 0x1, 0x0, 0x0, 0x0, 0x4, 0x74, 0x65, 0x73, 0x74}
		data := c.buildFrame(1, []byte("test"))

		if data == nil {
			t.Fatal("expected not nil")
		}

		if bytes.Compare(data, expect) != 0 {
			t.Fatalf("expected (%#v) got (%#v)", expect, data)
		}
	}
}

func TestSetNextDelay(t *testing.T) {
	t.Log("Testing setNextDelay")

	t.Log("delay == max")
	{
		viper.Set(config.KeyReverse, false)
		c, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		c.delay = c.maxDelay
		c.setNextDelay()
		if c.delay != c.maxDelay {
			t.Fatalf("delay changed, not set to max")
		}
	}

	t.Log("valid change")
	{
		viper.Set(config.KeyReverse, false)
		c, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		currDelay := c.delay

		c.setNextDelay()

		if c.delay == currDelay {
			t.Fatalf("delay did NOT changed %s == %s", c.delay.String(), currDelay.String())
		}

		min := time.Duration(minDelayStep) * time.Second
		max := time.Duration(maxDelayStep) * time.Second
		diff := c.delay - currDelay

		if diff < min {
			t.Fatalf("delay increment (%s) < minimum (%s)", diff.String(), min.String())
		}

		if diff > max {
			t.Fatalf("delay increment (%s) > maximum (%s)", diff.String(), max.String())
		}
	}

	t.Log("reset to max")
	{
		viper.Set(config.KeyReverse, false)
		c, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		c.delay = 61 * time.Second

		c.setNextDelay()

		if c.delay != c.maxDelay {
			t.Fatalf("delay did NOT reset %s == %s", c.delay.String(), c.maxDelay)
		}
	}
}

func TestResetConnectionAttempts(t *testing.T) {
	t.Log("Testing resetConnectionAttempts")

	viper.Set(config.KeyReverse, false)
	c, err := New(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got (%s)", err)
	}

	c.delay = 10 * time.Second
	c.connAttempts = 10

	c.resetConnectionAttempts()

	if c.delay != 1*time.Second {
		t.Fatalf("delay not reset (%s)", c.delay.String())
	}

	if c.connAttempts != 0 {
		t.Fatalf("attempts not reset (%d)", c.connAttempts)
	}
}
