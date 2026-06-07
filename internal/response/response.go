package response

import (
	"net/http"

	"github.com/gogf/gf/v2/net/ghttp"
)

const (
	CodeOK           = 0
	CodeBadRequest   = 400
	CodeUnauthorized = 401
	CodeForbidden    = 403
	CodeNotFound     = 404
	CodeInternal     = 500
)

type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func Success(r *ghttp.Request, data any) {
	JSON(r, http.StatusOK, CodeOK, "ok", data)
}

func Created(r *ghttp.Request, data any) {
	JSON(r, http.StatusCreated, CodeOK, "ok", data)
}

func Error(r *ghttp.Request, status int, code int, message string) {
	if message == "" {
		message = http.StatusText(status)
	}
	JSON(r, status, code, message, nil)
}

func BadRequest(r *ghttp.Request, message string) {
	Error(r, http.StatusBadRequest, CodeBadRequest, message)
}

func Unauthorized(r *ghttp.Request, message string) {
	Error(r, http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(r *ghttp.Request, message string) {
	Error(r, http.StatusForbidden, CodeForbidden, message)
}

func NotFound(r *ghttp.Request, message string) {
	Error(r, http.StatusNotFound, CodeNotFound, message)
}

func Internal(r *ghttp.Request, message string) {
	Error(r, http.StatusInternalServerError, CodeInternal, message)
}

func JSON(r *ghttp.Request, status int, code int, message string, data any) {
	r.Response.WriteStatus(status)
	r.Response.WriteJson(Body{
		Code:      code,
		Message:   message,
		Data:      data,
		RequestID: r.Request.Header.Get("X-Request-ID"),
	})
}
