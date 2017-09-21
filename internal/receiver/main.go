// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package receiver

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Metrics holds metrics received via HTTP PUT/POST
type Metrics map[string]interface{}

var (
	metricsmu sync.Mutex
	metrics   *Metrics
	logger    = log.With().Str("pkg", "receiver").Logger()
)

// Flush returns current metrics
func Flush() *Metrics {
	metricsmu.Lock()
	defer metricsmu.Unlock()

	if metrics == nil {
		return &Metrics{}
	}

	currMetrics := metrics
	metrics = nil

	return currMetrics
}

// NumMetricGroups returns number of metrics
func NumMetricGroups() int {
	metricsmu.Lock()
	defer metricsmu.Unlock()

	if metrics == nil {
		metrics = &Metrics{}
	}

	return len(*metrics)
}

// Parse handles incoming PUT/POST requests
func Parse(id string, data io.ReadCloser) error {
	metricsmu.Lock()
	defer metricsmu.Unlock()

	if metrics == nil {
		metrics = &Metrics{}
	}

	var tmp map[string]interface{}
	if err := json.NewDecoder(data).Decode(&tmp); err != nil {
		return errors.Wrapf(err, "parsing json for %s", id)
	}

	(*metrics)[id] = tmp

	return nil
}
