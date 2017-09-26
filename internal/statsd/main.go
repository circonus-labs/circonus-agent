// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
	"context"
	stdlog "log"
	"net"
	"regexp"

	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// New returns a statsd server definition
func New() (*Server, error) {
	s := Server{
		disabled:       viper.GetBool(config.KeyStatsdDisabled),
		logger:         log.With().Str("pkg", "statsd").Logger(),
		hostPrefix:     viper.GetString(config.KeyStatsdHostPrefix),
		hostCategory:   viper.GetString(config.KeyStatsdHostCategory),
		groupCID:       viper.GetString(config.KeyStatsdGroupCID),
		groupPrefix:    viper.GetString(config.KeyStatsdGroupPrefix),
		groupCounterOp: viper.GetString(config.KeyStatsdGroupCounters),
		groupGaugeOp:   viper.GetString(config.KeyStatsdGroupGauges),
		groupSetOp:     viper.GetString(config.KeyStatsdGroupSets),
		debugCGM:       viper.GetBool(config.KeyDebugCGM),
		apiKey:         viper.GetString(config.KeyAPITokenKey),
		apiApp:         viper.GetString(config.KeyAPITokenApp),
		apiURL:         viper.GetString(config.KeyAPIURL),
	}

	address := net.JoinHostPort("localhost", viper.GetString(config.KeyStatsdPort))
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving address '%s'", address)
	}

	s.address = addr
	s.metricRegex = regexp.MustCompile("^(?P<name>[^:\\s]+):(?P<value>[^|\\s]+)\\|(?P<type>[a-z]+)(?:@(?P<sample>[0-9.]+))?$")
	s.metricRegexGroupNames = s.metricRegex.SubexpNames()

	if !s.disabled {
		if err := s.initHostMetrics(); err != nil {
			return nil, errors.Wrap(err, "Initializing host metrics for StatsD")
		}

		if err := s.initGroupMetrics(); err != nil {
			return nil, errors.Wrap(err, "Initializing group metrics for StatsD")
		}
	}

	return &s, nil
}

// Start the StatsD listener
func (s *Server) Start(ctx context.Context) error {
	if s.disabled {
		s.logger.Info().Msg("disabled, not starting listener")
		return nil
	}

	s.logger.Info().Str("addr", s.address.String()).Msg("starting listener")
	{
		var err error
		s.listener, err = net.ListenUDP("udp", s.address)
		if err != nil {
			return err
		}
	}

	packetQueue := make(chan []byte, packetQueueSize)
	errCh := make(chan error, 10)

	// read packets from listener
	go func() {
		defer s.listener.Close()

		for {
			buff := make([]byte, maxPacketSize)
			n, err := s.listener.Read(buff)
			if err != nil {
				errCh <- err
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				if n > 0 {
					pkt := make([]byte, n)
					copy(pkt, buff[:n])
					packetQueue <- pkt
				}
			}
		}
	}()

	// run the packet handler separately so packet processing
	// does not block the listener
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case pkt := <-packetQueue:
				err := s.processPacket(pkt)
				if err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			close(packetQueue)
			return err
		}
	}
}

// Stop the server
func (s *Server) Stop() {
	if s.disabled {
		return
	}

	if s.listener != nil {
		s.logger.Debug().Msg("Stopping listener")
		err := s.listener.Close()
		if err != nil {
			s.logger.Warn().Err(err).Msg("Closing listener")
		}
	}

	// flush any outstanding group metrics (sent directly to circonus)
	if s.groupMetrics != nil {
		s.groupMetricsmu.Lock()
		s.groupMetrics.Flush()
		s.groupMetricsmu.Unlock()
	}
}

// Flush *host* metrics only
// NOTE: group metrics flush independently via circonus-gometrics to a different check
func (s *Server) Flush() *cgm.Metrics {
	if s.disabled {
		return nil
	}

	if s.hostMetrics == nil {
		return &cgm.Metrics{}
	}

	s.hostMetricsmu.Lock()
	defer s.hostMetricsmu.Unlock()
	return s.hostMetrics.FlushMetrics()
}

// initHostMetrics initializes the host metrics circonus-gometrics instance
func (s *Server) initHostMetrics() error {
	s.hostMetricsmu.Lock()
	defer s.hostMetricsmu.Unlock()

	cmc := &cgm.Config{
		Debug: s.debugCGM,
		Log:   stdlog.New(s.logger.With().Str("pkg", "statsd-host-check").Logger(), "", 0),
	}
	// put cgm into manual mode (no interval, no api key, invalid submission url)
	cmc.Interval = "0"                            // disable automatic flush
	cmc.CheckManager.Check.SubmissionURL = "none" // disable check management (create/update)

	hm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		return errors.Wrap(err, "statsd host check")
	}

	s.hostMetrics = hm

	s.logger.Info().Msg("host check initialized")
	return nil
}

// initGroupMetrics initializes the group metric circonus-gometrics instance
// NOTE: Group metrics are sent directly to circonus, to an existing HTTPTRAP
//       check created manually or by cosi - the group check is intended to be
//       used by multiple systems.
func (s *Server) initGroupMetrics() error {
	if s.groupCID == "" {
		log.Info().Msg("group check disabled")
		return nil
	}

	s.groupMetricsmu.Lock()
	defer s.groupMetricsmu.Unlock()

	cmc := &cgm.Config{
		Debug: s.debugCGM,
		Log:   stdlog.New(s.logger.With().Str("pkg", "statsd-group-check").Logger(), "", 0),
	}
	cmc.CheckManager.API.TokenKey = s.apiKey
	cmc.CheckManager.API.TokenApp = s.apiApp
	cmc.CheckManager.API.URL = s.apiURL
	cmc.CheckManager.Check.ID = s.groupCID

	gm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		return errors.Wrap(err, "statsd group check")
	}

	s.groupMetrics = gm

	s.logger.Info().Msg("group check initialized")
	return nil
}
