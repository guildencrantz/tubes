package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/Sirupsen/logrus"
	"github.com/getlantern/systray"
)

type SSH struct {
	*ssh.Client
	Name        string
	User        string
	Hostname    string
	Port        *int
	Tunnels     []*Tunnel
	SOCKS5s     []*SOCKS5
	AuthMethods []ssh.AuthMethod
	menu        *systray.MenuItem
}

func (s *SSH) Finalize() {
	if s.Name == "" {
		var p string
		if s.Port != nil {
			p = fmt.Sprintf(":%d", *s.Port)
		}
		s.Name = fmt.Sprintf("%s@%s%s", s.User, s.Hostname, p)
	}

	if len(s.AuthMethods) == 0 {
		s.AuthMethods = DefaultSSHAuthMethods()
	}

	if s.Port == nil {
		s.Port = IntPtr(22)
	}
}

func (s *SSH) Connect() error {
	log := s.logger("Connect")
	cfg := &ssh.ClientConfig{
		Auth:            s.AuthMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // FIXME
		Timeout:         15 * time.Second,            // TODO: Config
	}
	if s.User != "" {
		cfg.User = s.User
	}

	addr := fmt.Sprintf("%s:%d", s.Hostname, *s.Port)
	log.Debug("Dialing")
	client, err := ssh.Dial("tcp", addr, cfg) // FIXME: Why is `Timeout` not working when the connection comes in on the port directly? When the host is clicked it works, though?
	// Also might just be worth implementing: https://stackoverflow.com/questions/43116757/golang-ssh-dial-timeout
	if err != nil {
		return err
	}
	log.Debug("Connected")
	s.Client = client

	if s.menu != nil {
		s.menu.Check()
	}

	return nil
}

func (s *SSH) NewSession() (*ssh.Session, error) {
	if s.Client == nil {
		s.Connect()
	}
	return s.Client.NewSession()
}

// Dial connects to network, address (first creating the SSH client if it hasn't been created yet).
func (s *SSH) Dial(network, address string) (net.Conn, error) {
	log := s.logger("Dial")
	if s.Client == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
	}

	log.WithField("network", network).WithField("address", address).Debug("Dialing")
	return s.Client.Dial(network, address)
}

func (s *SSH) Tunnel() error {
	log := s.logger("Tunnel")

	for _, t := range s.Tunnels {
		t.SSH = s
		t.Finalize()
		log.WithField("tunnel", t).Debug("tunnel")
		go t.Listen()
	}

	for _, p := range s.SOCKS5s {
		p.SSH = s
		p.Finalize()
		log.WithField("SOCKS5", p).Debug("SOCKS5")
		go p.Listen()
	}

	return nil
}

func (s *SSH) Close() {
	log := s.logger("Close")
	log.Warn("Closing")
	defer log.Info("Closed")
	for _, t := range s.Tunnels {
		t.Close()
	}

	for _, p := range s.SOCKS5s {
		p.Close()
	}

	if s.Client != nil {
		s.Client.Close()
	}

	if s.menu != nil {
		// No remove?
		s.menu.Hide()
	}
}

func (s *SSH) systray() {
	systray.AddSeparator()
	s.menu = systray.AddMenuItem(s.Name, "")
	go s.handleClicks()
	for _, t := range s.Tunnels {
		t.systray()
	}

	for _, p := range s.SOCKS5s {
		p.systray()
	}
}

func (s *SSH) logger(method string) *logrus.Entry {
	return logrus.WithField("type", "SSH").WithField("method", method).WithField("ssh hostname", s.Hostname).WithField("ssh port", *s.Port)
}

func (s *SSH) handleClicks() {
	log := s.logger("handleClicks")
	for range s.menu.ClickedCh {
		if s.menu.Checked() {
			log.Info("Disconnecting")
			s.Client.Close()
			s.Client = nil
			s.menu.Uncheck()
		} else {
			if err := s.Connect(); err != nil {
				log.WithError(err).Error("Unable to connect")
			}
		}
	}
}

func DefaultSSHAuthMethods() []ssh.AuthMethod {
	addr := os.Getenv("SSH_AUTH_SOCK")
	log := logrus.WithField("function", "DefaultSSHAuthMethods").WithField("SSH_AUTH_SOCK", addr)

	sock, err := net.Dial("unix", addr)
	if err != nil {
		log.WithError(err).Fatal("Unable to connect to ssh agent")
	}

	agent := agent.NewClient(sock)
	signers, err := agent.Signers()
	if err != nil {
		log.WithError(err).Fatal("Unable to get SSH Agent signers")
	}

	// TODO: It'd be nice if the auths could be updated, especially if it could happen on failure rather than relying on timed polling.
	return []ssh.AuthMethod{ssh.PublicKeys(signers...)}
}
