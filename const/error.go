package appconst

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	// CodeOK 请求处理成功。
	CodeOK = 0
	// CodeInvalidParams 请求参数无效。
	CodeInvalidParams = 40001
	// CodeUnauthorized 未登录或登录会话无效。
	CodeUnauthorized = 40101
	// CodeNotFound 请求的资源不存在。
	CodeNotFound = 40401
	// CodeConflict 业务状态或数据发生冲突。
	CodeConflict = 40901
	// CodeInternal 内部系统错误。
	CodeInternal = 50001
	// CodeDatabase 数据库操作错误。
	CodeDatabase = 50002
)

// MsgError 是应用层错误。Err 只用于日志和错误追踪，不返回给前端。
type MsgError struct {
	Code       int
	Message    string
	Err        error
	HTTPStatus int
}

func (e *MsgError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *MsgError) Unwrap() error { return e.Err }

func NewInvalidParamsError(message string) *MsgError {
	return &MsgError{Code: CodeInvalidParams, Message: message, HTTPStatus: http.StatusBadRequest}
}

func NewUnauthorizedError(message string) *MsgError {
	return &MsgError{Code: CodeUnauthorized, Message: message, HTTPStatus: http.StatusUnauthorized}
}

func NewConflictError(message string) *MsgError {
	return &MsgError{Code: CodeConflict, Message: message, HTTPStatus: http.StatusConflict}
}

func NewNotFoundError(message string) *MsgError {
	return &MsgError{Code: CodeNotFound, Message: message, HTTPStatus: http.StatusNotFound}
}

// NewInternalError 保留已有业务错误，仅将未包装的错误转换为系统内部错误。
func NewInternalError(err error) *MsgError {
	if msgErr, ok := AsMsgError(err); ok {
		return msgErr
	}
	return &MsgError{Code: CodeInternal, Message: "系统繁忙，请稍后重试", Err: err, HTTPStatus: http.StatusInternalServerError}
}

func AsMsgError(err error) (*MsgError, bool) {
	var msgErr *MsgError
	ok := errors.As(err, &msgErr)
	return msgErr, ok && msgErr != nil
}
