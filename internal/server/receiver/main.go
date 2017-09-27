// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package receiver

import (
	"encoding/json"
	"io"

	"github.com/pkg/errors"
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
		if serr, ok := err.(*json.SyntaxError); ok {
			return errors.Wrapf(serr, "id:%s - offset %d", id, serr.Offset)
		}
		return errors.Wrapf(err, "parsing json for %s", id)
	}

	(*metrics)[id] = tmp

	return nil
}
