package main

import (
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/armon/go-socks5"
	"github.com/getlantern/systray"

	"golang.org/x/net/context"
)

type SOCKS5 struct {
	Name      string
	LocalBind string
	Port      int
	Disabled  bool
	SSH       *SSH
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

func (s *SOCKS5) Listen() error {
	if s.Disabled {
		return TunnelDisabledError{}
	}

	if s.server == nil {
		srv, err := socks5.New(
			&socks5.Config{
				Dial: func(ctx context.Context, n, a string) (net.Conn, error) { return s.SSH.Client.Dial(n, a) },
			},
		)
		if err != nil {
			logrus.WithError(err).Panic("Unable to create SOCKS5 server")
		}

		if err := srv.ListenAndServe("tcp", fmt.Sprintf("%s:%d", s.LocalBind, s.Port)); err != nil {
			logrus.WithError(err).Panic("Unable to create socks listener")
		}

		s.server = srv
	}

	return nil
}

func (s *SOCKS5) systray() {
	s.menu = systray.AddMenuItem(s.Name, "")
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
			s.server = nil
			s.menu.Uncheck()
		}
	}
}
