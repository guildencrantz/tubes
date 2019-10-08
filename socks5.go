package main

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/armon/go-socks5"
	"github.com/getlantern/systray"

	"golang.org/x/net/context"
	"golang.org/x/net/proxy"
)

// EmptyResolver prevents DNS from resolving on the local machine, rather than over the SSH connection.
type EmptyResolver struct{}

func (EmptyResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	return ctx, nil, nil
}

type SOCKS5 struct {
	Name      string
	LocalBind string
	Port      int
	Disabled  bool
	SSH       *SSH
	HTTP      string // Act as HTTP proxy, listinening on this address, routing traffic over the SOCKS5 connection. (if this string is just a port, 127.0.0.1 will be used as the address).
	httProxy  *httProxy
	local     net.Listener
	server    *socks5.Server
	menu      *systray.MenuItem
}

func (s *SOCKS5) Finalize() {
	if s.Name == "" {
		s.Name = fmt.Sprintf("SOCKS5 %d", s.Port)
	}

	if s.LocalBind == "" {
		s.LocalBind = "127.0.0.1"
	}

	if s.HTTP == "" || strings.Contains(s.HTTP, ":") { // HTTP is not set, or appears to be a full address (laziest of checks)
		return
	}

	if _, err := strconv.Atoi(s.HTTP); err != nil {
		logrus.WithFields(logrus.Fields{
			"method": "SOCKS5.Finalise",
			"HTTP":   s.HTTP,
		}).Panic("Unable to parse HTTP proxy address")
	}

	s.HTTP = "127.0.0.1:" + s.HTTP
}

func (s *SOCKS5) Listen() (err error) {
	log := logrus.WithField("method", "SOCKS5.Listen")

	if s.Disabled {
		return TunnelDisabledError{}
	}

	if s.local == nil {
		s.local, err = net.Listen("tcp", fmt.Sprintf("%s:%d", s.LocalBind, s.Port))
		if err != nil {
			return err
		}
	}

	if s.HTTP != "" && s.httProxy == nil {
		log.Info("Starting HTTP Proxy")
		dialer, err := proxy.SOCKS5("tcp", s.local.Addr().String(), nil, proxy.Direct)
		if err != nil {
			log.WithError(err).Panic("Unable to create SOCKS5 dialer")
		}

		s.httProxy = &httProxy{
			Server: &http.Server{
				Addr: s.HTTP,
			},
			dialer: dialer,
		}

		go func() {
			if err := s.httProxy.ListenAndServe(); err != nil {
				log.WithError(err).Panic("httProxy serve error")
			}
		}()
	}

	for {
		c, err := s.local.Accept()
		if err != nil {
			return err
		}

		// Now I'm regretting how I'm handling the connections.
		// TODO: Tighten up connection management (besides all else Tunnel should default to making a direct connection, and using an external entity should be an override).
		if s.SSH != nil && s.SSH.Client == nil {
			if err := s.SSH.Connect(); err != nil {
				return err
			}
		}

		if s.server == nil {
			srv, err := socks5.New(
				&socks5.Config{
					Resolver: EmptyResolver{},
					Dial: func(ctx context.Context, n, a string) (net.Conn, error) {
						return s.SSH.Client.Dial(n, a)
					},
				},
			)
			if err != nil {
				log.WithError(err).Panic("Unable to create SOCKS5 server")
			}

			s.server = srv
		}

		go func() {
			if err := s.server.ServeConn(c); err != nil {
				log.WithError(err).Error("Unable to serve connection")
			}
		}()
	}

	return nil
}

func (s *SOCKS5) systray() {
	s.menu = systray.AddMenuItem(s.Name, strconv.Itoa(s.Port))
	if !s.Disabled {
		s.menu.Check()
	}
	go s.handleClicks()
}

func (s *SOCKS5) handleClicks() {
	log := logrus.WithField("type", "SOCKS5").WithField("method", "handleClicks")

	for range s.menu.ClickedCh {
		if s.Disabled {
			log.Info("Enabling tunnel")
			s.Disabled = false
			go s.Listen()
			s.menu.Check()
		} else {
			log.Warn("Disabling tunnel")
			s.Disabled = true
			s.local.Close()
			s.local = nil
			s.server = nil
			s.menu.Uncheck()
		}
	}
}

func (s *SOCKS5) Close() {
	log := s.logger("Close")
	log.Warn("Closing")
	defer log.Info("Closed")
	if s.local != nil {
		s.local.Close()
	}

	if s.menu != nil {
		s.menu.Hide()
	}
}

func (s *SOCKS5) logger(method string) *logrus.Entry {
	return logrus.WithField("type", "SOCKS5").
		WithField("method", method).
		WithField("LocalBind", s.LocalBind).
		WithField("Port", s.Port)
}
