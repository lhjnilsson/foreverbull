package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/bufbuild/protovalidate-go"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"

	"github.com/lhjnilsson/foreverbull/internal/pb"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type HealthCheck struct {
	pb.UnimplementedHealthServer
}

func (h *HealthCheck) Check(_ context.Context, _ *emptypb.Empty) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{Status: pb.HealthCheckResponse_SERVING}, nil
}

const (
	FieldLength = 2
)

func InterceptorLogger(logger *zap.Logger) logging.Logger {
	parseMessage := func(msg string, fields ...any) []zap.Field {
		zFields := make([]zap.Field, 0, len(fields)/FieldLength)

		for i := 0; i < len(fields); i += FieldLength {
			key := fields[i]
			value := fields[i+1]

			switch value := value.(type) {
			case string:
				zFields = append(zFields, zap.String(key.(string), value)) //nolint: forcetypeassert
			case int:
				zFields = append(zFields, zap.Int(key.(string), value)) //nolint: forcetypeassert
			case bool:
				zFields = append(zFields, zap.Bool(key.(string), value)) //nolint: forcetypeassert
			default:
				zFields = append(zFields, zap.Any(key.(string), value)) //nolint: forcetypeassert
			}
		}

		return zFields
	}

	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		f := parseMessage(msg, fields...)
		logger := logger.WithOptions(zap.AddCallerSkip(1)).With(f...)

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

func NewServer() (*grpc.Server, error) {
	logger := zap.NewExample()

	allButHealthZ := func(ctx context.Context, callMeta interceptors.CallMeta) bool {
		return pb.Health_ServiceDesc.ServiceName != callMeta.Service
	}

	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	opts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
	}
	selector.MatchFunc(allButHealthZ)

	return grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			selector.UnaryServerInterceptor(
				logging.UnaryServerInterceptor(InterceptorLogger(logger), opts...),
				selector.MatchFunc(allButHealthZ),
			),
			protovalidate_middleware.UnaryServerInterceptor(validator),
		),
		grpc.ChainStreamInterceptor(
			selector.StreamServerInterceptor(
				logging.StreamServerInterceptor(InterceptorLogger(logger), opts...),
				selector.MatchFunc(allButHealthZ),
			),
			protovalidate_middleware.StreamServerInterceptor(validator),
		),
	), nil
}

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		NewServer(),
	),
	fx.Invoke(
		func(lc fx.Lifecycle, grpcServer *grpc.Server) error {
			lc.Append(
				fx.Hook{
					OnStart: func(context.Context) error {
						listener, err := net.Listen("tcp", ":50055") //nolint: gosec
						if err != nil {
							return fmt.Errorf("failed to listen: %w", err)
						}
						server := &HealthCheck{}
						pb.RegisterHealthServer(grpcServer, server)
						go func() {
							if err := grpcServer.Serve(listener); err != nil {
								panic(err)
							}
						}()
						return nil
					},
					OnStop: func(context.Context) error {
						grpcServer.GracefulStop()
						return nil
					},
				},
			)
			return nil
		},
	),
)
