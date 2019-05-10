package main

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"

	"open-match.dev/open-match/examples/director/pkg/backendapi"
	"open-match.dev/open-match/internal/pb"
)

func startSendProfile(ctx context.Context, profile *pb.MatchObject, mmfcfg *pb.MmfConfig, l *log.Entry) {
	for i := 0; i < maxSends || maxSends <= 0; i++ {
		sendLog := l.WithFields(log.Fields{
			"profile": profile.Id,
			"#send":   i,
		})

		sendLog.Debugf("Sending profile \"%s\" (attempt #%d/%d)...", profile.Id, i+1, maxSends)
		sendProfile(ctx, profile, mmfcfg, sendLog)

		if i < maxSends - 1 || maxSends <= 0 {
			sendLog.Debugf("Sleeping \"%s\"...", profile.Id)
			time.Sleep(sleepBetweenSends)
		}
	}
}

func sendProfile(ctx context.Context, profile *pb.MatchObject, mmfcfg *pb.MmfConfig, l *log.Entry) {
	var j int

	request := &pb.ListMatchesRequest{Match: profile, Mmfcfg: mmfcfg}
	err := backendapi.ListMatches(ctx, request, func(match *pb.MatchObject) (bool, error) {
		matchLog := l.WithField("#recv", j)

		proceed, err := handleProfileMatch(ctx, match, mmfcfg, matchLog)
		if err != nil {
			return false, err
		}

		if j++; j >= maxMatchesPerSend && maxMatchesPerSend > 0 {
			matchLog.Debug("Reached max num of match receive attempts, closing stream")
			return false, nil
		}

		return proceed, nil
	})

	if err != nil {
		l.WithError(err).Error(err)
	}
}

func handleProfileMatch(ctx context.Context, match *pb.MatchObject, mmfcfg *pb.MmfConfig, l *log.Entry) (bool, error) {
	matchLog := l.WithField("match", match.Id)

	if match.Error != "" {
		matchLog.WithField(log.ErrorKey, match.Error).Error("Received a match with non-empty error, skip this match")
		return true, nil
	}
	if !gjson.Valid(string(match.Properties)) {
		matchLog.Error("Invalid properties json, skip this match")
		return true, nil
	}

	// Run DGS allocator process
	allocChan := make(chan string, 1)
	go func() {
		defer close(allocChan)

		connstring, err := tryAllocate(ctx, match, matchLog)
		if err != nil {
			// TODO delete match, prevent complementary matches

			// Just log error for now
			matchLog.WithError(err).Errorf("Could not allocate a GS for a match %s", match.Id)
		} else {
			allocChan <- connstring
		}
	}()

	tasks := make(chan *pb.MatchObject, 1)
	tasks <- match

	// Run players assigner process
	go func() {
		connstring, ok := <-allocChan
		if !ok {
			return // TODO handle close of channel (meaning allocation failed)
		}

		var first *pb.MatchObject

		for {
			select {
			case <-ctx.Done():
				return

			case task, ok := <-tasks:
				if !ok {
					// TODO handle close of channel
					// May be ok: when partial match got complemented, and no more new matches expected to appear.
					// But also it could be that some error happened.
					return
				}

				players := collectPlayerIds(task.Rosters)
				matchLog.
					WithField("taskId", task.Id).
					WithField("players", players).
					WithField("assignment", connstring).
					Infof("Assigning %d new players for match %s to DGS at %s", len(players), match.Id, connstring)

				// Distribute connection string to players
				request := &pb.CreateAssignmentsRequest{Assignment: &pb.Assignments{Rosters: task.Rosters, Assignment: connstring}}
				err := backendapi.CreateAssginments(ctx, request)
				if err != nil {
					// ???
					// PLAN:
					// - If failed for the first match then delete match & unallocate DGS
					// - If failed for futher match then ignore error? Or just delete that match

					// Just log for now
					matchLog.
						WithField("taskId", task.Id).
						WithField("players", players).
						WithField("assignment", connstring).
						WithError(err).
						Errorf("Error assigning %d new players for match %s to DGS at %s", len(players), match.Id, connstring)
				}
				matchLog.WithField("assignment", connstring).WithField("players", players).Info("Create assignment")

				// Propagate newly matched rosters to the allocated DGS
				err = allocator.Send(connstring, task)
				if err != nil {
					matchLog.
						WithField("taskId", task.Id).
						WithField("players", players).
						WithField("assginment", connstring).
						WithError(err).
						Errorf("Error propagating the details of match %s to DGS at %s", match.Id, connstring)
				}

				if first == nil {
					first = task
				}
			}
		}
	}()

	return true, nil
}

func tryAllocate(ctx context.Context, match *pb.MatchObject, l *log.Entry) (string, error) {
	// Try to allocate a DGS; retry with exponential backoff and jitter
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 2 * time.Second
	b.MaxElapsedTime = 2 * time.Minute
	bt := backoff.NewTicker(backoff.WithContext(b, ctx))

	var connstring string
	for range bt.C {
		connstring, err = allocator.Allocate(match)
		if err != nil {
			l.WithError(err).Error("Allocation attempt failed")
			continue
		}
		bt.Stop()
		break
	}
	if err != nil {
		return "", err
	}
	return connstring, nil
}
