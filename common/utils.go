package common

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func GetHandleFunctionName(message protoreflect.ProtoMessage) string {
	return string(message.ProtoReflect().Descriptor().FullName())
}

type CheckWork struct {
	s    atomic.Value
	Name string
}

func (th *CheckWork) Check() bool {
	if i := th.s.Load(); i != nil {
		return i.(bool)
	}
	return false
}
func (th *CheckWork) Work() {
	if !th.Check() {
		logrus.Infof("[%s]Work", th.Name)
		th.s.Store(true)
	}
}
func (th *CheckWork) Idle() {
	if th.Check() {
		logrus.Infof("[%s]Idle", th.Name)
		th.s.Store(false)
	}
}
func GetStack(depth int) string {
	if !isDebug() {
		return ""
	}
	stack := string(debug.Stack())
	ss := strings.Split(stack, "\n")
	if len(ss) > depth+2 {
		stack = strings.Join(ss[depth:depth+2], "\n")
	}
	return stack
}
func GetFrame(skipFrames int) runtime.Frame {
	// We need the frame at index skipFrames+2, since we never want runtime.Callers and getFrame
	targetFrameIndex := skipFrames + 2

	// Set size to targetFrameIndex+2 to ensure we have room for one more caller than we need
	programCounters := make([]uintptr, targetFrameIndex+2)
	n := runtime.Callers(0, programCounters)
	frame := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more, frameIndex := true, 0; more && frameIndex <= targetFrameIndex; frameIndex++ {
			var frameCandidate runtime.Frame
			frameCandidate, more = frames.Next()
			if frameIndex == targetFrameIndex {
				frame = frameCandidate
			}
		}
	}
	return frame
}

type GracefulExit struct {
	w      sync.WaitGroup
	stacks sync.Map
}

var debugGracefulExit atomic.Value

func OpenDebugGracefulExit() {
	if !isDebug() {
		debugGracefulExit.Store(true)
	}
}
func CloseDebugGracefulExit() {
	if isDebug() {
		debugGracefulExit.Store(false)
	}
}
func isDebug() bool {
	if i := debugGracefulExit.Load(); i != nil {
		return i.(bool)
	}
	return false
}
func (th *GracefulExit) Add(key int, stack string) {
	if isDebug() {
		th.stacks.Store(key, stack)
	}
	th.w.Add(1)
}
func (th *GracefulExit) Done(key int, stack string) {
	if isDebug() {
		th.stacks.Delete(key)
	}
	th.w.Done()
}
func (th *GracefulExit) Wait() {
	th.w.Wait()
}
func (th *GracefulExit) Print() {
	if isDebug() {
		th.stacks.Range(func(key, value interface{}) bool {
			logrus.Debugf("GracefulExit.Print,%v,%v", key, value)
			return true
		})
	}
}

// monitor go create and destroy
type GoFunc interface {
	Go(fn func())
	GoN(fn func(p ...interface{}), p ...interface{})
}

type DefaultGoFunc struct {
}

func (th *DefaultGoFunc) Go(fn func()) {
	go fn()
}
func (th *DefaultGoFunc) GoN(fn func(p ...interface{}), p ...interface{}) {
	go fn(p...)
}

type GracefulGoFunc struct {
	alerter  *Alerter
	exitWait *GracefulExit

	stopGo   atomic.Value
	waitGo   sync.WaitGroup
	waitTest int32
}

func (th *GracefulGoFunc) canGo() bool {
	stop := th.stopGo.Load()
	if stop != nil {
		return !stop.(bool)
	}
	return true
}
func (th *GracefulGoFunc) UpdateExitWait(exitWait *GracefulExit) {
	th.exitWait = exitWait
}
func (th *GracefulGoFunc) ConfigAlerter(config *AlerterConfigure) {
	th.alerter = NewAlerter(config)
}
func getStack() string {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	return string(buf[:n])
}

//func Recover() {
//	if err := recover(); err != nil {
//		logrus.Error(getStack())
//		os.MkdirAll("log/crash", 744)
//		crashFile := fmt.Sprintf("log/crash/crash%d-%s.log", rand.Intn(10000), time.Now().Format("2006-01-02-15-04-05"))
//		ioutil.WriteFile(crashFile, []byte(getStack()), os.ModePerm)
//		if len(*crashNotify) > 0 {
//			cmd := exec.Command(*crashNotify, crashFile)
//			if err := cmd.Run(); err != nil {
//				logrus.Errorf("call %s,err,%s", *crashNotify, err.Error())
//			}
//		}
//		if NeedCrash() {
//			panic(err)
//		}
//	}
//}
func (th *GracefulGoFunc) Recover() {
	if err := recover(); err != nil {
		statck := getStack()
		logrus.Error(statck)
		os.MkdirAll("log/crash", 744)
		crashFile := fmt.Sprintf("log/crash/crash%d-%s.log", rand.Intn(10000), time.Now().Format("2006-01-02-15-04-05"))
		ioutil.WriteFile(crashFile, []byte(getStack()), os.ModePerm)
		if th.alerter != nil {
			th.alerter.Notify("panic", statck)
		} else {
			panic(err)
		}
	}
}
func (th *GracefulGoFunc) Go(fn func()) {
	if atomic.LoadInt32(&th.waitTest) == 1 {
		//maybe error "WaitGroup is reused before previous Wait has returned"
		logrus.Errorf("Go,WaitGo,%s", getStack())
	}
	if !th.canGo() {
		logrus.Errorf("Go,Wait,%s", getStack())
		return
	}
	stack := GetStack(7)
	r := rand.Int()
	th.exitWait.Add(r, stack)
	th.waitGo.Add(1)
	go func() {
		th.waitGo.Done()
		defer func() {
			th.exitWait.Done(r, stack)
			th.Recover()
		}()
		fn()
	}()
}
func (th *GracefulGoFunc) GoN(fn func(p ...interface{}), p ...interface{}) {
	if atomic.LoadInt32(&th.waitTest) == 1 {
		//maybe error "WaitGroup is reused before previous Wait has returned"
		logrus.Errorf("GoN,WaitGo,%s", getStack())
	}
	if !th.canGo() {
		logrus.Errorf("GoN,Wait,%s", getStack())
		return
	}
	stack := GetStack(7)
	r := rand.Int()
	th.exitWait.Add(r, stack)
	th.waitGo.Add(1)
	go func() {
		th.waitGo.Done()
		defer func() {
			th.exitWait.Done(r, stack)
			th.Recover()
		}()
		fn(p...)
	}()
}

//make sure all go running
func (th *GracefulGoFunc) WaitGo() {
	atomic.StoreInt32(&th.waitTest, 1)
	th.waitGo.Wait()
	atomic.StoreInt32(&th.waitTest, 0)
}
func (th *GracefulGoFunc) Add() {
	th.exitWait.Add(1, GetStack(7))
}
func (th *GracefulGoFunc) Done() {
	th.exitWait.Done(1, GetStack(7))
}
func (th *GracefulGoFunc) Wait() {
	th.stopGo.Store(true)
	th.exitWait.Wait()
}
func (th *GracefulGoFunc) PrintGoroutineStack() {
	th.exitWait.Print()
}

func GoID() int {
	if !isDebug() {
		return 0
	}
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}

type TimeElapse struct {
	t        int64
	lastCall string
	disable  bool
}

func (th *TimeElapse) Disable() {
	th.disable = true
}

func (th *TimeElapse) Call(call string) {
	if th.disable {
		return
	}
	t := time.Now().UnixNano()
	if th.t > 0 && t-th.t > 0 {
		logrus.Warnf("%s = > %s(%d)", th.lastCall, call, t-th.t)
	}
	th.t = t
	th.lastCall = call
}
