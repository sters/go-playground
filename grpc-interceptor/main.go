package main

import (
	"context"
	"net"
	"strings"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type server struct {
	pb.UnimplementedGreeterServer
}

var _ pb.GreeterServer = (*server)(nil)

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	ctxzap.Extract(ctx).Info("Received", zap.Any("in.name", in.Name))
	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func main() {
	port := "127.0.0.1:13000"

	l, _ := zap.NewDevelopment()
	zap.ReplaceGlobals(l)

	// server
	go func() {
		logger := zap.L().Named("Server")

		listener, err := net.Listen("tcp", port)
		if err != nil {
			logger.Error("failed to listen: %v", zap.Error(err))
		}
		defer listener.Close()

		s := grpc.NewServer(
			grpc_middleware.WithUnaryServerChain(
				grpc_zap.UnaryServerInterceptor(logger),
				accessLogUnaryServerInterceptor(),
			),
		)
		defer s.Stop()

		pb.RegisterGreeterServer(s, &server{})

		if err := s.Serve(listener); err != nil {
			logger.Error("failed to serve: %v", zap.Error(err))
		}
	}()

	// client
	go func() {
		time.Sleep(2 * time.Second)
		ctx := context.Background()
		logger := zap.L().Named("Client")

		client, err := grpc.DialContext(
			ctx,
			port,
			grpc.WithUserAgent("my-user-agent/1.0.0"),
			grpc.WithInsecure(),
		)
		defer func() {
			_ = client.Close()
		}()

		if err != nil {
			logger.Error("failed create client", zap.Error(err))
		}

		greeterClient := pb.NewGreeterClient(client)

		logger.Info("Do request")

		response, err := greeterClient.SayHello(ctx, &pb.HelloRequest{Name: "sters"})

		logger.Info("", zap.Any("response.Message", response.Message), zap.Error(err))
	}()

	time.Sleep(5 * time.Second)
	zap.L().Info("Shutdown")
}

func accessLogUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		clientIP := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			clientIP = p.Addr.String()
		}

		useragent := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if ua, ok := md["user-agent"]; ok {
				useragent = strings.Join(ua, ",")
			}
		}

		ctxzap.AddFields(
			ctx,
			zap.String("access.clientip", clientIP),
			zap.String("access.useragent", useragent),
		)

		return handler(ctx, req)
	}
}
