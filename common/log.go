package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

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
func InitLog() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(os.Stdout)
	logrus.SetFormatter(newTextFormat(true))
}
