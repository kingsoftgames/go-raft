package app

import (
	"flag"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/go-yaml/yaml"
)

type logConfigure struct {
	Level string `yaml:"Level"`
}

func newDefaultLogConfigure() logConfigure {
	return logConfigure{}
}

type configure struct {
	RaftAddr         string       `yaml:"RaftAddr"`
	GrpcApiAddr      string       `yaml:"GrpcApiAddr"`
	HttpApiAddr      string       `yaml:"HttpApiAddr"`
	StoreInMem       bool         `yaml:"StoreInMem"` //是否落地，false落地，true不落地
	StoreDir         string       `yaml:"StoreDir"`   //如果StoreInMem为true，这个参数无效
	Codec            string       `yaml:"Codec"`
	LogConfig        logConfigure `yaml:"LogConfig"`
	PortShift        int          `yaml:"PortShift"`
	NodeId           string       `yaml:"NodeId"`
	JoinAddr         string       `yaml:"JoinAddr"`
	ConnectTimeoutMs int          `yaml:"ConnectTimeoutMs"` //连接超时（毫秒）
}

func newDefaultConfigure() *configure {
	config := &configure{
		RaftAddr:         "127.0.0.1:18300",
		GrpcApiAddr:      "127.0.0.1:18310",
		HttpApiAddr:      "127.0.0.1:18320",
		StoreDir:         "./",
		StoreInMem:       true,
		Codec:            "msgpack",
		LogConfig:        newDefaultLogConfigure(),
		PortShift:        0,
		NodeId:           "",
		JoinAddr:         "",
		ConnectTimeoutMs: 5 * 1000,
	}
	trimConfigureFromFlag(config)
	trimConfigByShift(config)
	return config
}
func trimConfigByShift(config *configure) {
	v := reflect.ValueOf(config).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		n := v.Type().Field(i).Name
		if strings.HasSuffix(n, "Addr") {
			ss := strings.Split(f.String(), ":")
			if len(ss) == 2 {
				p, _ := strconv.Atoi(ss[1])
				f.SetString(ss[0] + ":" + strconv.Itoa(p+config.PortShift))
			}
		}
	}
}
func initConfigure(content []byte) (config *configure) {
	config = newDefaultConfigure()
	if err := yaml.Unmarshal(content, config); err != nil {
		logrus.Fatal("initConfigure yaml.Unmarshal err,%s", err.Error())
	}
	return config
}
func initConfigureFromFile(file string) *configure {
	if len(file) == 0 {
		return newDefaultConfigure()
	}
	if b, err := ioutil.ReadFile(file); err == nil {
		return initConfigure(b)
	} else {
		logrus.Fatalf("initConfigureFromFile readFile err,%s", err.Error())
	}
	return nil
}

var raftAddr *string
var gRpcApiAddr *string
var httpApiAddr *string
var storeDir *string
var storeInMem *bool
var codeC *string
var portShift *int
var nodeId *string
var joinAddr *string
var connectTimeoutMs *int

func trimConfigureFromFlag(config *configure) {
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "raft_addr":
			config.RaftAddr = *raftAddr
		case "grpc_addr":
			config.GrpcApiAddr = *gRpcApiAddr
		case "http_addr":
			config.HttpApiAddr = *httpApiAddr
		case "store_dir":
			config.StoreDir = *storeDir
		case "store_mem":
			config.StoreInMem = *storeInMem
		case "codec":
			config.Codec = *codeC
		case "port_shift":
			config.PortShift = *portShift
		case "node_id":
			config.NodeId = *nodeId
		case "join_addr":
			config.JoinAddr = *joinAddr
		case "con_timeout_ms":
			config.ConnectTimeoutMs = *connectTimeoutMs
		}
	})
}

func init() {
	raftAddr = flag.String("raft_addr", "", "addr for raft")
	gRpcApiAddr = flag.String("grpc_addr", "", "addr for grpc")
	httpApiAddr = flag.String("http_addr", "", "addr for http")
	storeDir = flag.String("store_dir", "", "raft store directory")
	storeInMem = flag.Bool("store_mem", true, "raft store in memory")
	codeC = flag.String("codec", "", "codec json/msgpack")
	portShift = flag.Int("port_shift", 0, "port shift")
	nodeId = flag.String("node_id", "", "nodeId for raft")
	joinAddr = flag.String("join_addr", "", "addr for join raft leader node")
	connectTimeoutMs = flag.Int("con_timeout_ms", 0, "timeout ms for connect")

}
