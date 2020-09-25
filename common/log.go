package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/orandin/lumberjackrus"

	_ "github.com/orandin/lumberjackrus"
	"github.com/sirupsen/logrus"
)

const (
	colorNoColor = "\033[0m"
	colorRed     = "\033[91m"
	colorGreen   = "\033[92m"
	colorYellow  = "\033[93m"
	colorMagenta = "\033[95m"
	colorCyan    = "\033[96m"
	colorWhite   = "\033[89m"

	timeFormat = "2006-01-02 15:04:05.000"
)

type textFormat struct {
	forceColors      bool
	PrettyPrint      bool
	CallerPrettyfier func(*runtime.Frame) (function string, file string)
}

func (f *textFormat) Format(entry *logrus.Entry) ([]byte, error) {
	levelText := strings.ToUpper(entry.Level.String())[:4]
	buf := bytes.NewBuffer(make([]byte, 0))
	if f.forceColors {
		color := colorNoColor
		switch entry.Level {
		case logrus.DebugLevel:
			color = colorCyan
		case logrus.InfoLevel:
			color = colorWhite
		case logrus.WarnLevel:
			color = colorYellow
		case logrus.ErrorLevel:
			color = colorMagenta
		case logrus.PanicLevel, logrus.FatalLevel:
			color = colorRed
		}
		buf.WriteString(color)
	}
	if app, ok := entry.Data["app"]; ok {
		buf.WriteString(fmt.Sprintf("[%s]", app))
	}
	buf.WriteString(fmt.Sprintf("[%s]", entry.Time.Format(timeFormat)))
	buf.WriteString(fmt.Sprintf("[%s]", levelText))

	if entry.HasCaller() {
		var funcVal, fileVal string
		if f.CallerPrettyfier != nil {
			funcVal, fileVal = f.CallerPrettyfier(entry.Caller)
		} else {
			funcVal = entry.Caller.Function
			fileVal = fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line)
		}
		buf.WriteString(fmt.Sprintf("[%s:%d:%s]", fileVal, entry.Caller.Line, funcVal))
	}

	buf.WriteString("\t" + entry.Message)
	if len(entry.Data) > 0 {
		encoder := json.NewEncoder(buf)
		if f.PrettyPrint {
			encoder.SetIndent("", "  ")
		}

		if err := encoder.Encode(entry.Data); err != nil {
			return nil, fmt.Errorf("failed to marshal fields to JSON, %v", err)
		}
	} else {
		buf.WriteString(fmt.Sprintf("\n"))
	}
	if f.forceColors {
		buf.WriteString(colorNoColor)
	}

	return buf.Bytes(), nil
}
func newTextFormat(color bool) *textFormat {
	return &textFormat{
		forceColors: color,
	}
}

var once sync.Once

func getLogLevel(level string) logrus.Level {
	l, err := logrus.ParseLevel(level)
	if err != nil {
		l = logrus.InfoLevel
	}
	return l
}
func InitLog(config *LogConfigure) {
	once.Do(func() {
		gin.DefaultWriter = NewFileLog(config, "", "gin")
		gin.DefaultErrorWriter = NewFileLog(config, "", "gin_err")
		logrus.SetLevel(getLogLevel(config.Level))
		logrus.SetOutput(os.Stdout)
		logrus.SetFormatter(newTextFormat(true))
		addFileHook(config)
	})
}

func addFileHook(config *LogConfigure) error {
	level := getLogLevel(config.Level)
	hook, err := lumberjackrus.NewHook(
		&lumberjackrus.LogFile{
			Filename:  fmt.Sprintf("%s/info.log", config.Path),
			MaxAge:    config.MaxAge,
			MaxSize:   config.MaxSize,
			Compress:  config.Compress,
			LocalTime: false,
		},
		level,
		&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
		},
		&lumberjackrus.LogFileOpts{
			logrus.ErrorLevel: &lumberjackrus.LogFile{
				Filename:  fmt.Sprintf("%s/error.log", config.Path),
				MaxAge:    config.MaxAge,
				MaxSize:   config.MaxSize,
				Compress:  config.Compress,
				LocalTime: false,
			},
			logrus.FatalLevel: &lumberjackrus.LogFile{
				Filename:  fmt.Sprintf("%s/fatal.log", config.Path),
				MaxAge:    config.MaxAge,
				MaxSize:   config.MaxSize,
				Compress:  config.Compress,
				LocalTime: false,
			},
		},
	)
	if err != nil {
		return err
	}
	logrus.AddHook(hook)
	logrus.AddHook(&StdErrHook{})
	return nil
}

type StdErrHook struct {
	format logrus.JSONFormatter
}

func (th *StdErrHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel}
}
func (th *StdErrHook) Fire(entry *logrus.Entry) error {
	msg, err := th.format.Format(entry)
	if err != nil {
		return err
	}
	_, err = os.Stderr.Write(msg)
	return err
}

type LogFileWrite struct {
	logger *logrus.Logger
	tag    string
}

func (th *LogFileWrite) Write(p []byte) (n int, err error) {
	if len(th.tag) > 0 {
		th.logger.WithField("logger", th.tag).Infof(string(p))
	} else {
		th.logger.Info(string(p))
	}
	return 0, nil
}

func NewFileLog(config *LogConfigure, tag, name string) *LogFileWrite {
	l := &LogFileWrite{
		tag: tag,
	}
	l.logger = logrus.New()
	l.logger.SetOutput(os.Stdout)
	hook, err := lumberjackrus.NewHook(
		&lumberjackrus.LogFile{
			Filename:  fmt.Sprintf("%s/%s.log", config.Path, name),
			MaxAge:    config.MaxAge,
			MaxSize:   config.MaxSize,
			Compress:  config.Compress,
			LocalTime: false,
		},
		logrus.DebugLevel,
		&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
		},
		nil,
	)
	if err != nil {
		return nil
	}
	l.logger.AddHook(hook)
	return l
}

var debugLog = false

func IsOpenDebugLog() bool {
	return debugLog
}
func OpenDebugLog() {
	debugLog = true
}
func Debugf(msg string, args ...interface{}) {
	if debugLog {
		logrus.Debugf(msg, args...)
	}
}
