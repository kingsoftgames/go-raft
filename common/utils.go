package common

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
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
	stack := string(debug.Stack())
	ss := strings.Split(stack, "\n")
	if len(ss) > depth+2 {
		stack = strings.Join(ss[depth:depth+2], "\n")
	}
	return stack
}

type GracefulExit struct {
	w sync.WaitGroup
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
func (th *GracefulExit) Add(stack string) {
	if isDebug() {
		logrus.Debugf("GracefulExit Add(%s)", stack)
	}
	th.w.Add(1)
}
func (th *GracefulExit) Done(stack string) {
	if isDebug() {
		logrus.Debugf("GracefulExit Done(%s)", stack)
	}
	th.w.Done()
}
func (th *GracefulExit) Wait() {
	th.w.Wait()
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
	exitWait *GracefulExit

	stopGo atomic.Value
	waitGo sync.WaitGroup
}

func (th *GracefulGoFunc) CanGo() bool {
	stop := th.stopGo.Load()
	if stop != nil {
		return !stop.(bool)
	}
	return true
}
func (th *GracefulGoFunc) UpdateExitWait(exitWait *GracefulExit) {
	th.exitWait = exitWait
}
func getStack() string {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	return string(buf[:n])
}
func Recover() {
	if err := recover(); err != nil {
		logrus.Error(getStack())
		os.MkdirAll("log/crash", os.ModePerm)
		crashFile := fmt.Sprintf("log/crash/crash%d-%s.log", rand.Intn(10000), time.Now().Format("2006-01-02-15-04-05"))
		ioutil.WriteFile(crashFile, []byte(getStack()), os.ModePerm)
		if len(*crashNotify) > 0 {
			cmd := exec.Command(*crashNotify, crashFile)
			if err := cmd.Run(); err != nil {
				logrus.Errorf("call %s,err,%s", *crashNotify, err.Error())
			}
		}
		if NeedCrash() {
			panic(err)
		}
	}
}
func (th *GracefulGoFunc) Go(fn func()) {
	if !th.CanGo() {
		return
	}
	stack := GetStack(7)
	th.exitWait.Add(stack)
	th.waitGo.Add(1)
	go func() {
		defer Recover()
		th.waitGo.Done()
		defer th.exitWait.Done(stack)
		fn()
	}()
}
func (th *GracefulGoFunc) GoN(fn func(p ...interface{}), p ...interface{}) {
	if !th.CanGo() {
		return
	}
	stack := GetStack(7)
	th.exitWait.Add(stack)
	th.waitGo.Add(1)
	go func() {
		defer Recover()
		th.waitGo.Done()
		defer th.exitWait.Done(stack)
		fn(p...)
	}()
}

//make sure all go running
func (th *GracefulGoFunc) WaitGo() {
	th.waitGo.Wait()
}
func (th *GracefulGoFunc) Add() {
	th.exitWait.Add(GetStack(7))
}
func (th *GracefulGoFunc) Done() {
	th.exitWait.Done(GetStack(7))
}
func (th *GracefulGoFunc) Wait() {
	th.stopGo.Store(true)
	th.exitWait.Wait()
}
