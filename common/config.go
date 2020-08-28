package common

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type LogConfigure struct {
	Level    string `yaml:"level"`
	Path     string `yaml:"path"`
	MaxAge   int    `yaml:"max_age"`
	MaxSize  int    `yaml:"max_size"`
	Compress bool   `yaml:"compress"`
}

func NewDefaultLogConfigure() *LogConfigure {
	return &LogConfigure{
		Level:   "DEBUG",
		Path:    "./log",
		MaxSize: 100,
	}
}

type Configure struct {
	RaftAddr         string        `yaml:"raft_addr"`
	GrpcApiAddr      string        `yaml:"grpc_addr"`
	HttpApiAddr      string        `yaml:"http_addr"`
	StoreInMem       bool          `yaml:"store_in_mem"`       //是否落地，false落地，true不落地
	StoreDir         string        `yaml:"store_dir"`          //如果StoreInMem为true，这个参数无效
	LogCacheCapacity int           `yaml:"log_cache_capacity"` //如果大于0，那么logStore使用 LogStoreCache
	Codec            string        `yaml:"codec"`
	LogConfig        *LogConfigure `yaml:"log_config"`
	PortShift        int           `yaml:"port_shift"`
	NodeId           string        `yaml:"node_id"`
	JoinAddr         string        `yaml:"join_addr"`
	TryJoinTime      int           `yaml:"try_join_time"`
	JoinFile         string        `yaml:"join_file"`
	ConnectTimeoutMs int           `yaml:"connect_timeout_ms"` //连接超时（毫秒）
	Bootstrap        bool          `yaml:"bootstrap"`
	BootstrapExpect  int           `yaml:"bootstrap_expect"`
}

func NewDefaultConfigure() *Configure {
	config := &Configure{
		RaftAddr:         "127.0.0.1:18300",
		GrpcApiAddr:      "127.0.0.1:18310",
		HttpApiAddr:      "127.0.0.1:18320",
		StoreDir:         "./",
		StoreInMem:       true,
		Codec:            "msgpack",
		LogConfig:        NewDefaultLogConfigure(),
		PortShift:        0,
		NodeId:           "",
		JoinAddr:         "",
		JoinFile:         "",
		TryJoinTime:      3,
		ConnectTimeoutMs: 100,
		Bootstrap:        true,
		BootstrapExpect:  0,
	}
	trim(config)
	return config
}
func trimConfigByShift(config *Configure) {
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
func trimConfigJoinFile(config *Configure) {
	if len(config.JoinFile) > 0 {
		addr, err := ReadJoinAddr(config.JoinFile)
		if err != nil {
			logrus.Fatalf("trimConfigJoinFile,err,%s,%s", config.JoinFile, err.Error())
		}
		config.JoinAddr = addr
	}
}
func ReadJoinAddr(fileName string) (string, error) {
	addr := ""
	if b, err := ioutil.ReadFile(fileName); err != nil {
		return "", err
	} else {
		r := bufio.NewReader(bytes.NewBuffer(b))
		for line, _, _ := r.ReadLine(); line != nil; line, _, _ = r.ReadLine() {
			if len(addr) == 0 {
				addr = string(line)
			} else {
				addr = fmt.Sprintf("%s,%s", addr, string(line))
			}
		}
	}
	return addr, nil
}
func trim(config *Configure) {
	trimConfigureFromFlag(config)
	trimConfigByShift(config)
	//trimConfigJoinFile(config)
}
func InitConfigure(content []byte) (config *Configure) {
	config = NewDefaultConfigure()
	if err := yaml.Unmarshal(content, config); err != nil {
		logrus.Fatal("initConfigure yaml.Unmarshal err, ", err.Error())
	}
	trim(config)
	return config
}
func InitConfigureFromFile(file string) *Configure {
	if len(file) == 0 {
		return NewDefaultConfigure()
	}
	if b, err := ioutil.ReadFile(file); err == nil {
		return InitConfigure(b)
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
var tryJoinTime *int
var joinFile *string
var connectTimeoutMs *int
var bootstrap *bool
var bootstrapExpect *int
var logCacheCapacity *int

func trimConfigureFromFlag(config *Configure) {
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
		case "try_join_time":
			config.TryJoinTime = *tryJoinTime
		case "join_file":
			config.JoinFile = *joinFile
		case "con_timeout_ms":
			config.ConnectTimeoutMs = *connectTimeoutMs
		case "bootstrap":
			config.Bootstrap = *bootstrap
		case "bootstrap_expect":
			config.BootstrapExpect = *bootstrapExpect
		case "log_cache_capacity":
			config.LogCacheCapacity = *logCacheCapacity
		}
	})
}

func init() {
	raftAddr = flag.String("raft_addr", "", "addr for raft")
	gRpcApiAddr = flag.String("grpc_addr", "", "addr for grpc")
	httpApiAddr = flag.String("http_addr", "", "addr for http")
	storeDir = flag.String("store_dir", "", "raft store directory")
	storeInMem = flag.Bool("store_mem", true, "raft store in memory(default true)")
	codeC = flag.String("codec", "", "codec json/msgpack")
	portShift = flag.Int("port_shift", 0, "port shift")
	nodeId = flag.String("node_id", "", "nodeId for raft")
	joinAddr = flag.String("join_addr", "", "addr for join raft leader node")
	tryJoinTime = flag.Int("try_join_time", 0, "try join time")
	joinFile = flag.String("join_file", "", "if len(join_file)!=0 join addr provide from file name")
	connectTimeoutMs = flag.Int("con_timeout_ms", 0, "timeout ms for connect")
	bootstrap = flag.Bool("bootstrap", false, "start as bootstrap( as leader)")
	bootstrapExpect = flag.Int("bootstrap_expect", 0, "node num expect")
	logCacheCapacity = flag.Int("log_cache_capacity", 0, "if >0 use LogStoreCache as logStore for raft")
}
