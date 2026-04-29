package forge

// ErrCode classifies a forge error so callers can map it to a protocol-level error code.
type ErrCode int

const (
	ErrCodeUnknown    ErrCode = 0
	ErrCodeAuthFailed ErrCode = 1
	ErrCodeNotFound   ErrCode = 2
)

// Error is a typed forge error that carries a machine-readable code.
//
// Detail optionally carries the forge's raw response body. It is used for 403
// responses so clients can distinguish a bad token from an organization-level
// SSO authorization requirement (e.g. GitHub Enterprise SAML SSO returns 403
// with a body explaining the missing authorization). Truncated to ~500 chars
// at the source.
type Error struct {
	Code   ErrCode
	Msg    string
	Detail string
}

func (e *Error) Error() string { return e.Msg }
