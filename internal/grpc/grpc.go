package grpc

import (
	"context"
	"errors"
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

	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
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

func pgxErrorToStatus(err *pgconn.PgError) error {
	switch err.Code {
	case "23505":
		return status.Error(codes.AlreadyExists, "Resource already exists")
	case "23503":
		return status.Error(codes.FailedPrecondition, "Referenced resource not found")
	case "23502":
		return status.Error(codes.InvalidArgument, "Required field missing")
	case "22P02":
		return status.Error(codes.InvalidArgument, "Invalid input format")
	case "42P01":
		return status.Error(codes.NotFound, "Resource not found")
	case "42703":
		return status.Error(codes.Internal, "Invalid field reference")
	case "53300":
		return status.Error(codes.ResourceExhausted, "Database connection limit reached")
	case "57014":
		return status.Error(codes.DeadlineExceeded, "Query timeout")
	default:
		return status.Error(codes.Internal, "Internal database error")
	}
}

func pgxErrorInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		// Check if it's a PostgreSQL error
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgxErrorToStatus(pgErr)
		}
		return resp, err
	}
	return resp, nil
}

func pgxStreamErrorInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := handler(srv, ss)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgxErrorToStatus(pgErr)
		}
		return err
	}
	return nil
}

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

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			selector.UnaryServerInterceptor(
				logging.UnaryServerInterceptor(InterceptorLogger(logger), opts...),
				selector.MatchFunc(allButHealthZ),
			),
			protovalidate_middleware.UnaryServerInterceptor(validator),
			pgxErrorInterceptor,
		),
		grpc.ChainStreamInterceptor(
			selector.StreamServerInterceptor(
				logging.StreamServerInterceptor(InterceptorLogger(logger), opts...),
				selector.MatchFunc(allButHealthZ),
			),
			protovalidate_middleware.StreamServerInterceptor(validator),
			pgxStreamErrorInterceptor,
		),
	)
	reflection.Register(server)
	return server, nil
}

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func() (*grpc.Server, error) {
			return NewServer()
		},
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
