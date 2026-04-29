package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/yourorg/codewalker/internal/forge"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestForgeErrToStatus(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantCode     codes.Code
		wantContains []string
	}{
		{
			name:         "auth failed without detail",
			err:          &forge.Error{Code: forge.ErrCodeAuthFailed, Msg: "GitHub API auth failed (401 Unauthorized)"},
			wantCode:     codes.PermissionDenied,
			wantContains: []string{"GitHub API auth failed"},
		},
		{
			name: "403 with SAML SSO body",
			err: &forge.Error{
				Code:   forge.ErrCodeAuthFailed,
				Msg:    "GitHub API auth failed (403 Forbidden)",
				Detail: `{"message":"Resource protected by organization SAML enforcement"}`,
			},
			wantCode:     codes.PermissionDenied,
			wantContains: []string{"GitHub API auth failed", "Resource protected by organization SAML enforcement"},
		},
		{
			name:         "not found",
			err:          &forge.Error{Code: forge.ErrCodeNotFound, Msg: "GitHub API not found (404 Not Found)"},
			wantCode:     codes.NotFound,
			wantContains: []string{"not found"},
		},
		{
			name:         "unknown error",
			err:          errors.New("network broke"),
			wantCode:     codes.Internal,
			wantContains: []string{"network broke"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st, ok := status.FromError(forgeErrToStatus(tc.err, "fetch diff"))
			if !ok {
				t.Fatalf("not a gRPC status: %v", tc.err)
			}
			if st.Code() != tc.wantCode {
				t.Errorf("code = %v, want %v", st.Code(), tc.wantCode)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(st.Message(), want) {
					t.Errorf("status message missing %q: got %q", want, st.Message())
				}
			}
		})
	}
}
