package main

import (
	"log"
	"net"

	"google.golang.org/grpc"

	"players/pb"
	"players/server"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("unable to open port: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPlayerServiceServer(grpcServer, server.NewPlayerServer())

	log.Println("gRPC server listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
