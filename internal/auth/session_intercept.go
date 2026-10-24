package auth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// SessionInterceptors require x-foreverbull-session on every call to a listener.
// They do not read the user bearer.
func SessionInterceptors(secret string) []grpc.ServerOption {
	check := func(ctx context.Context) error {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Error(codes.Unauthenticated, "unauthorized")
		}
		values := md.Get(sessionMetadata)
		if len(values) != 1 || !SecretMatches(values[0], secret) {
			return status.Error(codes.Unauthenticated, "unauthorized")
		}
		return nil
	}
	unary := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := check(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
	stream := func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := check(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unary),
		grpc.ChainStreamInterceptor(stream),
	}
}
