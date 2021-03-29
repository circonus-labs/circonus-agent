package multiagent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/server"
	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Metrics map[string]Metric

type Metric struct {
	Type      string      `json:"_type"`
	Value     interface{} `json:"_value"`
	Flags     string      `json:"_fl,omitempty"`
	Timestamp uint64      `json:"_ts,omitempty"`
}

type TrapResult struct {
	CheckUUID  string
	Error      string `json:"error,omitempty"`
	SubmitUUID uuid.UUID
	Filtered   uint64 `json:"filtered,omitempty"`
	Stats      uint64 `json:"stats"`
}

type Submitter struct {
	brokerTLSConfig *tls.Config
	client          *http.Client
	svr             *server.Server
	submissionURL   string
	checkUUID       string
	traceSubmits    string
	logger          zerolog.Logger
	useCompression  bool
	enabled         bool
	accumulate      bool
	interval        time.Duration
}

// submitLogshim is used to satisfy submission use of retryable-http Logger interface (avoiding ptr receiver issue)
type submitLogshim struct {
	logh zerolog.Logger
}

const (
	compressionThreshold = 1024
	traceTSFormat        = "20060102_150405.000000000"
)

func New(parentLogger zerolog.Logger, chk *check.Check, svr *server.Server) (*Submitter, error) {
	if chk == nil {
		return nil, errors.New("invalid check (nil")
	}
	if svr == nil {
		return nil, errors.New("invalid server (nil)")
	}

	s := &Submitter{
		enabled:      viper.GetBool(config.KeyMultiAgent),
		logger:       parentLogger.With().Str("pkg", "multiagent").Logger(),
		traceSubmits: viper.GetString(config.KeyDebugDumpMetrics),
		accumulate:   viper.GetBool(config.KeyMultiAgentAccumulate),
	}

	if !s.enabled {
		return s, nil
	}

	s.svr = svr
	s.useCompression = true

	cm, err := chk.CheckMeta()
	if err != nil {
		return nil, err
	}

	s.checkUUID = cm.CheckUUID

	interval := viper.GetString(config.KeyMultiAgentInterval)
	i, err := time.ParseDuration(interval)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid submission interval (%s)", interval)
	}

	s.interval = i

	surl, tlsConfig, err := chk.SubmissionURL()
	if err != nil {
		return nil, errors.Wrap(err, "getting submission url")
	}

	s.submissionURL = surl
	s.brokerTLSConfig = tlsConfig

	if s.brokerTLSConfig != nil {
		s.client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:       10 * time.Second,
					KeepAlive:     3 * time.Second,
					FallbackDelay: -1 * time.Millisecond,
				}).DialContext,
				TLSClientConfig:     s.brokerTLSConfig,
				TLSHandshakeTimeout: 10 * time.Second,
				DisableKeepAlives:   true,
				DisableCompression:  false,
				MaxIdleConns:        1,
				MaxIdleConnsPerHost: 0,
			},
		}
	} else {
		s.client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:       10 * time.Second,
					KeepAlive:     3 * time.Second,
					FallbackDelay: -1 * time.Millisecond,
				}).DialContext,
				DisableKeepAlives:   true,
				DisableCompression:  false,
				MaxIdleConns:        1,
				MaxIdleConnsPerHost: 0,
			},
		}
	}

	return s, nil
}

func (s *Submitter) Start(ctx context.Context) error {
	if !s.enabled {
		s.logger.Info().Msg("disabled, not starting")
		return nil
	}
	s.logger.Debug().Str("interval", s.interval.String()).Msg("starting submitter")

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.sendMetrics(ctx); err != nil {
				s.logger.Error().Err(err).Msg("submitting multi-agent metrics")
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Submitter) sendMetrics(ctx context.Context) error {
	metrics := s.getMetrics()
	if metrics == nil {
		return errors.New("invalid metrics (nil)")
	}
	if len(metrics) == 0 {
		return nil
	}

	rawData, err := json.Marshal(metrics)
	if err != nil {
		s.logger.Error().Err(err).Msg("json encoding metrics")
		return errors.Wrap(err, "marshaling metrics")
	}

	start := time.Now()

	submitUUID, err := uuid.NewRandom()
	if err != nil {
		s.logger.Error().Err(err).Msg("creating new submit ID")
		return errors.Wrap(err, "creating new submit ID")
	}

	payloadIsCompressed := false

	var subData *bytes.Buffer
	if s.useCompression && len(rawData) > compressionThreshold {
		subData = bytes.NewBuffer([]byte{})
		zw := gzip.NewWriter(subData)
		n, e1 := zw.Write(rawData)
		if e1 != nil {
			s.logger.Error().Err(e1).Msg("compressing metrics")
			return errors.Wrap(e1, "compressing metrics")
		}
		if n != len(rawData) {
			s.logger.Error().Int("data_len", len(rawData)).Int("written", n).Msg("gzip write length mismatch")
			return errors.Errorf("write length mismatch data length %d != written length %d", len(rawData), n)
		}
		if e2 := zw.Close(); e2 != nil {
			s.logger.Error().Err(e2).Msg("closing gzip writer")
			return errors.Wrap(e2, "closing gzip writer")
		}
		payloadIsCompressed = true
	} else {
		subData = bytes.NewBuffer(rawData)
	}

	if dumpDir := s.traceSubmits; dumpDir != "" {
		fn := path.Join(dumpDir, time.Now().UTC().Format(traceTSFormat)+"_"+submitUUID.String()+".json")
		if payloadIsCompressed {
			fn += ".gz"
		}

		if fh, e1 := os.Create(fn); e1 != nil {
			s.logger.Error().Err(e1).Str("file", fn).Msg("skipping submit trace")
		} else {
			if _, e2 := fh.Write(subData.Bytes()); e2 != nil {
				s.logger.Error().Err(e2).Msg("writing metric trace")
			}
			if e3 := fh.Close(); e3 != nil {
				s.logger.Error().Err(e3).Str("file", fn).Msg("closing metric trace")
			}
		}
	}

	dataLen := subData.Len()

	reqStart := time.Now()

	req, err := retryablehttp.NewRequest("PUT", s.submissionURL, subData)
	if err != nil {
		s.logger.Error().Err(err).Msg("creating submission request")
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", release.NAME+"/"+release.VERSION)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "close")
	req.Header.Set("Content-Length", strconv.Itoa(dataLen))
	if payloadIsCompressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = s.client
	retryClient.Logger = submitLogshim{logh: s.logger.With().Str("pkg", "retryablehttp").Logger()}
	retryClient.RetryWaitMin = 50 * time.Millisecond
	retryClient.RetryWaitMax = 1 * time.Second
	retryClient.RetryMax = 10
	retryClient.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, attempt int) {
		if attempt > 0 {
			reqStart = time.Now()
			s.logger.Warn().Str("url", r.URL.String()).Int("retry", attempt).Msg("retrying...")
		}
	}
	retryClient.ResponseLogHook = func(l retryablehttp.Logger, r *http.Response) {
		if r.StatusCode != http.StatusOK {
			s.logger.Warn().Str("url", r.Request.URL.String()).Str("status", r.Status).Msg("non-200 response...")
		}
		s.logger.Debug().Str("duration", time.Since(reqStart).String()).Msg("submission attempt")
	}

	defer retryClient.HTTPClient.CloseIdleConnections()

	resp, err := retryClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		s.logger.Error().Err(err).Msg("making request")
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error().Err(err).Msg("reading body")
		return err
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Error().Str("url", s.submissionURL).Str("status", resp.Status).Str("body", string(body)).Msg("submitting telemetry")
		return errors.Errorf("submitting metrics (%s %s)", s.submissionURL, resp.Status)
	}

	var result TrapResult
	if err := json.Unmarshal(body, &result); err != nil {
		s.logger.Error().Err(err).Str("body", string(body)).Msg("parsing response")
		return errors.Wrapf(err, "parsing response (%s)", string(body))
	}

	result.CheckUUID = s.checkUUID
	result.SubmitUUID = submitUUID

	if result.Error != "" {
		s.logger.Warn().Interface("result", result).Msg("error message in result from broker")
	}

	s.logger.Debug().
		Str("duration", time.Since(start).String()).
		Interface("result", result).
		Str("bytes_sent", bytefmt.ByteSize(uint64(dataLen))).
		Msg("submitted")

	return nil

}

func (s *Submitter) getMetrics() Metrics {
	cmetrics := s.svr.GetMetrics([]string{}, "")

	// metrics coming from multiple agents to the same check need some special handling
	// the broker understands an flags parameter, cgm does not currently support it.
	//
	// the special flags parameter `_fl` understands `+` for accumulate or `~` for average
	//
	metrics := make(Metrics, len(cmetrics))
	for mn, mv := range cmetrics {
		flag := ""
		switch mv.Type {
		case "h", "H", "s":
			// noop for histogram, cumulative histogram, and text metrics
		case "i", "I", "l", "L", "n":
			if s.accumulate {
				flag = "+" // accumulate for all numeric metrics
			}
		default:
			s.logger.Warn().Str("metric", mn).Interface("value", mv).Msg("unknown type, skipping")
			continue
		}
		metrics[mn] = Metric{
			Value: mv.Value,
			Type:  mv.Type,
			Flags: flag,
		}
	}

	s.logger.Debug().Int("num_metrics", len(metrics)).Msg("collected")

	return metrics
}

func (l submitLogshim) Printf(fmt string, v ...interface{}) {
	if strings.HasPrefix(fmt, "[DEBUG]") {
		if e := l.logh.Debug(); !e.Enabled() {
			return
		}
	}

	l.logh.Printf(fmt, v...)
}
