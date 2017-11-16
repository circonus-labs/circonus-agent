// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package promrecv

// Metrics holds metrics received via HTTP PUT/POST
import (
	"regexp"
	"sync"

	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/rs/zerolog/log"
)

var (
	id                  string
	nameCleanerRx       *regexp.Regexp
	metricNameSeparator = "`"
	metricsmu           sync.Mutex
	metrics             *cgm.CirconusMetrics
	parseRx             *regexp.Regexp
	logger              = log.With().Str("pkg", "promrecv").Logger()
)
