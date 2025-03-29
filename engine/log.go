package engine

import "github.com/zishang520/engine.io/v2/log"

type Log struct {
	*log.Log
}

func NewLog(prefix string) *Log {
	return &Log{Log: log.NewLog(prefix)}
}

func (l *Log) Debugf(message string, args ...any) {
	l.Debug(message, args...)
}

func (l *Log) Errorf(message string, args ...any) {
	l.Error(message, args...)
}

func (l *Log) Warnf(message string, args ...any) {
	l.Warning(message, args...)
}
