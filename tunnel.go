package main

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/getlantern/systray"
	//https://github.com/armon/go-socks5
)

type TunnelDisabledError struct{}

func (e TunnelDisabledError) String() string {
	return "Tunnel Disabled"
}
func (e TunnelDisabledError) Error() string {
	return e.String()
}

type Tunnel struct {
	//TODO: Remote flag
	Name       string
	LocalBind  string
	Port       int
	RemoteBind string
	RemotePort *int
	Disabled   bool
	SSH        *SSH
	local      net.Listener
	remote     net.Conn
	menu       *systray.MenuItem
}

func (t *Tunnel) Finalize() {
	if t.Name == "" {
		t.Name = "TODO: Create a tunnel name based on the binds and ports"
	}

	if t.LocalBind == "" {
		t.LocalBind = "127.0.0.1"
	}

	if t.RemoteBind == "" {
		t.RemoteBind = "127.0.0.1"
	}

	if t.RemotePort == nil {
		t.RemotePort = IntPtr(t.Port)
	}
}

func (t *Tunnel) Listen() (err error) {
	if t.Disabled {
		return TunnelDisabledError{}
	}

	log := t.logger("Listen")
	// TODO: Non-tcp

	if t.local == nil {
		t.local, err = net.Listen("tcp", fmt.Sprintf("%s:%d", t.LocalBind, t.Port))
		if err != nil {
			return err
		}
		log.Debug("listening")
	}

	for {
		c, err := t.local.Accept()
		if err != nil {
			log.WithError(err)
			return err
		}
		log.Debug("Connection accepted")

		// Now I'm regretting how I'm handling the connections.
		// TODO: Tighten up connection management (besides all else Tunnel should default to making a direct connection, and using an external entity should be an override).
		if t.SSH != nil && t.SSH.Client == nil {
			if err = t.SSH.Connect(); err != nil {
				return err
			}
		}

		addr := fmt.Sprintf("%s:%d", t.RemoteBind, *t.RemotePort)
		log.Debug("dialing")

		r, err := t.SSH.Dial("tcp", addr)
		if err != nil {
			return err
		}

		plumb(c, r)
	}

	return nil
}

func (t *Tunnel) Close() {
	log := t.logger("Close")
	log.Warn("Closing")
	defer log.Info("Closed")
	if t.local != nil {
		t.local.Close()
	}
	if t.remote != nil {
		t.remote.Close()
	}

	if t.menu != nil {
		t.menu.Hide()
	}
}

func (t *Tunnel) systray() {
	t.menu = systray.AddMenuItem(t.Name, "")
	if !t.Disabled {
		t.menu.Check()
	}
	go t.handleClicks()
}

func (t *Tunnel) handleClicks() {
	log := t.logger("handleClicks")

	for range t.menu.ClickedCh {
		if t.Disabled {
			log.Info("Enabling tunnel")
			t.Disabled = false
			go t.Listen()
			t.menu.Check()
		} else {
			log.Warn("Disabling tunnel")
			t.Disabled = true
			t.local.Close()
			t.local = nil
			t.menu.Uncheck()
		}
	}
}

func (t *Tunnel) logger(method string) *logrus.Entry {
	return logrus.WithField("type", "Tunnel").
		WithField("method", method).
		WithField("LocalBind", t.LocalBind).
		WithField("Port", t.Port).
		WithField("RemoteBind", t.RemoteBind).
		WithField("RemotePort", t.RemotePort)
}

// Copy the bytes from a to b, and b to a (bidirectionally connect the ports). When copying completes (error or otherwise) both connections are closed.
func plumb(a, b net.Conn) {
	log := logrus.WithField("function", "plumb")
	// TODO: What to do on copy errors?

	var once sync.Once
	close := func() {
		a.Close()
		b.Close()
	}

	go func() {
		defer once.Do(close)
		if _, err := io.Copy(a, b); err != nil {
			log.WithError(err)
		}
	}()

	// Copy remote to local
	go func() {
		defer once.Do(close)
		if _, err := io.Copy(b, a); err != nil {
			log.WithError(err)
		}
	}()
}
