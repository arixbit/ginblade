package errcode

var (
	// InvalidParams means the request payload or query string failed validation.
	InvalidParams = newError(1001, "INVALID_PARAMS")
	// Unauthorized means authentication is required or invalid.
	Unauthorized = newError(1002, "UNAUTHORIZED")
	// PermissionDenied means the caller is authenticated but not allowed.
	PermissionDenied = newError(1003, "PERMISSION_DENIED")
	// TooManyRequests means the request was blocked by rate limiting.
	TooManyRequests = newError(1004, "TOO_MANY_REQUESTS")
	// RequestTimeout means request processing exceeded the configured deadline.
	RequestTimeout = newError(1005, "REQUEST_TIMEOUT")

	// InternalError is the fallback for unexpected server-side errors.
	InternalError = newError(9001, "INTERNAL_ERROR")
	// DatabaseError wraps persistence failures exposed by services.
	DatabaseError = newError(9002, "DATABASE_ERROR")
	// QueueUnavailable means async task publishing was requested but not configured.
	QueueUnavailable = newError(9003, "QUEUE_UNAVAILABLE")
	// QueueError wraps async task publishing failures exposed by services.
	QueueError = newError(9004, "QUEUE_ERROR")

	// NotFound is used when a requested resource does not exist.
	NotFound = newError(2001, "NOT_FOUND")
	// InsufficientBalance is used when a transfer would overdraw the source wallet.
	InsufficientBalance = newError(2002, "INSUFFICIENT_BALANCE")
)
