package watcher

import (
	"fmt"
	"time"

	"github.com/goat-systems/baking-monitor/internal/notifier/twilio"
	"github.com/goat-systems/go-tezos/v3/rpc"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type cycleChan struct {
	Hash  string
	Cycle int
}

type Watcher struct {
	r                           rpc.IFace
	notifier                    twilio.IFace
	delegate                    string
	endorsementCurrentBlockChan chan *rpc.Block
	bakingCurrentBlockChan      chan *rpc.Block
	currentCurrentBlockChan     chan *rpc.Block
}

func New(delegate string, r rpc.IFace, notifier twilio.IFace) *Watcher {
	return &Watcher{
		r:                           r,
		notifier:                    notifier,
		delegate:                    delegate,
		endorsementCurrentBlockChan: make(chan *rpc.Block, 5),
		bakingCurrentBlockChan:      make(chan *rpc.Block, 5),
		currentCurrentBlockChan:     make(chan *rpc.Block, 5),
	}
}

func (w *Watcher) startStateUpdater() {
	ticker := time.NewTicker(time.Second * 30)
	currentBlock := 0
	currentCycle := 0
	go func() {
		for range ticker.C {
			head, err := w.r.Head()
			if err != nil {
				logrus.WithField("error", err.Error()).Error("Failed to get current block height.")
			}

			if head.Metadata.Level.Cycle > currentCycle {
				logrus.WithFields(logrus.Fields{"cycle": head.Metadata.Level.Cycle, "level": head.Header.Level, "hash": head.Hash}).Info("Updated to new cycle.")
				currentCycle = head.Metadata.Level.Cycle
				w.currentCurrentBlockChan <- head
			}

			if head.Header.Level > currentBlock {
				logrus.WithFields(logrus.Fields{"level": head.Header.Level, "hash": head.Hash}).Info("Updated head block.")
				currentBlock = head.Header.Level

				w.endorsementCurrentBlockChan <- head
				w.bakingCurrentBlockChan <- head
			}
		}
	}()
}

func (w *Watcher) Start() {
	logrus.Info("Starting baking monitor.")
	w.startStateUpdater()
	go func() {
		var doneBr, doneEr chan struct{}
		for block := range w.currentCurrentBlockChan {
			logrus.WithField("cycle", block.Metadata.Level.Cycle).Info("Updating current cycle rights.")
			if doneBr != nil && doneEr != nil {
				doneEr <- struct{}{}
				doneBr <- struct{}{}
			}

			br, err := w.r.BakingRights(rpc.BakingRightsInput{
				BlockHash:   block.Hash,
				Cycle:       block.Metadata.Level.Cycle,
				Delegate:    w.delegate,
				MaxPriority: 0,
			})
			if err != nil {
				logrus.WithFields(logrus.Fields{"error": err.Error(), "cycle": block.Metadata.Level.Cycle}).Error("failed to get baking rights")
				break
			}
			logrus.WithFields(logrus.Fields{"rights": fmt.Sprintf("%+v", br), "cycle": block.Metadata.Level.Cycle}).Info("Received baking rights for cycle.")
			doneBr = make(chan struct{})
			w.watchBakingRights(br, doneBr)

			er, err := w.r.EndorsingRights(rpc.EndorsingRightsInput{
				BlockHash: block.Hash,
				Cycle:     block.Metadata.Level.Cycle,
				Delegate:  w.delegate,
			})
			if err != nil {
				logrus.WithFields(logrus.Fields{"error": err.Error(), "cycle": block.Metadata.Level.Cycle}).Error("failed to get endorsing rights")
				break
			}
			logrus.WithFields(logrus.Fields{"rights": fmt.Sprintf("%+v", er), "cycle": block.Metadata.Level.Cycle}).Info("Received endorsement rights for cycle.")
			doneEr = make(chan struct{})
			w.watchEndorsements(er, doneEr)
		}
	}()
}

func (w *Watcher) watchBakingRights(br *rpc.BakingRights, done chan struct{}) {
	uuid := uuid.New()
	logrus.WithField("uuid", uuid.String()).Info("Starting new baking rights worker.")
	go func() {
		for {
			select {
			case block := <-w.bakingCurrentBlockChan:
				for _, r := range *br {
					if r.Level == block.Header.Level && r.Priority == 0 {
						logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "block": block.Header.Level}).Info("Baking rights worker found baking slot for priority 0.")
						if block.Metadata.Baker != w.delegate {
							logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "block": block.Header.Level}).Error("Missed Baking Opportunity.")
							w.notifier.Send(fmt.Sprintf("Missed Baking Opportunity at level '%d'.", block.Header.Level))
						} else {
							logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "block": block.Header.Level}).Info("Successfully Baked Block.")
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

	go func() {
		for {
			select {
			case block := <-w.endorsementCurrentBlockChan:
				for _, r := range *er {
					if r.Level == block.Header.Level-1 {
						logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "right-level": r.Level, "block": block.Header.Level}).Info("Endorsing rights worker found endorsing slot.")
						var found bool
						for _, operations := range block.Operations {
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
							logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "right-level": r.Level, "block": block.Header.Level}).Error("Missed Endorsing Opportunity.")
							w.notifier.Send(fmt.Sprintf("Missed Endorsement Opportunity at level '%d'", r.Level))
						} else {
							logrus.WithFields(logrus.Fields{"uuid": uuid.String(), "right-level": r.Level, "block": block.Header.Level}).Info("Successfully Endorsed Block.")
						}
					}
				}
			case <-done:
				logrus.WithField("uuid", uuid.String()).Info("Ending endorsing rights worker.")
				break
			}
		}
	}()
}
