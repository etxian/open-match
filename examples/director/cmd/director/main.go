package main

import (
	"context"
	"errors"
	"open-match.dev/open-match/examples/director/pkg/agones"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"open-match.dev/open-match/examples/director/config"
)

var (
	// Profiles debugging config:
	maxSends          int
	maxMatchesPerSend int
	sleepBetweenSends = 30 * time.Second

	dirLogFields = log.Fields{
		"app":       "open-match",
		"component": "director",
	}
	dirLog = log.WithFields(dirLogFields)

	allocator Allocator
	cfg = viper.New()

	err = errors.New("")
)

func init() {
	cfg, err = config.Read()
	if err != nil {
		dirLog.WithError(err).Fatal("Unable to load config file")
	}
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	dirLog.WithField("cfg", cfg.AllSettings()).Info("Configuration provided")

	// Profiles debugging
	maxSends = cfg.GetInt("debug.maxSends")
	maxMatchesPerSend = cfg.GetInt("debug.maxMatchesPerSend")
	sleepBetweenSends = time.Duration(cfg.GetInt64("debug.sleepBetweenSendsSeconds") * int64(time.Second))

	var namespace, fleetName, generateName string
	if namespace = cfg.GetString("agones.namespace"); namespace == "" {
		dirLog.Fatalf("Incomplete Agones configuration: missing \"agones.namespace\"")
	}
	if fleetName = cfg.GetString("agones.fleetName"); fleetName == "" {
		dirLog.Fatalf("Incomplete Agones configuration: missing \"agones.fleetName\"")
	}
	if generateName = cfg.GetString("agones.generateName"); generateName == "" {
		dirLog.Fatalf("Incomplete Agones configuration: missing \"agones.generateName\"")
	}

	allocator, err = agones.NewGameServerAllocator(namespace, fleetName, generateName, dirLog)
	if err != nil {
		dirLog.WithError(err).Fatal("Could not create Agones allocator")
	}
}

func main() {
	profile, mmfcfg, err := readProfile("profile.json")
	if err != nil {
		dirLog.WithError(err).Fatalf(`error reading file "profile.json": %s`, err.Error())
	}

	startSendProfile(context.Background(), profile, mmfcfg, dirLog)
}