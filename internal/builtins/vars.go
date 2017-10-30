// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package builtins

import (
	"sync"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/rs/zerolog"
)

// Builtins defines the internal metric collector manager
type Builtins struct {
	collectors map[string]collector.Collector
	logger     zerolog.Logger
	running    bool
	sync.Mutex
}
