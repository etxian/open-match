package backendapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"google.golang.org/grpc"

	"open-match.dev/open-match/internal/pb"
)

// Connect to in-cluster Open-match BackendAPI service
func Connect() (*grpc.ClientConn, pb.BackendClient, error) {
	addrs, err := net.LookupHost("om-backendapi.open-match")
	if err != nil {
		return nil, nil, errors.New("error creating Backend API client: lookup failed: " + err.Error())
	}

	addr := fmt.Sprintf("%s:50505", addrs[0])

	beAPIConn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, nil, errors.New("error creating Backend API client: failed to connect: " + err.Error())
	}
	beAPI := pb.NewBackendClient(beAPIConn)
	return beAPIConn, beAPI, nil
}

// MatchFunc is a function that is applied to each item of ListMatches() stream.
// Iteration is stopped and stream is closed if function return false or error.
type MatchFunc func(*pb.MatchObject) (bool, error)

func ListMatches(ctx context.Context, profile *pb.ListMatchesRequest, fn MatchFunc) error {
	beAPIConn, beAPI, err := Connect()
	if err != nil {
		return err
	}
	defer beAPIConn.Close()

	stream, err := beAPI.ListMatches(ctx, profile)
	if err != nil {
		return errors.New("error opening matches stream: " + err.Error())
	}

	for {
		match, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			stream.CloseSend()
			return errors.New("error receiving match: " + err.Error())
		}

		var ok bool
		if ok, err = fn(match.Match); err != nil {
			stream.CloseSend()
			return errors.New("error processing match: " + err.Error())
		}
		if !ok {
			stream.CloseSend()
			return nil
		}
	}
}

func CreateMatch(ctx context.Context, request *pb.CreateMatchRequest) (*pb.MatchObject, error) {
	beAPIConn, beAPI, err := Connect()
	if err != nil {
		return nil, err
	}
	defer beAPIConn.Close()

	match, err := beAPI.CreateMatch(ctx, request)
	if err != nil {
		return nil, errors.New("error creating match: " + err.Error())
	}

	return match.Match, nil
}

func DeleteMatch(ctx context.Context, request *pb.DeleteMatchRequest) error {
	beAPIConn, beAPI, err := Connect()
	if err != nil {
		return err
	}
	defer beAPIConn.Close()

	_, err = beAPI.DeleteMatch(ctx, request)
	if err != nil {
		return errors.New("error deleting match: " + err.Error())
	}
	return nil
}

func CreateAssginments(ctx context.Context, request *pb.CreateAssignmentsRequest) error {
	beAPIConn, beAPI, err := Connect()
	if err != nil {
		return err
	}
	defer beAPIConn.Close()

	_, err = beAPI.CreateAssignments(ctx, request)
	if err != nil {
		return errors.New("error creating assignments: " + err.Error())
	}
	return nil
}