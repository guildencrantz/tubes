package main

import (
	"log"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	DEFAULT_LOG_LEVEL = "info"
)

func init() {
	viper.SetDefault("log.level", DEFAULT_LOG_LEVEL)

	viper.SetConfigName("tubes") // TODO: Flags.
	//viper.AddConfigPath(os.Getenv("HOME") + "/.config")
	viper.AddConfigPath("$HOME/.config")
	viper.ReadInConfig()

	l, err := logrus.ParseLevel(viper.GetString("log.level"))
	if err != nil {
		log.Fatal("Unable to configure logger: ", err)
	}
	logrus.SetLevel(l)

	if strings.ToLower(viper.GetString("log.formatter")) == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	// TODO: Read ~/.ssh/config: https://github.com/kevinburke/ssh_config
}
