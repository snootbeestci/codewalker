package forge

// ErrCode classifies a forge error so callers can map it to a protocol-level error code.
type ErrCode int

const (
	ErrCodeUnknown    ErrCode = 0
	ErrCodeAuthFailed ErrCode = 1
	ErrCodeNotFound   ErrCode = 2
)

// Error is a typed forge error that carries a machine-readable code.
type Error struct {
	Code ErrCode
	Msg  string
}

func (e *Error) Error() string { return e.Msg }
