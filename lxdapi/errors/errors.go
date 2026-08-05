package errors

import "fmt"

type AppError struct {
	Code       int
	Message    string
	Detail     string
	Suggestion string
	FuncName   string
	Cause      error
	TraceID    string
	Context    map[string]interface{}
}

func NewAppError(funcName string, code int, message string, cause error) *AppError {
	err := &AppError{
		Code:       code,
		Message:    message,
		FuncName:   funcName,
		Cause:      cause,
		Suggestion: GetSuggestion(code),
		Context:    make(map[string]interface{}),
	}
	
	if cause != nil {
		err.Detail = cause.Error()
	}
	
	return err
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%s] %s: %s (code: %d)", e.FuncName, e.Message, e.Detail, e.Code)
	}
	return fmt.Sprintf("[%s] %s (code: %d)", e.FuncName, e.Message, e.Code)
}

func (e *AppError) WithContext(key string, value interface{}) *AppError {
	e.Context[key] = value
	return e
}

func (e *AppError) WithTraceID(traceID string) *AppError {
	e.TraceID = traceID
	return e
}

