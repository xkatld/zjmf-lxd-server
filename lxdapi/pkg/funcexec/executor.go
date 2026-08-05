package funcexec

import (
	"context"
	"time"

	"lxdapi/errors"
)

type Executor struct {
	ctx     context.Context
	results []*Result
}

func New(ctx context.Context) *Executor {
	return &Executor{
		ctx:     ctx,
		results: make([]*Result, 0),
	}
}

func (e *Executor) Run(name string, fn func(*FuncLog) error) error {
	log := newFuncLog(e.ctx, name)
	
	result := &Result{
		FuncName:  name,
		StartTime: time.Now(),
		Data:      make(map[string]interface{}),
	}
	
	err := fn(log)
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	if err != nil {
		result.Success = false
		
		if appErr, ok := err.(*errors.AppError); ok {
			result.ErrorCode = appErr.Code
			result.Message = appErr.Message
			result.Suggestion = appErr.Suggestion
			if appErr.Detail != "" {
				result.Message = result.Message + ": " + appErr.Detail
			}
		} else {
			result.ErrorCode = errors.ERR_SYSTEM_UNKNOWN
			result.Message = err.Error()
			result.Suggestion = errors.GetSuggestion(errors.ERR_SYSTEM_UNKNOWN)
		}
	} else {
		result.Success = true
		result.ErrorCode = errors.ERR_SUCCESS
	}
	
	e.results = append(e.results, result)
	
	return err
}

func (e *Executor) Results() []*Result {
	return e.results
}

func (e *Executor) FailedFunc() *Result {
	for _, r := range e.results {
		if !r.Success {
			return r
		}
	}
	return nil
}

