package main

import (
	"context"
	"log"
	"net"
	"unicode/utf8"

	demopb "jl560-order-demo/gen/demo"

	"google.golang.org/grpc"
)

type demoServer struct {
	demopb.UnimplementedDemoServer
}

func (demoServer) InspectUsername(ctx context.Context, req *demopb.InspectUsernameRequest) (*demopb.InspectUsernameReply, error) {
	n := utf8.RuneCountInString(req.GetUsername())
	return &demopb.InspectUsernameReply{
		Length:     int32(n),
		Acceptable: n >= 3 && n <= 32,
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	demopb.RegisterDemoServer(server, demoServer{})
	log.Println("gRPC 服务监听 :50051")
	if err := server.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
