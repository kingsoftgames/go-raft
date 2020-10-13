package app

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/hashicorp/raft"
)

type HandlerValue struct {
	paramReq reflect.Type
	paramRsp reflect.Type
	fn       reflect.Value
}
type Handler struct {
	h map[string]*HandlerValue
}

type FutureSlice struct {
	f []raft.ApplyFuture
	t int64
}

var cnt, totalTime int64

func GetFutureAve() string {
	return fmt.Sprintf("FutureAve cnt %d, totalTime %dmics,ave %vmics", cnt, totalTime, totalTime/cnt)
}

//TODO only support one this version
func (th *FutureSlice) Add(f raft.ApplyFuture) {
	if th.f == nil {
		th.f = make([]raft.ApplyFuture, 0)
	}
	if len(th.f) >= 1 {
		panic("only support one future")
	}
	th.f = append(th.f, f)
	th.t = time.Now().UnixNano() / 1e3
}
func (th *FutureSlice) foreach(cb func(int, raft.ApplyFuture)) {
	for i, f := range th.f {
		cb(i, f)
	}
}
func (th *FutureSlice) Len() int {
	return len(th.f)
}

func (th *FutureSlice) Error() error {
	defer func() {
		atomic.AddInt64(&cnt, 1)
		atomic.AddInt64(&totalTime, time.Now().UnixNano()/1e3-th.t)
	}()
	if th.Len() == 1 {
		return th.f[0].Error()
	} else if th.Len() > 1 {
		var err FutureSliceError = make(FutureSliceError, th.Len())
		var w sync.WaitGroup
		th.foreach(func(i int, future raft.ApplyFuture) {
			w.Add(1)
			go func() {
				defer w.Done()
				if e := future.Error(); e != nil {
					err[i] = e
				}
			}()
		})
		w.Wait()

		return err.Error()
	}
	return nil
}
func (th *FutureSlice) Clear() {
	th.f = nil
}

type FutureSliceError []error

func (th *FutureSliceError) add(err error) {
	if *th == nil {
		*th = make([]error, 0)
	}
	*th = append(*th, err)
}
func (th *FutureSliceError) isNil() bool {
	return *th == nil
}
func (th FutureSliceError) Error() error {
	if th == nil {
		return nil
	}
	errs := make([]error, 0)
	for _, e := range th {
		if e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("FutureSliceError %v", th)
	}
	return nil
}

type HandlerRtv struct {
	Err     error
	Futures FutureSlice
}

func (th *HandlerRtv) Clear() {
	th.Err = nil
	th.Futures.Clear()
}

func (th *Handler) Register(impl interface{}) {
	th.h = map[string]*HandlerValue{}
	implType := reflect.TypeOf(impl)
	for i := 0; i < implType.NumMethod(); i++ {
		method := implType.Method(i)
		if method.Func.Type().NumIn() >= 2 && method.Func.Type().In(1).Kind() == reflect.Ptr {
			first := reflect.New(method.Func.Type().In(1).Elem()).Interface()
			if msg, ok := first.(protoreflect.ProtoMessage); ok { //注册App中的逻辑处理
				logrus.Infof("GRpcRegister LogicHandler : %s", method.Name)
				methodName := common.GetHandleFunctionName(msg)

				th.h[methodName] = &HandlerValue{
					fn:       method.Func,
					paramReq: method.Func.Type().In(1).Elem(),
					paramRsp: method.Func.Type().In(2).Elem(),
				}
			}
		} else if method.Func.Type().NumOut() == 2 && method.Func.Type().Out(0).Kind() == reflect.Ptr {
			firstReturn := reflect.New(method.Func.Type().Out(0).Elem()).Interface()
			if _, ok := firstReturn.(protoreflect.ProtoMessage); ok {
				req := reflect.New(method.Func.Type().In(2).Elem()).Interface()
				if msg, ok := req.(protoreflect.ProtoMessage); ok {
					logrus.Infof("GRpcRegister grpc client: %s", method.Name)
					methodName := common.GetHandleFunctionName(msg)
					th.h[methodName] = &HandlerValue{
						fn: method.Func,
					}
				}
			}
		}
	}
}
func (th *Handler) Handle(method string, v []reflect.Value) ([]reflect.Value, error) {
	if h, ok := th.h[method]; ok {
		return h.fn.Call(v), nil
	}
	return nil, fmt.Errorf("grpc lost method,%s", method)
}

func (th *Handler) GetHandlerValue(method string) *HandlerValue {
	return th.h[method]
}

func (th *Handler) Foreach(cb func(name string, hd *HandlerValue)) {
	for name, hd := range th.h {
		cb(name, hd)
	}
}
