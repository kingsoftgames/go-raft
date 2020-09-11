package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
)

var count = 500
var timeout = 50000 * time.Millisecond

func Test_markSingleAppGRpc(t *testing.T) {
	leaderYaml = genYamlBase(leaderYaml, true, 0, true, func(configure *common.Configure) {
	})
	singleAppTemplate(t, func() {
		c := newClient("127.0.0.1:18310")
		var w sync.WaitGroup

		for i := 0; i < count; i++ {
			w.Add(1)
			go func() {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				runClient(t, c, "set", key, 1, true)
				runClient(t, c, "get", key, 1, true)
				runClient(t, c, "del", key, 1, true)
				runClient(t, c, "get", key, 1, true)
			}()
		}
		w.Wait()
	})
}
func Test_markSingleAppHttp(t *testing.T) {
	genTestSingleYaml()
	singleAppTemplate(t, func() {
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func() {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				httpClient(t, "http://127.0.0.1:18320", "/test.SetReq", key, 1, true)
				httpClient(t, "http://127.0.0.1:18320", "/test.GetReq", key, 1, true)
				httpClient(t, "http://127.0.0.1:18320", "/test.DelReq", key, 1, true)
				httpClient(t, "http://127.0.0.1:18320", "/test.GetReq", key, 1, true)
			}()
		}
		w.Wait()
	})
}
func Test_ClusterAppGRpc(t *testing.T) {
	common.OpenDebugLog()
	genTestClusterAppGrpcYaml()
	clusterAppTemplate(t, func() {
		c := []*testGRpcClient{newClient("127.0.0.1:18310"), newClient("127.0.0.1:18311")}
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				//runClient(t, c[idx%len(c)], "get", key, 1, true)
				//runClient(t, c[idx%len(c)], "get", key, 1, true)
				runClient(t, c[idx%len(c)], "get", key, 1, true)
				runClient(t, c[idx%len(c)], "get", key, 1, true)

				runClient(t, c[idx%len(c)], "set", key, 1, true)
				runClient(t, c[idx%len(c)], "set", key, 1, true)
				//runClient(t, c[idx%len(c)], "set", key, 1, false)
				//runClient(t, c[idx%len(c)], "set", key, 1, false)
			}(i)
		}
		w.Wait()
	})
}

func Test_ClusterAppHttp(t *testing.T) {
	genTestClusterAppGrpcYaml()
	clusterAppTemplate(t, func() {
		addr := []string{"http://127.0.0.1:18320", "http://127.0.0.1:18321"}
		var w sync.WaitGroup
		w.Add(count)
		for i := 0; i < count; i++ {
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				httpClient(t, addr[idx%len(addr)], "/test.GetReq", key, 1, true)
				httpClient(t, addr[idx%len(addr)], "/test.SetReq", key, 1, true)
				httpClient(t, addr[idx%len(addr)], "/test.GetReq", key, 1, true)
				httpClient(t, addr[idx%len(addr)], "/test.DelRdq", key, 1, true)
			}(i)
		}
		w.Wait()
	})
}

func httpClient2(t *testing.T, idx int, doneWait *sync.WaitGroup, addr, path string, key string, writeTimes int, hash bool) {
	logrus.Infof("httpClient2,%d", idx)
	doneWait.Add(1)
	defer func() {
		doneWait.Done()
		logrus.Infof("doneWait.Done(),%d", idx)
	}()
	var req interface{}
	switch path {
	case "/test.GetReq":
		req = &GetReq{
			A: key,
		}
	case "/test.SetReq":
		A := make(map[string]*TestUnit, 0)
		for i := 0; i < writeTimes; i++ {
			A[fmt.Sprintf("%s_%d", key, i)] = newTestUnit()
		}
		req = &SetReq{
			A: A,
		}
	case "/test.DelReq":
		req = &DelReq{
			A: key,
		}
	}
	b, _ := json.Marshal(req)
	//rsp, err := http.DefaultClient.Post(addr+path, "application/json", bytes.NewReader(b))
	if !hash {
		key = ""
	}
	rsp, err := httpPost(key, addr+path, "application/json", bytes.NewReader(b))
	if err != nil {
		logrus.Infof("http rsp err %d,%s", idx, err.Error())
	} else {
		bb := make([]byte, 4096)
		rsp.Body.Read(bb)
		//t.Logf("http rsp : %s", string(bb))
	}
}
func Test_ClusterApp2GRpc(t *testing.T) {
	genTestClusterApp2GrpcYaml()
	clusterApp2Template(t, func() {
		c := []*testGRpcClient{newClient("127.0.0.1:18310"), newClient("127.0.0.1:18311"), newClient("127.0.0.1:18312")}
		var w sync.WaitGroup

		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				runClient(t, c[idx%len(c)], "set", key, 1, false)
				runClient(t, c[idx%len(c)], "get", key, 1, false)
				runClient(t, c[idx%len(c)], "del", key, 1, false)
				runClient(t, c[idx%len(c)], "get", key, 1, false)
			}(i)
		}
		w.Wait()
	})
}

func Test_ClusterApp2Http(t *testing.T) {
	genTestClusterApp2GrpcYaml()
	clusterApp2Template(t, func() {
		addr := []string{"http://127.0.0.1:18320", "http://127.0.0.1:18321", "http://127.0.0.1:18322"}
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				httpClient(t, addr[idx%len(addr)], "/test.SetReq", key, 1, false)
				httpClient(t, addr[idx%len(addr)], "/test.GetReq", key, 1, false)
				httpClient(t, addr[idx%len(addr)], "/test.DelReq", key, 1, false)
				httpClient(t, addr[idx%len(addr)], "/test.GetReq", key, 1, false)
			}(i)
		}
		w.Wait()
	})
}

type testGRpcClient struct {
	c *app.GRpcClient
}

func (th *testGRpcClient) Get() *testClient {
	return th.c.Get().(*testClient)
}

func newClient(addr string) *testGRpcClient {
	c := app.NewGRpcClient(2000, NewTestClient)
	if err := c.Connect(addr, ""); err != nil {
		log.Fatalf("connect failed,%s", err.Error())
	}
	return &testGRpcClient{
		c: c,
	}
}
func newTestUnit() *TestUnit {
	return &TestUnit{
		A: "123",
		B: 1,
		C: 2,
		D: "321",
	}
}
func httpPost(hash string, url, contentType string, body io.Reader) (resp *http.Response, err error) {
	c := http.DefaultClient
	ctx := context.WithValue(context.Background(), "hash", hash)
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	if len(hash) > 0 {
		req.Header.Set("hash", hash)
	}
	return c.Do(req)
}

func httpClient(t *testing.T, addr, path string, key string, writeTimes int, hash bool) {
	var req interface{}
	switch path {
	case "/test.GetReq":
		req = &GetReq{
			A: key,
		}
	case "/test.SetReq":
		A := make(map[string]*TestUnit, 0)
		for i := 0; i < writeTimes; i++ {
			A[fmt.Sprintf("%s_%d", key, i)] = newTestUnit()
		}
		req = &SetReq{
			A: A,
		}
	case "/test.DelReq":
		req = &DelReq{
			A: key,
		}
	}
	b, _ := json.Marshal(req)
	//rsp, err := http.DefaultClient.Post(addr+path, "application/json", bytes.NewReader(b))
	if !hash {
		key = ""
	}
	rsp, err := httpPost(key, addr+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Logf("http rsp err ,%s", err.Error())
	} else {
		bb := make([]byte, 4096)
		rsp.Body.Read(bb)
		t.Logf("http rsp : %s", string(bb))
	}
}

func runClient(t *testing.T, c *testGRpcClient, cmd string, key string, writeTimes int, hash bool) {
	header := &Header{}
	if hash {
		header.Hash = key
	}
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	switch cmd {
	case "get":
		_, e := c.Get().GetRequest(ctx, &GetReq{
			Header: header,
			A:      key,
			B:      0,
			C:      0,
		})
		if e != nil {
			t.Logf("send err ,%s", e.Error())
		} else {
			//t.Logf("getrsp")
		}
	case "set":
		A := make(map[string]*TestUnit, 0)
		for i := 0; i < writeTimes; i++ {
			A[fmt.Sprintf("%s_%d", key, i)] = newTestUnit()
		}
		_, e := c.Get().SetRequest(ctx, &SetReq{
			Header: header,
			A:      A,
		})
		if e != nil {
			t.Logf("send err ,%s", e.Error())
		} else {
			//t.Logf("setrsp, %v", *rsp)
		}
	case "del":
		_, e := c.Get().DelRequest(ctx, &DelReq{
			Header: header,
			A:      key,
		})
		if e != nil {
			t.Logf("send err ,%s", e.Error())
		} else {
			//t.Logf("delrsp, %v", *rsp)
		}
	default:
	}
}
func init() {
	rand.Seed(time.Now().UnixNano())
}
