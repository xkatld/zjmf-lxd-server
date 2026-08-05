package funcexec

import "time"

type Result struct {
	FuncName   string
	Success    bool
	ErrorCode  int
	Message    string
	Suggestion string
	Duration   time.Duration
	Data       map[string]interface{}
	StartTime  time.Time
	EndTime    time.Time
}

