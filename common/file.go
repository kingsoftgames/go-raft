package common

import (
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type fileInfo struct {
	name string
	info os.FileInfo
	cb   func(string)
}

var files map[string]*fileInfo
var l sync.Mutex
var ticker *Ticker

func AddWatch(fileName string, cb func(string)) {
	l.Lock()
	defer l.Unlock()
	if files == nil {
		files = map[string]*fileInfo{}
	}
	info, err := os.Stat(fileName)
	if err != nil {
		logrus.Errorf("AddWatch %s error,%s", fileName, err.Error())
		return
	}
	files[fileName] = &fileInfo{
		name: fileName,
		info: info,
		cb:   cb,
	}
}
func RemoveWatch(fileName string) {
	l.Lock()
	defer l.Unlock()
	delete(files, fileName)
}
func StartWatch() {
	ticker = NewTicker(time.Millisecond*50, func() {
		l.Lock()
		defer l.Unlock()
		for _, f := range files {
			info, err := os.Stat(f.name)
			if err == nil {
				if f.info.ModTime() != info.ModTime() || f.info.Size() != info.Size() || f.info.Mode() != info.Mode() {
					f.cb(f.name)
					f.info = info
				}
			}
		}
	})
}
func StopWatch() {
	ticker.Stop()
}
