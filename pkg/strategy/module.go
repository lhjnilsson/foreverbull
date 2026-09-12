package strategy

import "go.uber.org/fx"

// Module is quarantined. RunStrategy is not a real run and must not be
// registered on the gRPC server.
var Module = fx.Options() //nolint: gochecknoglobals
