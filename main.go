package main

import (
	"github.com/goat-systems/baking-monitor/internal/config"
	"github.com/goat-systems/baking-monitor/internal/notifier/twilio"
	"github.com/goat-systems/baking-monitor/internal/watcher"
	"github.com/goat-systems/go-tezos/v3/rpc"
	"github.com/sirupsen/logrus"
)

func main() {
	hang := make(chan struct{})

	conf, err := config.New()
	if err != nil {
		logrus.WithField("error", err.Error()).Fatal("Failed to load configuration.")
	}

	tw := twilio.New(twilio.Client{
		AccountSID: conf.Twilio.AccountSID,
		AuthToken:  conf.Twilio.AuthToken,
		From:       conf.Twilio.From,
		To:         conf.Twilio.To,
	})

	r, err := rpc.New(conf.TezosAPI)
	if err != nil {
		logrus.WithField("error", err.Error()).Fatal("Failed to connect to Tezos RPC.")
	}

	w := watcher.New(conf.Baker, r, tw)
	w.Start()

	<-hang
}
