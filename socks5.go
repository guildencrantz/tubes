package main

import (
	"fmt"
	"net"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/armon/go-socks5"
	"github.com/getlantern/systray"

	"golang.org/x/net/context"
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

}

func (s *SOCKS5) Listen() (err error) {
	if s.Disabled {
		return TunnelDisabledError{}
	}

	if s.local == nil {
		s.local, err = net.Listen("tcp", fmt.Sprintf("%s:%d", s.LocalBind, s.Port))
		if err != nil {
			return err
		}
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
				logrus.WithError(err).Panic("Unable to create SOCKS5 server")
			}

			s.server = srv
		}

		go func() {
			if err := s.server.ServeConn(c); err != nil {
				logrus.WithError(err).Error("Unable to serve connection")
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
