// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
	"bufio"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/maier/go-appstats"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// Server defines a statsd server
type Server struct {
	disabled              bool
	enableUDPListener     bool // NOTE: defaults to TRUE; uses !disabled (not really a separate option)
	enableTCPListener     bool // NOTE: defaults to FALSE
	debugCGM              bool
	group                 *errgroup.Group
	groupCtx              context.Context
	udpAddress            *net.UDPAddr
	tcpAddress            *net.TCPAddr
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
	nameSpaceReplaceRx    *regexp.Regexp
	metricRegexGroupNames []string
	apiKey                string
	apiApp                string
	apiURL                string
	apiCAFile             string
	udpListener           *net.UDPConn
	tcpListener           *net.TCPListener
	tcpMaxConnections     uint
	tcpConnections        map[string]*net.TCPConn
	baseTags              []string
	sync.Mutex
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
		group:             g,
		groupCtx:          gctx,
		disabled:          viper.GetBool(config.KeyStatsdDisabled),
		logger:            log.With().Str("pkg", "statsd").Logger(),
		hostPrefix:        viper.GetString(config.KeyStatsdHostPrefix),
		hostCategory:      viper.GetString(config.KeyStatsdHostCategory),
		groupCID:          viper.GetString(config.KeyStatsdGroupCID),
		groupPrefix:       viper.GetString(config.KeyStatsdGroupPrefix),
		groupCounterOp:    viper.GetString(config.KeyStatsdGroupCounters),
		groupGaugeOp:      viper.GetString(config.KeyStatsdGroupGauges),
		groupSetOp:        viper.GetString(config.KeyStatsdGroupSets),
		debugCGM:          viper.GetBool(config.KeyDebugCGM),
		apiKey:            viper.GetString(config.KeyAPITokenKey),
		apiApp:            viper.GetString(config.KeyAPITokenApp),
		apiURL:            viper.GetString(config.KeyAPIURL),
		apiCAFile:         viper.GetString(config.KeyAPICAFile),
		baseTags:          tags.GetBaseTags(),
		tcpConnections:    map[string]*net.TCPConn{},
		tcpMaxConnections: viper.GetUint(config.KeyStatsdMaxTCPConns),
	}

	s.enableUDPListener = !s.disabled
	s.enableTCPListener = viper.GetBool(config.KeyStatsdEnableTCP)

	s.baseTags = append(s.baseTags, []string{
		"source:" + release.NAME,
		"collector:statsd",
	}...)

	// standard statsd metric format supported (with addition of tags):
	// name:value|type[|@rate][|#tag_list]
	// where tag_list is comma separated list of <tag_category:tag_value> pairs
	s.metricRegex = regexp.MustCompile(`^(?P<name>[^:]+):(?P<value>[^|]+)\|(?P<type>[a-z]+)(?:\|@(?P<sample>[0-9.]+))?(?:\|#(?P<tags>[^:,]+:[^:,]+(,[^:,]+:[^:,]+)*))?$`)
	s.metricRegexGroupNames = s.metricRegex.SubexpNames()
	s.nameSpaceReplaceRx = regexp.MustCompile(`\s+`)

	if !s.disabled {
		if ierr := s.initHostMetrics(); ierr != nil {
			return nil, fmt.Errorf("initializing host metrics for StatsD: %w", ierr)
		}

		if ierr := s.initGroupMetrics(); ierr != nil {
			return nil, fmt.Errorf("initializing group metrics for StatsD: %w", ierr)
		}
	}

	addr := viper.GetString(config.KeyStatsdAddr)
	if addr == "" {
		addr = defaults.StatsdAddr
	}
	port := viper.GetString(config.KeyStatsdPort)
	if port == "" {
		port = defaults.StatsdPort
	}
	address := net.JoinHostPort(addr, port)
	// UDP listening address
	if s.enableUDPListener {
		addr, err := net.ResolveUDPAddr("udp", address)
		if err != nil {
			return nil, fmt.Errorf("resolving UDP address '%s': %w", address, err)
		}
		s.udpAddress = addr
	}
	// TCP listening address
	if s.enableTCPListener {
		addr, err := net.ResolveTCPAddr("tcp", address)
		if err != nil {
			return nil, fmt.Errorf("resolving TCP address '%s': %w", address, err)
		}
		s.tcpAddress = addr
	}

	return &s, nil
}

// Start the StatsD service
func (s *Server) Start() error {
	if s.disabled {
		s.logger.Info().Msg("disabled, not starting listener")
		return nil
	}

	if err := s.startUDP(); err != nil {
		return fmt.Errorf("starting UDP listener: %w", err)
	}
	if err := s.startTCP(); err != nil {
		return fmt.Errorf("starting TCP listener: %w", err)
	}

	packetCh := make(chan []byte, packetQueueSize)

	if s.enableUDPListener && s.udpListener != nil {
		s.group.Go(func() error {
			s.logger.Debug().Msg("starting udp listener")
			return s.udpReader(packetCh)
		})
	}
	if s.enableTCPListener && s.tcpListener != nil {
		s.group.Go(func() error {
			s.logger.Debug().Msg("starting tcp listener")
			return s.tcpHandler(packetCh)
		})
	}
	s.group.Go(func() error {
		s.logger.Debug().Msg("starting packet processor")

		return s.processor(packetCh)
	})

	go func() {
		s.logger.Debug().Msg("waiting for group")

		_ = s.group.Wait()
		close(packetCh)
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

// startUDP the StatsD UDP listener
func (s *Server) startUDP() error {
	if !s.enableUDPListener {
		return nil
	}
	if s.udpAddress == nil {
		return nil
	}
	l, err := net.ListenUDP("udp", s.udpAddress)
	if err != nil {
		return fmt.Errorf("starting statsd udp listener: %w", err)
	}
	s.udpListener = l
	return nil
}

// startTCP the StatsD TCP listener
func (s *Server) startTCP() error {
	if !s.enableTCPListener {
		return nil
	}
	if s.tcpAddress == nil {
		return nil
	}
	l, err := net.ListenTCP("tcp", s.tcpAddress)
	if err != nil {
		return fmt.Errorf("starting statsd tcp listener: %w", err)
	}
	s.tcpListener = l
	return nil
}

// initHostMetrics initializes the host metrics circonus-gometrics instance
func (s *Server) initHostMetrics() error {
	s.hostMetricsmu.Lock()
	defer s.hostMetricsmu.Unlock()

	cmc := &cgm.Config{}

	// put cgm into manual mode (no interval, no api key, invalid submission url)
	cmc.Interval = "0"                            // disable automatic flush
	cmc.CheckManager.Check.SubmissionURL = "none" // disable check management (create/update)

	hm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		return fmt.Errorf("statsd host check: %w", err)
	}

	s.hostMetrics = hm

	s.logger.Info().Msg("host check initialized")
	return nil
}

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue)
type logshim struct {
	logh zerolog.Logger
}

func (l logshim) Printf(msgfmt string, v ...interface{}) {
	l.logh.Info().Msg(fmt.Sprintf(msgfmt, v...))
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

	cmc := &cgm.Config{}
	if s.debugCGM {
		cmc.Debug = s.debugCGM
		cmc.Log = logshim{logh: s.logger.With().Str("pkg", "cgm.statsd-group-check").Logger()}
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
			return fmt.Errorf("using api CA cert %#v", cert)
		}

		cmc.CheckManager.API.CACert = cp
	}

	gm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		return fmt.Errorf("statsd group check: %w", err)
	}

	s.groupMetrics = gm

	s.logger.Info().Msg("group check initialized")
	return nil
}

// udpReader reads packets from the statsd udp listener, adds packets recevied to the queue
func (s *Server) udpReader(packetCh chan<- []byte) error {
	for {
		if s.done() {
			return nil
		}
		buff := make([]byte, maxPacketSize)
		n, err := s.udpListener.Read(buff)
		if err != nil {
			s.logger.Warn().Err(err).Msg("udp reader")
			continue
		}
		if n > 0 {
			_ = appstats.IncrementInt("statsd_packets_total")
			pkt := make([]byte, n)
			copy(pkt, buff[:n])
			packetCh <- pkt
		}
	}
}

// tcpHandler reads packets from the statsd tcp listener, adds packets recevied to the queue
func (s *Server) tcpHandler(packetCh chan<- []byte) error {
	for {
		if s.done() {
			return nil
		}
		conn, err := s.tcpListener.AcceptTCP()
		if err != nil {
			s.logger.Warn().Err(err).Msg("accepting tcp connection")
			continue
		}
		s.Lock()
		if uint(len(s.tcpConnections)) > s.tcpMaxConnections {
			s.tcpRefuseConnection(conn)
			s.Unlock()
			continue
		}
		s.Unlock()
		s.tcpAddConnection(conn)
		go func(conn *net.TCPConn) {
			if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				s.logger.Warn().Err(err).Msg("setting statsd tcp connection deadline")
			}
			if err := s.tcpReader(conn, packetCh); err != nil {
				s.logger.Warn().Err(err).Msg("handling tcp connection")
			}
		}(conn)
	}
}

// tcpReader reads packets from the statsd tcp listener, adds packets recevied to the queue
func (s *Server) tcpReader(conn *net.TCPConn, packetCh chan<- []byte) error {
	addr := conn.RemoteAddr().String()
	defer func() {
		s.logger.Debug().Str("remote", addr).Msg("closing statsd tcp connection")
		conn.Close()
		s.tcpRemoveConnection(addr)
	}()

	for {
		if s.done() {
			return nil
		}
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			_ = appstats.IncrementInt("statsd_packets_total")
			packetCh <- scanner.Bytes()
		}
		if s.done() {
			return nil
		}
		if err := scanner.Err(); err != nil {
			s.logger.Debug().Err(err).Str("remote", addr).Msg("statsd tcp conn scanner error")

			var nerr net.Error
			if errors.As(err, &nerr) && nerr.Timeout() {
				s.logger.Debug().Err(nerr).Str("remote", addr).Msg("resetting deadline")
				if derr := conn.SetDeadline(time.Now().Add(10 * time.Second)); derr != nil {
					return derr
				}
				continue
			}

			return err
		}
	}
}

// tcpRefuseConnection refuses a tcp client connection and logs the event
func (s *Server) tcpRefuseConnection(conn *net.TCPConn) {
	conn.Close()
	s.logger.Warn().Str("remote", conn.RemoteAddr().String()).Msg("max tcp client connections reached, refusing new connection attempt")
}

// tcpAddConnection tracks tcp client connections
func (s *Server) tcpAddConnection(conn *net.TCPConn) {
	s.Lock()
	s.tcpConnections[conn.RemoteAddr().String()] = conn
	s.Unlock()
}

// tcpRemoveConnection removes a tracked tcp client connection from tracking list
func (s *Server) tcpRemoveConnection(id string) {
	s.Lock()
	delete(s.tcpConnections, id)
	s.Unlock()
}

// processor reads the packet queue and processes each packet
func (s *Server) processor(packetCh <-chan []byte) error {
	for {
		select {
		case <-s.groupCtx.Done():
			if s.udpListener != nil {
				s.logger.Debug().Msg("closing udp listener")
				s.udpListener.Close()
			}
			if s.tcpListener != nil {
				s.Lock()
				s.logger.Debug().Msg("closing tcp listener")
				s.tcpListener.Close()
				if len(s.tcpConnections) > 0 {
					s.logger.Debug().Msg("closing tcp connections")
					var connList []*net.TCPConn
					for _, conn := range s.tcpConnections {
						connList = append(connList, conn)
					}
					for _, conn := range connList {
						conn.Close()
					}
				}
				s.Unlock()
			}
			return nil
		case pkt := <-packetCh:
			s.processPacket(pkt)
		}
	}
}

// done checks whether context is done
func (s *Server) done() bool {
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
		return fmt.Errorf("invalid StatsD port (empty)")
	}
	if ok, err := regexp.MatchString("^[0-9]+$", port); err != nil {
		return fmt.Errorf("invalid StatsD port (%s): %w", port, err)
	} else if !ok {
		return fmt.Errorf("invalid StatsD port (%s)", port)
	}
	if pnum, err := strconv.ParseUint(port, 10, 32); err != nil {
		return fmt.Errorf("invalid StatsD port: %w", err)
	} else if pnum < 1024 || pnum > 65535 {
		return fmt.Errorf("invalid StatsD port 1024>%s<65535", port)
	}

	// can be empty (all metrics go to host)
	// validate further if group check is enabled (see groupPrefix validation below)
	hostPrefix := viper.GetString(config.KeyStatsdHostPrefix)

	hostCat := viper.GetString(config.KeyStatsdHostCategory)
	if hostCat == "" {
		return fmt.Errorf("invalid StatsD host category (empty)")
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
		return fmt.Errorf("validating StatsD Group Check ID: %w", err)
	}
	if !ok {
		return fmt.Errorf("invalid StatsD Group Check ID (%s)", groupCID)
	}

	groupPrefix := viper.GetString(config.KeyStatsdGroupPrefix)
	if hostPrefix == "" && groupPrefix == "" {
		return fmt.Errorf("invalid StatsD host/group prefix (both empty)")
	}

	if hostPrefix == groupPrefix {
		return fmt.Errorf("invalid StatsD host/group prefix (same)")
	}

	counterOp := viper.GetString(config.KeyStatsdGroupCounters)
	if counterOp == "" {
		return fmt.Errorf("invalid StatsD counter operator (empty)")
	}
	if ok, err := regexp.MatchString("^(average|sum)$", counterOp); err != nil {
		return fmt.Errorf("invalid StatsD counter operator (%s): %w", counterOp, err)
	} else if !ok {
		return fmt.Errorf("invalid StatsD counter operator (%s)", counterOp)
	}

	gaugeOp := viper.GetString(config.KeyStatsdGroupGauges)
	if gaugeOp == "" {
		return fmt.Errorf("invalid StatsD gauge operator (empty)")
	}
	if ok, err := regexp.MatchString("^(average|sum)$", gaugeOp); err != nil {
		return fmt.Errorf("invalid StatsD gauge operator (%s): %w", gaugeOp, err)
	} else if !ok {
		return fmt.Errorf("invalid StatsD gauge operator (%s)", gaugeOp)
	}

	setOp := viper.GetString(config.KeyStatsdGroupSets)
	if setOp == "" {
		return fmt.Errorf("invalid StatsD set operator (empty)")
	}
	if ok, err := regexp.MatchString("^(average|sum)$", setOp); err != nil {
		return fmt.Errorf("invalid StatsD set operator (%s): %w", setOp, err)
	} else if !ok {
		return fmt.Errorf("invalid StatsD set operator (%s)", setOp)
	}

	return nil
}
