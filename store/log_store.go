package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"github.com/sirupsen/logrus"

	raftboltdb "github.com/hashicorp/raft-boltdb"

	"github.com/hashicorp/raft"
)

type LogStoreCache struct {
	curIdx    int
	lowIndex  uint64
	highIndex uint64
	cache     []*raft.Log
	capacity  int
	store     *raftboltdb.BoltStore

	l          sync.RWMutex
	firstIndex uint64
	lastIndex  uint64

	name string
}

var debugCrash = false

func OpenLogStoreDebugCrash() {
	debugCrash = true
}

func NewLogStoreCache(capacity int, path string) (*LogStoreCache, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("capacity must be positive")
	}
	if err := os.MkdirAll(path, 744); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("logStoreCache path not accessible: %v", err)
	}
	btDB, err := raftboltdb.NewBoltStore(filepath.Join(path, "log.db"))
	if err != nil {
		return nil, err
	}
	c := &LogStoreCache{
		store:    btDB,
		capacity: capacity,
		cache:    make([]*raft.Log, capacity),
	}
	if common.IsOpenDebugLog() {
		paths := strings.Split(path, "\\")
		c.name = paths[len(paths)-1]
	}

	if c.firstIndex, err = btDB.FirstIndex(); err != nil {
		return nil, err
	}
	if c.lastIndex, err = btDB.LastIndex(); err != nil {
		return nil, err
	}
	return c, nil
}
func (th *LogStoreCache) GetLog(idx uint64, log *raft.Log) error {
	th.l.RLock()
	defer th.l.RUnlock()
	common.Debugf("[%s]GetLog,curIdx:%d,lowIdx:%d,highIdx:%d,firstIdx:%d,lastIdx:%d,%d", th.name, th.curIdx, th.lowIndex, th.highIndex, th.firstIndex, th.lastIndex, idx)
	if idx >= th.lowIndex && idx <= th.highIndex {
		if idx-th.lowIndex >= uint64(th.capacity) {
			logrus.Errorf("[%s]GetLog,curIdx:%d,lowIdx:%d,highIdx:%d,firstIdx:%d,lastIdx:%d,%d", th.name, th.curIdx, th.lowIndex, th.highIndex, th.firstIndex, th.lastIndex, idx)
		}
		cached := th.cache[idx-th.lowIndex]
		if cached != nil && cached.Index == idx {
			*log = *cached
			return nil
		}
	}
	//logrus.Warnf("[%s]GetLog,%d,%d,%d,%d", th.name, th.curIdx, th.lowIndex, th.highIndex, idx)
	return th.store.GetLog(idx, log)
}

func (th *LogStoreCache) StoreLog(log *raft.Log) error {
	common.Debugf("[%s]StoreLog,curIdx:%d,lowIdx:%d,highIdx:%d,firstIdx:%d,lastIdx:%d,%d", th.name, th.curIdx, th.lowIndex, th.highIndex, th.firstIndex, th.lastIndex, log.Index)
	return th.StoreLogs([]*raft.Log{log})
}

func (th *LogStoreCache) StoreLogs(logs []*raft.Log) error {
	th.l.Lock()
	defer th.l.Unlock()
	common.Debugf("[%s]StoreLogs,curIdx:%d,lowIdx:%d,highIdx:%d,firstIdx:%d,lastIdx:%d", th.name, th.curIdx, th.lowIndex, th.highIndex, th.firstIndex, th.lastIndex)
	for i, l := range logs {
		common.Debugf("[%s]StoreLogs[%d],%d,%d,%d", th.name, i, l.Index, l.Term, l.Type)
	}
	for _, l := range logs {
		th.cache[th.curIdx] = l
		th.curIdx++
		if th.firstIndex == 0 {
			th.firstIndex = l.Index
		}
		if th.lowIndex == 0 {
			th.lowIndex = l.Index
		}
		if l.Index < th.lowIndex {
			th.lowIndex = l.Index
		}
		if l.Index > th.highIndex {
			th.highIndex = l.Index
		}
		if l.Index > th.lastIndex {
			th.lastIndex = l.Index
		}
		if th.curIdx == th.capacity {
			if err := th.flushToDB(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (th *LogStoreCache) FirstIndex() (uint64, error) {
	th.l.RLock()
	defer th.l.RUnlock()
	common.Debugf("[%s]FirstIndex,curIdx:%d,lowIdx:%d,highIdx:%d,firstIdx:%d,lastIdx:%d", th.name, th.curIdx, th.lowIndex, th.highIndex, th.firstIndex, th.lastIndex)
	return th.firstIndex, nil
}

func (th *LogStoreCache) LastIndex() (uint64, error) {
	th.l.RLock()
	defer th.l.RUnlock()
	common.Debugf("[%s]LastIndex,curIdx:%d,lowIdx:%d,highIdx:%d,firstIdx:%d,lastIdx:%d", th.name, th.curIdx, th.lowIndex, th.highIndex, th.firstIndex, th.lastIndex)
	return th.lastIndex, nil
}

func (th *LogStoreCache) DeleteRange(min, max uint64) error {
	th.l.Lock()
	defer th.l.Unlock()
	common.Debugf("[%s]DeleteRange,curIdx:%d,lowIdx:%d,highIdx:%d,firstIdx:%d,lastIdx:%d,%d,%d", th.name, th.curIdx, th.lowIndex, th.highIndex, th.firstIndex, th.lastIndex, min, max)
	if err := th.flushToDB(); err != nil {
		return err
	}
	return th.store.DeleteRange(min, max)
}
func (th *LogStoreCache) flushToDB() error {
	common.Debugf("[%s]flushToDB,curIdx:%d,lowIdx:%d,highIdx:%d,firstIdx:%d,lastIdx:%d", th.name, th.curIdx, th.lowIndex, th.highIndex, th.firstIndex, th.lastIndex)
	if th.curIdx > 0 {
		if err := th.store.StoreLogs(th.cache[0:th.curIdx]); err != nil {
			logrus.Debugf("flushToDB err,%s", err.Error())
			return err
		}
		th.cache = make([]*raft.Log, th.capacity)
		th.curIdx = 0
		th.lowIndex = 0
		th.highIndex = 0
	}
	return nil
}

func (th *LogStoreCache) Close() {
	th.l.Lock()
	defer th.l.Unlock()
	common.Debugf("[%s]Close,curIdx:%d,lowIdx:%d,highIdx:%d,firstIdx:%d,lastIdx:%d", th.name, th.curIdx, th.lowIndex, th.highIndex, th.firstIndex, th.lastIndex)
	if th.store != nil {
		if !debugCrash {
			th.flushToDB()
		}
		if err := th.store.Close(); err != nil {
			logrus.Errorf("store.Close,err,%s", err.Error())
		}
	}
}
