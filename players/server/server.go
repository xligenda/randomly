package server

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"players/mc"
	pb "players/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultTimeout = 5 * time.Second
	cacheDuration  = 3 * time.Second
)

type PlayerServer struct {
	pb.UnimplementedPlayerServiceServer
	Timeout time.Duration

	mu           sync.RWMutex
	lastStatus   *mc.StatusResponse
	lastPingedAt time.Time
}

func NewPlayerServer() *PlayerServer {
	return &PlayerServer{Timeout: defaultTimeout}
}

func (s *PlayerServer) GetRandomPlayer(ctx context.Context, req *pb.ServerAdress) (*pb.ServerPlayerResponse, error) {
	address := req.GetAddress()
	if address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required, format host:port")
	}

	statusResp, err := s.getStatusWithCache(address)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get server status: %v", err)
	}

	sample := statusResp.Players.Sample
	if len(sample) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "server returned no player sample: either nobody is online or the server hides the sample")
	}

	player := sample[rand.Intn(len(sample))]
	return &pb.ServerPlayerResponse{
		Username:    player.Name,
		Id:          player.ID,
		OnlineCount: int32(statusResp.Players.Online),
		MaxCount:    int32(statusResp.Players.Max),
	}, nil
}

func (s *PlayerServer) GetPlayer(ctx context.Context, req *pb.PlayerRequest) (*pb.Player, error) {
	if req.GetIdentifier() == nil {
		return nil, status.Error(codes.InvalidArgument, "identifier field is required")
	}

	var identifier string
	switch id := req.GetIdentifier().(type) {
	case *pb.PlayerRequest_Uuid:
		identifier = id.Uuid
	case *pb.PlayerRequest_Username:
		identifier = id.Username
	default:
		return nil, status.Error(codes.Unimplemented, "unknown identifier type")
	}

	profile, err := mc.FetchProfile(identifier, s.Timeout)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to fetch profile: %v", err)
	}

	return &pb.Player{
		Id:       profile.ID,
		Username: profile.Name,
	}, nil
}

func (s *PlayerServer) ServerOnline(
	ctx context.Context,
	req *pb.ServerAdress,
) (*pb.ServerOnlineResponse, error) {
	address := req.GetAddress()
	if address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required, format host:port")
	}

	statusResp, err := s.getStatusWithCache(address)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get server status: %v", err)
	}

	return &pb.ServerOnlineResponse{
		OnlineCount: int32(statusResp.Players.Online),
	}, nil
}
