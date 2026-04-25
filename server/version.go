package server

import (
	"context"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
)

// Version and ProtoMajor are set at build time via -ldflags.
// Defaults are used when running outside of a release build.
var (
	Version    = "dev"
	ProtoMajor = uint32(1)
)

func (s *Server) GetVersion(_ context.Context, _ *v1.GetVersionRequest) (*v1.GetVersionResponse, error) {
	return &v1.GetVersionResponse{
		Version:                 Version,
		ProtoMajor:              ProtoMajor,
		MinCompatibleProtoMajor: 1,
	}, nil
}
