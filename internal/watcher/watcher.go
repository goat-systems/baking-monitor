package watcher

import (
	"fmt"
	"time"

	"github.com/goat-systems/baking-monitor/internal/notifier/twilio"
	"github.com/goat-systems/go-tezos/v3/rpc"
	"github.com/google/uuid"
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
					MaxPriority: 0,
				})
				if err != nil {
					logrus.WithFields(logrus.Fields{"error": err.Error(), "cycle": head.Metadata.Level.Cycle}).Error("failed to get baking rights")
					break
				}
				logrus.WithFields(logrus.Fields{"rights": fmt.Sprintf("%+v", br), "cycle": head.Metadata.Level.Cycle}).Info("Received baking rights for cycle.")
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
				logrus.WithFields(logrus.Fields{"rights": fmt.Sprintf("%+v", er), "cycle": head.Metadata.Level.Cycle}).Info("Received endorsement rights for cycle.")
				doneEr = make(chan struct{})
				w.watchEndorsements(er, doneEr)

				currentCycle = head.Metadata.Level.Cycle
			}
		}
	}()
}

func (w *Watcher) watchBakingRights(br *rpc.BakingRights, done chan struct{}) {
	uuid := uuid.New()
	logrus.WithField("uuid", uuid.String()).Info("Starting new baking rights worker.")
	t := time.NewTicker(time.Minute)
	go func() {
		for {
			select {
			case <-t.C:
				head, err := w.r.Head()
				if err != nil {
					logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "error": err.Error()}).Error("Failed to get current block height.")
				}
				logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "block": head.Header.Level}).Info("Baking rights worker found new block.")

				for _, r := range *br {
					if r.Level == head.Header.Level && r.Priority == 0 {
						logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "block": head.Header.Level}).Info("Baking rights worker found baking slot for priority 0.")
						if head.Metadata.Baker != w.delegate {
							logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "block": head.Header.Level}).Error("Missed Baking Opportunity.")
							w.notifier.Send(fmt.Sprintf("Missed Baking Opportunity at level '%d'.", head.Header.Level))
						} else {
							logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "block": head.Header.Level}).Info("Successfully Baked Block.")
						}
					}
				}
			case <-done:
				logrus.WithField("uuid", uuid.String()).Info("Ending baking rights worker.")
				break
			}
		}
	}()
}

func (w *Watcher) watchEndorsements(er *rpc.EndorsingRights, done chan struct{}) {
	uuid := uuid.New()
	logrus.WithField("uuid", uuid.String()).Info("Starting new endorsement rights worker.")
	t := time.NewTicker(time.Minute)

	go func() {
		for {
			select {
			case <-t.C:
				head, err := w.r.Head()
				if err != nil {
					logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "error": err.Error()}).Error("Failed to get current block height.")
				}
				logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "block": head.Header.Level}).Info("Endorsing rights worker found new block.")

				for _, r := range *er {
					if r.Level == head.Header.Level-1 {
						logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "right-level": r.Level, "block": head.Header.Level}).Info("Endorsing rights worker found endorsing slot.")
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
							logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "right-level": r.Level, "block": head.Header.Level}).Error("Missed Endorsing Opportunity.")
							w.notifier.Send(fmt.Sprintf("Missed Endorsement Opportunity at level '%d'", r.Level))
						} else {
							logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "right-level": r.Level, "block": head.Header.Level}).Info("Successfully Endorsed Block.")
						}
					}
				}
			case <-done:
				break
			}
		}
	}()
}
