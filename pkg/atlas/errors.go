package atlas

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/r2northstar/atlas/v2/pkg/jsonx"
)

type ErrorCode string

const (
	ErrorCodeAuthMissing = "auth_missing"
	ErrorCodeAuthInvalid = "auth_invalid"

	ErrorCodeAuthPlayerMissing   = "auth_player_missing"
	ErrorCodeAuthPlayerExpired   = "auth_player_expired"
	ErrorCodeAuthPlayerDestroyed = "auth_player_destroyed"

	ErrorCodeAuthServerMissing   = "auth_server_missing"
	ErrorCodeAuthServerExpired   = "auth_server_expired"
	ErrorCodeAuthServerDestroyed = "auth_server_destroyed"

	ErrorCodePdataLocked = "pdata_locked"

	ErrorCodeServerNotFound = "server_not_found"

	ErrorCodeBackendServiceUnavailable = "backend_service_unavailable"

	ErrorCodeBadRequest        = "bad_request"
	ErrorCodeInternalError     = "internal_error"
	ErrorCodeGenericError      = "generic_error"
	ErrorCodeGenericErrorFatal = "generic_error_fatal"
)

func (ec ErrorCode) Code() string {
	return string(ec)
}

func (ec ErrorCode) String() string {
	switch ec {
	case ErrorCodeAuthMissing:
		return "no session token"
	case ErrorCodeAuthInvalid:
		return "invalid session token"
	case ErrorCodeAuthPlayerMissing:
		return "current session does not have an authenticated player"
	case ErrorCodeAuthPlayerExpired:
		return "current session player authentication expired"
	case ErrorCodeAuthPlayerDestroyed:
		return "current session player authentication destroyed"
	case ErrorCodeAuthServerMissing:
		return "current session does not have a verified server"
	case ErrorCodeAuthServerExpired:
		return "current session server verification expired"
	case ErrorCodeAuthServerDestroyed:
		return "current session server verification destroyed"
	case ErrorCodePdataLocked:
		return "pdata is locked"
	case ErrorCodeServerNotFound:
		return "server not found"
	case ErrorCodeBackendServiceUnavailable:
		return "backend service unavailable"
	case ErrorCodeBadRequest:
		return "bad request"
	case ErrorCodeInternalError:
		return "internal server error"
	case ErrorCodeGenericError:
		return "unknown error"
	case ErrorCodeGenericErrorFatal:
		return "unknown fatal error"
	default:
		return "unknown error"
	}
}

func (ec ErrorCode) Explanation() string {
	switch ec {
	case ErrorCodeAuthMissing:
		return "the client must provide a session token for this kind of request"
	case ErrorCodeAuthInvalid:
		return "the client must be restarted since the provided session token does not exist or is no longer valid"
	case ErrorCodeAuthPlayerMissing:
		return "the client must authenticate the player for this kind of request"
	case ErrorCodeAuthPlayerExpired:
		return "the client must re-authenticate the player since the authentication has timed out"
	case ErrorCodeAuthPlayerDestroyed:
		return "the client must be restarted since another client has authenticated the current player"
	case ErrorCodeAuthServerMissing:
		return "the client must verify the server for this kind of request"
	case ErrorCodeAuthServerExpired:
		return "the client must re-verify the server since the verification has timed out"
	case ErrorCodeAuthServerDestroyed:
		return "the client must be restarted since another server has verified with the current ip/port"
	case ErrorCodePdataLocked:
		return "the client should log an error since the pdata operation did not succeed since the client is not currently holding the write lock for pdata"
	case ErrorCodeServerNotFound:
		return "the client should log an error (or if it is the server itself, attempt to register again) since the server id is not known"
	case ErrorCodeBackendServiceUnavailable:
		return "the client should try again later since a required backend service was unavailable"
	case ErrorCodeBadRequest:
		return "the client sent an invalid request (this is probably a northstar bug or a configuration error)"
	case ErrorCodeInternalError:
		return "an internal server error ocurred while processing the request (this is probably an atlas bug or downtime)"
	case ErrorCodeGenericError:
		return "an error ocurred while processing the request"
	case ErrorCodeGenericErrorFatal:
		return "an error ocurred while processing the request, and the client should not try again"
	default:
		return ""
	}
}

func (ec ErrorCode) StatusCode() int {
	switch ec {
	case ErrorCodeAuthMissing:
		return http.StatusUnauthorized
	case ErrorCodeAuthInvalid:
		return http.StatusForbidden
	case ErrorCodeAuthPlayerMissing:
		return http.StatusUnauthorized
	case ErrorCodeAuthPlayerExpired:
		return http.StatusUnauthorized
	case ErrorCodeAuthPlayerDestroyed:
		return http.StatusForbidden
	case ErrorCodeAuthServerMissing:
		return http.StatusUnauthorized
	case ErrorCodeAuthServerExpired:
		return http.StatusUnauthorized
	case ErrorCodeAuthServerDestroyed:
		return http.StatusForbidden
	case ErrorCodePdataLocked:
		return http.StatusUnauthorized
	case ErrorCodeServerNotFound:
		return http.StatusNotFound
	case ErrorCodeBackendServiceUnavailable:
		return http.StatusServiceUnavailable
	case ErrorCodeBadRequest:
		return http.StatusBadRequest
	case ErrorCodeInternalError:
		return http.StatusInternalServerError
	case ErrorCodeGenericError:
		return http.StatusInternalServerError
	case ErrorCodeGenericErrorFatal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

type Error struct {
	Code    ErrorCode
	Cause   error // not shown to clients
	Message string
}

func (e Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Code.String())
	b.WriteString(" (")
	b.WriteString(e.Code.Code())
	b.WriteString(")")
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if e.Cause != nil {
		b.WriteString(" (cause: ")
		b.WriteString(e.Cause.Error())
		b.WriteString(")")
	}
	return b.String()
}

func (e Error) Unwrap() error {
	return e.Cause
}

func (e Error) ClientError() string {
	var b strings.Builder
	b.WriteString(e.Code.String())
	b.WriteString(" (")
	b.WriteString(e.Code.Code())
	b.WriteString(")")
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if e.Cause != nil {
		b.WriteString(" (explanation: ")
		b.WriteString(e.Code.Explanation())
		b.WriteString(")")
	}
	return b.String()
}

func (e Error) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, 512)
	b = append(jsonx.AppendString(append(b, '{'), "error"), ':')
	b = jsonx.AppendString(append(jsonx.AppendString(append(b, '{'), "code"), ':'), e.Code.Code())
	if e.Message == "" {
		b = jsonx.AppendString(append(jsonx.AppendString(append(b, ','), "error"), ':'), e.Code.String())
	} else {
		b = jsonx.AppendString(append(jsonx.AppendString(append(b, ','), "error"), ':'), e.Code.String()+": "+e.Message)
	}
	if e.Code.Explanation() != "" {
		b = jsonx.AppendString(append(jsonx.AppendString(append(b, ','), "explanation"), ':'), e.Code.Explanation())
	}
	b = append(b, '}')
	b = append(b, '}')
	return b, nil
}

func (e Error) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	buf, _ := e.MarshalJSON()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
	w.WriteHeader(e.Code.StatusCode())
	w.Write(buf)
}
