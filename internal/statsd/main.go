// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
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

	port := viper.GetString(config.KeyStatsdPort)
	address := net.JoinHostPort("localhost", port)
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
func (s *Server) Start() error {
	if s.disabled {
		s.logger.Info().Msg("disabled, not starting listener")
		return nil
	}

	var err error
	s.server, err = s.newStatsdServer()
	if err != nil {
		return err
	}

	s.server.t.Go(s.reader)
	s.server.t.Go(s.processor)

	return s.server.t.Wait()
}

// Stop the server
func (s *Server) Stop() error {
	if s.disabled {
		return nil
	}

	s.logger.Info().Msg("Stopping StatsD Server")

	if s.server.t.Alive() {
		s.server.t.Kill(nil)
	}

	if s.groupMetrics != nil {
		s.logger.Info().Msg("Flushing group metrics")
		s.groupMetricsmu.Lock()
		s.groupMetrics.Flush()
		s.groupMetricsmu.Unlock()
	}

	return nil
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
		s.logger.Info().Msg("group check disabled")
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

// newStatsdServer returns a new statsd listening server
func (s *Server) newStatsdServer() (*statsdServer, error) {
	s.logger.Info().Str("addr", s.address.String()).Msg("starting listener")
	l, err := net.ListenUDP("udp", s.address)
	if err != nil {
		return nil, err
	}
	return &statsdServer{
		listener: l,
		packetCh: make(chan []byte, packetQueueSize),
	}, nil
}

// reader reads packets from the statsd listener, adds packets recevied to the queue
func (s *Server) reader() error {
	defer close(s.server.packetCh)
	for {
		buff := make([]byte, maxPacketSize)
		n, err := s.server.listener.Read(buff)
		if s.shutdown() {
			return nil
		}
		if err != nil {
			return errors.Wrap(err, "reader")
		}
		if n > 0 {
			pkt := make([]byte, n)
			copy(pkt, buff[:n])
			s.server.packetCh <- pkt
		}
	}
}

// processor reads the packet queue and processes each packet
func (s *Server) processor() error {
	defer s.server.listener.Close()
	for {
		select {
		case <-s.server.t.Dying():
			return nil
		case pkt := <-s.server.packetCh:
			err := s.processPacket(pkt)
			if err != nil {
				return errors.Wrap(err, "processor")
			}
		default:
		}
	}
}

// shutdown checks whether tomb is dying
func (s *Server) shutdown() bool {
	select {
	case <-s.server.t.Dying():
		return true
	default:
		return false
	}
}
