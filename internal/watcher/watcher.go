package watcher

import (
	"fmt"
	"time"

	"github.com/goat-systems/baking-monitor/internal/notifier/twilio"
	"github.com/goat-systems/go-tezos/v3/rpc"
	"github.com/sirupsen/logrus"
)

type Watcher struct {
	r        rpc.IFace
	notifier twilio.IFace
	delegate string
}

func New(delegate string, r rpc.IFace, notifier twilio.IFace) *Watcher {
	return &Watcher{
		r:        r,
		notifier: notifier,
		delegate: delegate,
	}
}

func (w *Watcher) Start() {
	logrus.Info("Starting baking monitor.")
	go func() {
		t := time.NewTicker(time.Minute)
		currentCycle := 0
		var doneBr, doneEr chan struct{}
		for range t.C {
			head, err := w.r.Head()
			if err != nil {
				logrus.WithField("error", err.Error()).Error("Failed to get current block height.")
				break
			}
			if head.Metadata.Level.Cycle > currentCycle {
				logrus.WithField("cycle", head.Metadata.Level.Cycle).Info("Updating current cycle rights.")
				if doneBr != nil && doneEr != nil {
					doneEr <- struct{}{}
					doneBr <- struct{}{}
				}

				br, err := w.r.BakingRights(rpc.BakingRightsInput{
					BlockHash:   head.Hash,
					Cycle:       head.Metadata.Level.Cycle,
					Delegate:    w.delegate,
					MaxPriority: 1,
				})
				if err != nil {
					logrus.WithFields(logrus.Fields{"error": err.Error(), "cycle": head.Metadata.Level.Cycle}).Error("failed to get baking rights")
					break
				}
				doneBr = make(chan struct{})
				w.watchBakingRights(br, doneBr)

				er, err := w.r.EndorsingRights(rpc.EndorsingRightsInput{
					BlockHash: head.Hash,
					Cycle:     head.Metadata.Level.Cycle,
					Delegate:  w.delegate,
				})
				if err != nil {
					logrus.WithFields(logrus.Fields{"error": err.Error(), "cycle": head.Metadata.Level.Cycle}).Error("failed to get endorsing rights")
					break
				}
				doneEr = make(chan struct{})
				w.watchEndorsements(er, doneEr)

				currentCycle = head.Metadata.Level.Cycle
			}
		}
	}()
}

func (w *Watcher) watchBakingRights(br *rpc.BakingRights, done chan struct{}) {
	t := time.NewTicker(time.Minute)
	for {
		select {
		case <-t.C:
			head, err := w.r.Head()
			if err != nil {
				logrus.WithField("error", err.Error()).Error("failed to get current block height")
			}
			for _, r := range *br {
				if r.Level == head.Header.Level && r.Priority == 0 {
					if head.Metadata.Baker != w.delegate {
						logrus.WithField("level", head.Header.Level).Info("Missed Baking Opportunity")
						w.notifier.Send(fmt.Sprintf("Missed Baking Opportunity at level '%d'", head.Header.Level))
					} else {
						logrus.WithField("level", head.Header.Level).Info("Successfully Baked Block")
					}
				}
			}
		case <-done:
			break
		}
	}
}

func (w *Watcher) watchEndorsements(er *rpc.EndorsingRights, done chan struct{}) {
	t := time.NewTicker(time.Minute)
	for {
		select {
		case <-t.C:
			head, err := w.r.Head()
			if err != nil {
				logrus.WithField("error", err.Error()).Error("failed to get current block height")
			}
			for _, r := range *er {
				if r.Level == head.Header.Level-1 {
					var found bool
					for _, operations := range head.Operations {
						for _, operation := range operations {
							contents := operation.Contents.Organize()
							for _, endorsement := range contents.Endorsements {
								if endorsement.Metadata.Delegate == w.delegate {
									found = true
								}
							}
						}
					}
					if !found {
						w.notifier.Send(fmt.Sprintf("Missed Endorsement Opportunity at level '%d'", r.Level))
						logrus.WithField("level", r.Level).Info("Missed Endorsement Opportunity")
					} else {
						logrus.WithField("level", r.Level).Info("Successfully Endorsed Block")
					}
				}
			}
		case <-done:
			break
		}
	}
}
