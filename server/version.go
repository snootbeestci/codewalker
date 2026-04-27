package server

import (
	"context"
	"strconv"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
)

// Version and ProtoMajor are set at build time via -ldflags.
// Defaults are used when running outside of a release build.
// ProtoMajor must be a string because -X ldflags can only inject string vars.
var (
	Version    = "dev"
	ProtoMajor = "1"
)

func (s *Server) GetVersion(_ context.Context, _ *v1.GetVersionRequest) (*v1.GetVersionResponse, error) {
	major, _ := strconv.ParseUint(ProtoMajor, 10, 32)
	return &v1.GetVersionResponse{
		Version:                 Version,
		ProtoMajor:              uint32(major),
		MinCompatibleProtoMajor: 1,
	}, nil
}
