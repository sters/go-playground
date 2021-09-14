package main

import (
	"context"
	"errors"
	"net"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/mercari/go-circuitbreaker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
)

type server struct {
	pb.UnimplementedGreeterServer

	duringError time.Time
}

var _ pb.GreeterServer = (*server)(nil)

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	if s.duringError.Sub(time.Now()) > 0 {
		return nil, errors.New("fail during time")
	}

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
			// grpc_zap.UnaryServerInterceptor(logger),
			),
		)
		defer s.Stop()

		pb.RegisterGreeterServer(s, &server{
			duringError: time.Now().Add(10 * time.Second),
		})

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
			grpc.WithUnaryInterceptor(
				grpc_middleware.ChainUnaryClient(
					grpc_zap.UnaryClientInterceptor(logger),
					circuitBreakerInterceptor(),
				),
			),
		)
		defer func() {
			_ = client.Close()
		}()

		if err != nil {
			logger.Error("failed create client", zap.Error(err))
		}

		greeterClient := pb.NewGreeterClient(client)

		timer := time.NewTicker(time.Second)
		for {
			select {
			case <-timer.C:
				logger.Info("Do request")
				response, err := greeterClient.SayHello(ctx, &pb.HelloRequest{Name: "sters"})
				logger.Info("", zap.Any("response", response), zap.Error(err))
			default:
			}
		}
	}()

	time.Sleep(30 * time.Second)
	zap.L().Info("Shutdown")
}

func circuitBreakerInterceptor() grpc.UnaryClientInterceptor {
	cb := circuitbreaker.New(&circuitbreaker.Options{
		// reset state during this interval
		Interval: time.Minute,

		// if N times fails, state will be open.
		ShouldTrip: circuitbreaker.NewTripFuncThreshold(1),

		// If state is open.
		// After this time, state will be changed to half-open.
		// if keep failing, back to open.
		OpenTimeout: 5 * time.Second,

		// If state is half-open.
		// if success this times, state will be back to close.
		HalfOpenMaxSuccesses: 3,
	})

	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		_, err := cb.Do(ctx, func() (interface{}, error) {
			zap.L().Info("cb", zap.Any("cb.state", cb.State()))
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err != nil {
				return nil, err
			}

			return nil, nil
		})

		return err
	}
}
