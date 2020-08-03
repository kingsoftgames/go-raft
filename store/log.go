package store

import (
	"io"
	"log"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/hashicorp/go-hclog"
)

type raftLog struct {
}

func (th *raftLog) Log(level hclog.Level, msg string, args ...interface{}) {
	panic("implement me")
}

func (th *raftLog) ImpliedArgs() []interface{} {
	panic("implement me")
}

func (th *raftLog) Name() string {
	panic("implement me")
}

func (th *raftLog) Trace(msg string, args ...interface{}) {
	logrus.Tracef(msg, args...)
}
func (th *raftLog) Debug(msg string, args ...interface{}) {
	logrus.Debugf(msg, args...)
}
func (th *raftLog) Info(msg string, args ...interface{}) {
	logrus.Infof(msg, args...)
}
func (th *raftLog) Warn(msg string, args ...interface{}) {
	logrus.Warnf(msg, args...)
}
func (th *raftLog) Error(msg string, args ...interface{}) {
	logrus.Errorf(msg, args...)
}
func (th *raftLog) IsTrace() bool {
	return logrus.IsLevelEnabled(logrus.TraceLevel)
}
func (th *raftLog) IsDebug() bool {
	return logrus.IsLevelEnabled(logrus.DebugLevel)
}
func (th *raftLog) IsInfo() bool {
	return logrus.IsLevelEnabled(logrus.InfoLevel)
}
func (th *raftLog) IsWarn() bool {
	return logrus.IsLevelEnabled(logrus.WarnLevel)
}
func (th *raftLog) IsError() bool {
	return logrus.IsLevelEnabled(logrus.ErrorLevel)
}

func (th *raftLog) With(args ...interface{}) hclog.Logger {
	return th
}
func (th *raftLog) Named(name string) hclog.Logger {
	return th
}
func (th *raftLog) ResetNamed(name string) hclog.Logger {
	return th
}
func (th *raftLog) SetLevel(level hclog.Level) {
}
func (th *raftLog) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	if opts == nil {
		opts = &hclog.StandardLoggerOptions{}
	}
	return log.New(th.StandardWriter(opts), "", 0)
}
func (th *raftLog) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return os.Stdout
}
