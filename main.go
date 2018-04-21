package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/getlantern/systray"
	"github.com/spf13/viper"
)

func main() {
	sc := []*SSH{}
	// TODO: viper.OnConfigChange
	viper.UnmarshalKey("ssh", &sc)

	systray.Run(
		func() { // OnReady
			systray.SetTitle("tubes")

			r := systray.AddMenuItem("Restart", "")
			go func() {
				for range r.ClickedCh {
					sc = restart(sc)
				}
			}()
			q := systray.AddMenuItem("Quit", "")
			go func() {
				<-q.ClickedCh
				systray.Quit()
			}()

			systray.AddSeparator()

			for _, s := range sc {
				s.Finalize()
				s.systray() // Add the menu item before tunneling so connection indicators show correctly.
				s.Tunnel()
			}
		},
		func() { // OnExit
			for _, s := range sc {
				s.Close()
			}
		},
	)
}

func restart(sc []*SSH) []*SSH {
	log := logrus.WithField("function", "restart")
	log.Warn("Restarting")
	defer log.Info("Restart complete")

	for _, s := range sc {
		s.Close()
	}

	sc = []*SSH{}

	viper.UnmarshalKey("ssh", &sc)
	for _, s := range sc {
		s.Finalize()
		s.systray()
		s.Tunnel()
	}

	return sc
}
