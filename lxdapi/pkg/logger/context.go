package logger

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

var ctxKey = contextKey{}

type Context struct {
	TraceID   string
	TaskID    uint
	Container string
	Action    string
	ClientIP  string
}

func NewContext(ctx context.Context, lc *Context) context.Context {
	if lc.TraceID == "" {
		lc.TraceID = uuid.New().String()
	}
	return context.WithValue(ctx, ctxKey, lc)
}

func FromContext(ctx context.Context) *Context {
	if lc, ok := ctx.Value(ctxKey).(*Context); ok {
		return lc
	}
	return &Context{
		TraceID: uuid.New().String(),
	}
}

