package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	"git.shiyou.kingsoft.com/wangxu13/ppx-app/inner"
	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

type httpApi struct {
	engine  *gin.Engine
	h       IHttpHandler
	mainApp *MainApp
}

func newHttpApi(mainApp *MainApp, h IHttpHandler) *httpApi {
	return &httpApi{
		engine:  gin.New(),
		mainApp: mainApp,
		h:       h,
	}
}
func decodeJsonFromRead(read io.Reader, v interface{}) error {
	return json.NewDecoder(read).Decode(v)
}
func (th *httpApi) init(addr string) {
	th.h.OnHttpRegister()
	//heartbeat for elb
	th.engine.GET("/health", func(context *gin.Context) {
		context.String(200, "OK")
	})
	for path := range th.h.GetHandleMap() {
		th.Post(path)
	}
	go func() {
		if e := th.engine.Run(addr); e != nil {
			logrus.Fatalf("http run failed,%s,%s", addr, e.Error())
		}
	}()
}

type ErrResponse struct {
	Message string `json:"message"`
}

func newErr(err string) *ErrResponse {
	return &ErrResponse{
		Message: err,
	}
}

const sContentType = "application/json"

func (th *httpApi) Post(path string) {
	th.engine.POST(path, func(ctx *gin.Context) {
		contentType := ctx.GetHeader("content-type")
		if contentType != sContentType {
			ctx.JSON(403, newErr("content-type must be application/json"))
			return
		}
		if hd := th.h.get(path); hd == nil {
			ctx.JSON(403, newErr("wrong path"))
			return
		} else {
			leader := func() {
				var c context.Context = ctx
				if hash := ctx.Request.Header.Get("hash"); len(hash) > 0 {
					c = context.WithValue(context.Background(), "hash", hash)
				}
				if err, rsp := th.handle(c, hd, ctx.Request.Body); err != nil {
					logrus.Errorf("handle err,%s,%s", path, err.Error())
				} else {
					ctx.JSON(200, rsp)
				}
			}
			if th.mainApp.service.IsLeader() { //leader逻辑处理
				leader()
			} else if th.mainApp.service.IsFollower() { //非leader
				if readOnly := ctx.Request.Header.Get("readOnly"); readOnly == "true" {
					leader()
					return
				}
				req := &inner.TransHttpReq{
					Path: path,
					Data: make([]byte, ctx.Request.ContentLength),
				}
				if _, e := ctx.Request.Body.Read(req.Data); e != nil && e != io.EOF {
					ctx.JSON(403, newErr("read err "+e.Error()))
					return
				}
				if hash := ctx.Request.Header.Get("hash"); len(hash) > 0 {
					req.Hash = hash
				}
				c, _ := context.WithTimeout(ctx, 5*time.Second)
				rsp, e := th.mainApp.service.GetInner().TransHttpRequest(c, req)
				if e == nil {
					ctx.Data(200, sContentType, rsp.Data)
				} else {
					ctx.JSON(403, newErr("transfer leader err "+e.Error()))
				}
			} else {
				ctx.JSON(403, newErr("invalid node Candidate"))
			}
		}

	})
}

//这个肯定是leader
func (th *httpApi) call(ctx context.Context, path string, data []byte) ([]byte, error) {
	hd := th.h.get(path)
	if hd == nil {
		return nil, fmt.Errorf("wrong path")
	}
	if err, rsp := th.handle(ctx, hd, bytes.NewBuffer(data)); err != nil {
		logrus.Errorf("handle err,%s,%s", path, err.Error())
		return nil, err
	} else {
		return json.Marshal(rsp)
	}
}
func (th *httpApi) handle(ctx context.Context, hd *handler, data io.Reader) (error, interface{}) {
	req := reflect.New(hd.paramReq)
	rsp := reflect.New(hd.paramRsp)
	rtv := &HandlerRtv{}
	if e := decodeJsonFromRead(data, req.Interface()); e != nil {
		return fmt.Errorf("Json Decode err : " + e.Error()), nil
	}
	future := NewHttpReplyFuture()
	h := func() {
		hd.fn.Call([]reflect.Value{req, rsp, reflect.ValueOf(rtv)})
		if rtv.Futures.Len() != 0 {
			if err := rtv.Futures.Error(); err != nil {
				logrus.Errorf("Future err,%s", err.Error())
			}
		}
		future.Done()
	}
	//th.mainApp.runChan <- h
	handleContext(&th.mainApp.runLogic, ctx, h)
	future.Wait()
	return nil, rsp.Interface()
}

type handler struct {
	paramReq reflect.Type
	paramRsp reflect.Type
	fn       reflect.Value
}
type IHttpHandler interface {
	GetHandleMap() map[string]*handler
	OnHttpRegister()
	get(path string) *handler
}
type BaseHttpHandler struct {
	h map[string]*handler
}

func (th *BaseHttpHandler) GetHandleMap() map[string]*handler {
	return th.h
}
func (th *BaseHttpHandler) Put(path string, fn interface{}) {
	if th.h == nil {
		th.h = map[string]*handler{}
	}
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		logrus.Fatalf("HttpHandle Need Function Type,%s", fnType.Name())
	}
	h := &handler{
		fn:       reflect.ValueOf(fn),
		paramReq: fnType.In(0).Elem(),
		paramRsp: fnType.In(1).Elem(),
	}
	th.h[path] = h
}
func (th *BaseHttpHandler) get(path string) *handler {
	return th.h[path]
}
