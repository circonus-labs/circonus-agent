// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
	"context"
	"crypto/x509"
	"io/ioutil"
	"net"
	"regexp"
	"strconv"
	"sync"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/maier/go-appstats"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// Server defines a statsd server
type Server struct {
	group                 *errgroup.Group
	groupCtx              context.Context
	disabled              bool
	address               *net.UDPAddr
	hostMetrics           *cgm.CirconusMetrics
	hostMetricsmu         sync.Mutex
	groupMetrics          *cgm.CirconusMetrics
	groupMetricsmu        sync.Mutex
	logger                zerolog.Logger
	hostPrefix            string
	hostCategory          string
	groupCID              string
	groupPrefix           string
	groupCounterOp        string
	groupGaugeOp          string
	groupSetOp            string
	metricRegex           *regexp.Regexp
	metricRegexGroupNames []string
	apiKey                string
	apiApp                string
	apiURL                string
	apiCAFile             string
	debugCGM              bool
	listener              *net.UDPConn
	packetCh              chan []byte
	baseTags              []string
}

const (
	maxPacketSize   = 1472
	packetQueueSize = 1000
	destHost        = "host"
	destGroup       = "group"
	destIgnore      = "ignore"
)

// New returns a statsd server definition
func New(ctx context.Context) (*Server, error) {
	s := Server{
		disabled: viper.GetBool(config.KeyStatsdDisabled),
		logger:   log.With().Str("pkg", "statsd").Logger(),
	}

	if s.disabled {
		s.logger.Info().Msg("disabled, not configuring")
		return &s, nil
	}

	err := validateStatsdOptions()
	if err != nil {
		return nil, err
	}

	g, gctx := errgroup.WithContext(ctx)

	s = Server{
		group:          g,
		groupCtx:       gctx,
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
		apiCAFile:      viper.GetString(config.KeyAPICAFile),
		packetCh:       make(chan []byte, packetQueueSize),
		baseTags:       tags.GetBaseTags(),
	}

	port := viper.GetString(config.KeyStatsdPort)
	address := net.JoinHostPort("localhost", port)
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving address '%s'", address)
	}

	s.address = addr
	s.metricRegex = regexp.MustCompile(`^(?P<name>[^:\s]+):(?P<value>[^|\s]+)\|(?P<type>[a-z]+)(?:\|@(?P<sample>[0-9.]+))?(?:\|#(?P<tags>[^:,]+:[^:,]+(,[^:,]+:[^:,]+)*))?$`)
	s.metricRegexGroupNames = s.metricRegex.SubexpNames()

	if !s.disabled {
		if ierr := s.initHostMetrics(); ierr != nil {
			return nil, errors.Wrap(ierr, "initializing host metrics for StatsD")
		}

		if ierr := s.initGroupMetrics(); ierr != nil {
			return nil, errors.Wrap(ierr, "initializing group metrics for StatsD")
		}
	}

	l, err := net.ListenUDP("udp", s.address)
	if err != nil {
		return nil, err
	}
	s.listener = l

	return &s, nil
}

// Start the StatsD listener
func (s *Server) Start() error {
	if s.disabled {
		s.logger.Info().Msg("disabled, not starting listener")
		return nil
	}

	s.group.Go(s.reader)
	s.group.Go(s.processor)

	go func() {
		s.group.Wait()
		// only try to flush group metrics since they go
		// directly to a broker. there is no point in trying
		// to flush host metrics as the 'server' portion of
		// the agent may have already closed.
		if s.groupMetrics != nil {
			s.logger.Info().Msg("flushing group metrics")
			s.groupMetricsmu.Lock()
			s.groupMetrics.Flush()
			s.groupMetricsmu.Unlock()
		}
	}()

	return s.group.Wait()
}

// Flush *host* metrics only
// NOTE: group metrics flush independently to a different check via circonus-gometrics
func (s *Server) Flush() *cgm.Metrics {
	if s.disabled {
		return nil
	}

	s.hostMetricsmu.Lock()
	defer s.hostMetricsmu.Unlock()

	if s.hostMetrics == nil {
		return &cgm.Metrics{}
	}

	return s.hostMetrics.FlushMetrics()
}

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue)
type logshim struct {
	logh zerolog.Logger
}

func (l logshim) Printf(fmt string, v ...interface{}) {
	l.logh.Printf(fmt, v...)
}

// initHostMetrics initializes the host metrics circonus-gometrics instance
func (s *Server) initHostMetrics() error {
	s.hostMetricsmu.Lock()
	defer s.hostMetricsmu.Unlock()

	cmc := &cgm.Config{
		Debug: s.debugCGM,
		Log:   logshim{logh: s.logger.With().Str("pkg", "cgm.statsd-host-check").Logger()},
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
		Log:   logshim{logh: s.logger.With().Str("pkg", "cgm.statsd-group-check").Logger()},
	}
	cmc.CheckManager.API.TokenKey = s.apiKey
	cmc.CheckManager.API.TokenApp = s.apiApp
	cmc.CheckManager.API.URL = s.apiURL
	cmc.CheckManager.Check.ID = s.groupCID

	if s.apiCAFile != "" {
		cert, err := ioutil.ReadFile(s.apiCAFile)
		if err != nil {
			return err
		}

		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(cert) {
			return errors.Errorf("using api CA cert %#v", cert)
		}

		cmc.CheckManager.API.CACert = cp
	}

	gm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		return errors.Wrap(err, "statsd group check")
	}

	s.groupMetrics = gm

	s.logger.Info().Msg("group check initialized")
	return nil
}

// reader reads packets from the statsd listener, adds packets recevied to the queue
func (s *Server) reader() error {
	for {
		buff := make([]byte, maxPacketSize)
		n, err := s.listener.Read(buff)
		if s.shutdown() {
			return nil
		}
		if err != nil {
			s.logger.Error().Err(err).Msg("reader")
			return errors.Wrap(err, "reader")
		}
		if n > 0 {
			appstats.IncrementInt("statsd_packets_total")
			pkt := make([]byte, n)
			copy(pkt, buff[:n])
			s.packetCh <- pkt
		}
	}
}

// processor reads the packet queue and processes each packet
func (s *Server) processor() error {
	defer s.listener.Close()
	for {
		select {
		case <-s.groupCtx.Done():
			return nil
		case pkt := <-s.packetCh:
			err := s.processPacket(pkt)
			if err != nil {
				appstats.IncrementInt("statsd_packets_bad")
				s.logger.Error().Err(err).Msg("processor")
				return errors.Wrap(err, "processor")
			}
		}
	}
}

// shutdown checks whether tomb is dying
func (s *Server) shutdown() bool {
	select {
	case <-s.groupCtx.Done():
		return true
	default:
		return false
	}
}

func validateStatsdOptions() error {
	if viper.GetBool(config.KeyStatsdDisabled) {
		return nil
	}

	port := viper.GetString(config.KeyStatsdPort)
	if port == "" {
		return errors.New("invalid StatsD port (empty)")
	}
	if ok, err := regexp.MatchString("^[0-9]+$", port); err != nil {
		return errors.Wrapf(err, "invalid StatsD port (%s)", port)
	} else if !ok {
		return errors.Errorf("invalid StatsD port (%s)", port)
	}
	if pnum, err := strconv.ParseUint(port, 10, 32); err != nil {
		return errors.Wrap(err, "invalid StatsD port")
	} else if pnum < 1024 || pnum > 65535 {
		return errors.Errorf("invalid StatsD port 1024>%s<65535", port)
	}

	// can be empty (all metrics go to host)
	// validate further if group check is enabled (see groupPrefix validation below)
	hostPrefix := viper.GetString(config.KeyStatsdHostPrefix)

	hostCat := viper.GetString(config.KeyStatsdHostCategory)
	if hostCat == "" {
		return errors.New("invalid StatsD host category (empty)")
	}

	groupCID := viper.GetString(config.KeyStatsdGroupCID)
	if groupCID == "" {
		return nil // statsd group check support disabled, all metrics go to host
	}

	if groupCID == "cosi" {
		cid, err := config.LoadCosiCheckID("group")
		if err != nil {
			return err
		}
		groupCID = cid
		viper.Set(config.KeyStatsdGroupCID, groupCID)
	}

	ok, err := config.IsValidCheckID(groupCID)
	if err != nil {
		return errors.Wrap(err, "StatsD Group Check ID")
	}
	if !ok {
		return errors.Errorf("invalid StatsD Group Check ID (%s)", groupCID)
	}

	groupPrefix := viper.GetString(config.KeyStatsdGroupPrefix)
	if hostPrefix == "" && groupPrefix == "" {
		return errors.New("StatsD host/group prefix mismatch (both empty)")
	}

	if hostPrefix == groupPrefix {
		return errors.New("StatsD host/group prefix mismatch (same)")
	}

	counterOp := viper.GetString(config.KeyStatsdGroupCounters)
	if counterOp == "" {
		return errors.New("invalid StatsD counter operator (empty)")
	}
	if ok, err := regexp.MatchString("^(average|sum)$", counterOp); err != nil {
		return errors.Wrapf(err, "invalid StatsD counter operator (%s)", counterOp)
	} else if !ok {
		return errors.Errorf("invalid StatsD counter operator (%s)", counterOp)
	}

	gaugeOp := viper.GetString(config.KeyStatsdGroupGauges)
	if gaugeOp == "" {
		return errors.New("invalid StatsD gauge operator (empty)")
	}
	if ok, err := regexp.MatchString("^(average|sum)$", gaugeOp); err != nil {
		return errors.Wrapf(err, "invalid StatsD gauge operator (%s)", gaugeOp)
	} else if !ok {
		return errors.Errorf("invalid StatsD gauge operator (%s)", gaugeOp)
	}

	setOp := viper.GetString(config.KeyStatsdGroupSets)
	if setOp == "" {
		return errors.New("invalid StatsD set operator (empty)")
	}
	if ok, err := regexp.MatchString("^(average|sum)$", setOp); err != nil {
		return errors.Wrapf(err, "invalid StatsD set operator (%s)", setOp)
	} else if !ok {
		return errors.Errorf("invalid StatsD set operator (%s)", setOp)
	}

	return nil
}
