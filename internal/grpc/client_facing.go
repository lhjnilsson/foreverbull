package grpc

import (
	"context"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// urlPassword matches the password in scheme://user:pw@ URLs so it never
// reaches a gRPC client. Same shape as the ingest redactor.
var urlPassword = regexp.MustCompile(`(\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]*:)[^\s@]*@`)

// secretNeedle means the message still names a credential after URL
// password redaction: password=…, POSTGRES_URL=…, or a URL userinfo that
// is not already ***. A host-only postgres://…:***@… line is allowed.
var secretNeedle = regexp.MustCompile(`(?i)((?:password|passwd|pwd)\s*=|(?:POSTGRES_URL|DATABASE_URL)\s*=|secret[_-]?key\s*=|access[_-]?key\s*=|://[^:@\s]+:[^@*\s]+@)`)

const (
	// clientInternal is the only message for an unmapped fault.
	// Handlers that return a plain error (fmt.Errorf wrapping a DB or
	// dial failure) land here so the client never sees the cause.
	clientInternal = "Something went wrong. Try again, or run `fbenv status` if things look unhealthy."
	// clientInternalDatabase aliases clientInternal for the pgx default path
	// and for status messages that still look like they name a credential.
	clientInternalDatabase = clientInternal
)

func redactURLPasswords(text string) string {
	return urlPassword.ReplaceAllString(text, "${1}***@")
}

func looksLikeSecret(text string) bool {
	return secretNeedle.MatchString(text)
}

// clientFacingError turns a handler error into a status the client may see.
//
// Already-status errors keep their code. The message is redacted for URL
// passwords; if it still looks like a credential or connection string, it is
// replaced with a fixed internal line. Plain errors become Internal with
// clientInternal and never echo err.Error().
func clientFacingError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return status.Error(codes.Internal, clientInternal)
	}
	msg := redactURLPasswords(st.Message())
	if looksLikeSecret(msg) {
		switch st.Code() {
		case codes.NotFound:
			// Keep NotFound so "unknown backtest" clients still branch, but
			// drop any secret-shaped text from a buggy handler.
			return status.Error(codes.NotFound, "not found")
		case codes.InvalidArgument, codes.FailedPrecondition, codes.AlreadyExists,
			codes.PermissionDenied, codes.Unauthenticated, codes.ResourceExhausted,
			codes.Unavailable, codes.DeadlineExceeded:
			return status.Error(st.Code(), clientInternal)
		default:
			return status.Error(codes.Internal, clientInternalDatabase)
		}
	}
	if msg == st.Message() {
		return err
	}
	return status.Error(st.Code(), msg)
}

func clientFacingUnary(
	ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (interface{}, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		return resp, clientFacingError(err)
	}
	return resp, nil
}

func clientFacingStream(
	srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler,
) error {
	err := handler(srv, ss)
	if err != nil {
		return clientFacingError(err)
	}
	return nil
}
