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

type FileWatch struct {
	files  map[string]*fileInfo
	l      sync.Mutex
	ticker *Ticker
	goFunc GoFunc
}

func NewFileWatch(goFunc GoFunc) *FileWatch {
	return &FileWatch{
		goFunc: goFunc,
	}
}

func (th *FileWatch) Add(fileName string, cb func(string)) {
	th.l.Lock()
	defer th.l.Unlock()
	if th.files == nil {
		th.files = map[string]*fileInfo{}
	}
	info, err := os.Stat(fileName)
	if err != nil {
		logrus.Errorf("AddWatch %s error,%s", fileName, err.Error())
		return
	}
	th.files[fileName] = &fileInfo{
		name: fileName,
		info: info,
		cb:   cb,
	}
}
func (th *FileWatch) RemoveWatch(fileName string) {
	th.l.Lock()
	defer th.l.Unlock()
	delete(th.files, fileName)
}
func (th *FileWatch) Start() {
	th.ticker = NewTickerWithGo(time.Millisecond*50, func() {
		th.l.Lock()
		defer th.l.Unlock()
		for _, f := range th.files {
			info, err := os.Stat(f.name)
			if err == nil {
				if f.info.ModTime() != info.ModTime() || f.info.Size() != info.Size() || f.info.Mode() != info.Mode() {
					f.cb(f.name)
					f.info = info
				}
			}
		}
	}, th.goFunc)
}
func (th *FileWatch) Stop() {
	if th.ticker != nil {
		th.ticker.Stop()
	}
}
