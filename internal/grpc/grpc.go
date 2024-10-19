package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"

	"github.com/lhjnilsson/foreverbull/internal/pb"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type HealthCheck struct {
	pb.UnimplementedHealthServer
}

func (h *HealthCheck) Check(ctx context.Context, req *emptypb.Empty) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{Status: pb.HealthCheckResponse_SERVING}, nil
}

const (
	FieldLength = 2
)

func InterceptorLogger(l *zap.Logger) logging.Logger {
	parseMessage := func(msg string, fields ...any) []zap.Field {
		f := make([]zap.Field, 0, len(fields)/FieldLength)

		for i := 0; i < len(fields); i += FieldLength {
			key := fields[i]
			value := fields[i+1]

			switch v := value.(type) {
			case string:
				f = append(f, zap.String(key.(string), v))
			case int:
				f = append(f, zap.Int(key.(string), v))
			case bool:
				f = append(f, zap.Bool(key.(string), v))
			default:
				f = append(f, zap.Any(key.(string), v))
			}
		}

		return f
	}

	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		f := parseMessage(msg, fields...)
		logger := l.WithOptions(zap.AddCallerSkip(1)).With(f...)

		switch lvl {
		case logging.LevelDebug:
			logger.Debug(msg)
		case logging.LevelInfo:
			logger.Info(msg)
		case logging.LevelWarn:
			logger.Warn(msg)
		case logging.LevelError:
			logger.Error(msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}

var Module = fx.Options(
	fx.Provide(
		func() *grpc.Server {
			logger := zap.NewExample()
			opts := []logging.Option{
				logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
			}
			return grpc.NewServer(
				grpc.ChainUnaryInterceptor(
					logging.UnaryServerInterceptor(InterceptorLogger(logger), opts...),
				),
				grpc.ChainStreamInterceptor(
					logging.StreamServerInterceptor(InterceptorLogger(logger), opts...),
				),
			)
		},
	),
	fx.Invoke(
		func(lc fx.Lifecycle, g *grpc.Server) error {
			lc.Append(
				fx.Hook{
					OnStart: func(context.Context) error {
						listener, err := net.Listen("tcp", ":50055") //nolint: gosec
						if err != nil {
							return fmt.Errorf("failed to listen: %w", err)
						}
						server := &HealthCheck{}
						pb.RegisterHealthServer(g, server)
						go func() {
							if err := g.Serve(listener); err != nil {
								panic(err)
							}
						}()
						return nil
					},
					OnStop: func(context.Context) error {
						g.GracefulStop()
						return nil
					},
				},
			)
			return nil
		},
	),
)
