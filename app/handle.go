package app

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/WANGXU13/ppx-app/common"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/hashicorp/raft"
)

type Handler struct {
	h map[string]reflect.Value
}

type FutureSlice []raft.ApplyFuture

func (th *FutureSlice) Add(f raft.ApplyFuture) {
	if *th == nil {
		*th = make([]raft.ApplyFuture, 0)
	}
	*th = append(*th, f)
}
func (th *FutureSlice) foreach(cb func(int, raft.ApplyFuture)) {
	for i, f := range *th {
		cb(i, f)
	}
}
func (th *FutureSlice) Len() int {
	return len(*th)
}

func (th *FutureSlice) Error() error {
	if th.Len() == 1 {
		return (*th)[0].Error()
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
	Err error
	//Future  raft.ApplyFuture
	Futures FutureSlice
}

//func (th *Handler) Register(impl interface{}) {
//	th.h = map[string]reflect.Value{}
//	implType := reflect.TypeOf(impl)
//	for i := 0; i < implType.NumMethod(); i++ {
//		method := implType.Method(i)
//		if method.Func.Type().NumIn() == 4 { //注册App中的逻辑处理
//			app := reflect.New(method.Func.Type().In(0).Elem()).Interface()
//			if _, ok := app.(IApp); ok {
//				req := reflect.New(method.Func.Type().In(1).Elem()).Interface()
//				if msg, ok := req.(protoreflect.ProtoMessage); ok {
//					logrus.Infof("GRpcRegister LogicHandler : %s", method.Name)
//					methodName := common.GetHandleFunctionName(msg)
//					th.h[methodName] = method.Func
//				}
//			}
//		} else if method.Func.Type().NumIn() >= 2 { //注册grpcClient
//			ctx := reflect.New(method.Func.Type().In(1).Elem()).Interface()
//			if _, ok := ctx.(context.Context); ok {
//				req := reflect.New(method.Func.Type().In(1).Elem()).Interface()
//				if msg, ok := req.(protoreflect.ProtoMessage); ok {
//					logrus.Infof("GRpcRegister grpc client: %s", method.Name)
//					methodName := common.GetHandleFunctionName(msg)
//					th.h[methodName] = method.Func
//				}
//			}
//		}
//		//if method.Func.Type().NumIn() == 4 {
//		//	req := reflect.New(method.Func.Type().In(1).Elem()).Interface()
//		//	if msg, ok := req.(protoreflect.ProtoMessage); ok {
//		//		logrus.Infof("GRpcRegister : %s", method.Name)
//		//		methodName := common.GetHandleFunctionName(msg)
//		//		th.h[methodName] = method.Func
//		//	}
//		//}
//	}
//}

func (th *Handler) Register(impl interface{}) {
	th.h = map[string]reflect.Value{}
	implType := reflect.TypeOf(impl)
	for i := 0; i < implType.NumMethod(); i++ {
		method := implType.Method(i)
		if method.Func.Type().NumIn() >= 2 && method.Func.Type().In(1).Kind() == reflect.Ptr {
			first := reflect.New(method.Func.Type().In(1).Elem()).Interface()
			if msg, ok := first.(protoreflect.ProtoMessage); ok { //注册App中的逻辑处理
				logrus.Infof("GRpcRegister LogicHandler : %s", method.Name)
				methodName := common.GetHandleFunctionName(msg)
				th.h[methodName] = method.Func
			}
		} else if method.Func.Type().NumOut() == 2 && method.Func.Type().Out(0).Kind() == reflect.Ptr {
			firstReturn := reflect.New(method.Func.Type().Out(0).Elem()).Interface()
			if _, ok := firstReturn.(protoreflect.ProtoMessage); ok {
				req := reflect.New(method.Func.Type().In(2).Elem()).Interface()
				if msg, ok := req.(protoreflect.ProtoMessage); ok {
					logrus.Infof("GRpcRegister grpc client: %s", method.Name)
					methodName := common.GetHandleFunctionName(msg)
					th.h[methodName] = method.Func
				}
			}
		}
	}
}
func (th *Handler) Handle(method string, v []reflect.Value) ([]reflect.Value, error) {
	if h, ok := th.h[method]; ok {
		return h.Call(v), nil
	}
	return nil, fmt.Errorf("grpc lost method,%s", method)
}
