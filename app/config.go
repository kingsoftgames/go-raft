package app

import (
	"git.shiyou.kingsoft.com/infra/go-raft/common"
	"git.shiyou.kingsoft.com/infra/go-raft/store"
)

type DebugConfigure struct {
	TraceLine       bool `yaml:"trace_line" json:"trace_line" help:"if true,open future trace line log"`
	PrintIntervalMs int  `yaml:"print_interval_ms" json:"print_interval_ms" help:"print debug log interval ms"`
	GRpcHandleHash  bool `yaml:"grpc_handle_hash" json:"grpc_handle_hash" help:""`
}

type Configure struct {
	common.YamlType
	GrpcApiAddr           string                      `yaml:"grpc_addr" json:"grpc_addr" help:"addr for grpc api" default:":18310"`
	HttpApiAddr           string                      `yaml:"http_addr" json:"http_addr" help:"addr for http api" default:":18320"`
	InnerAddr             string                      `yaml:"inner_addr" json:"inner_addr" help:"addr for inner grpc" default:":18330"`
	Codec                 string                      `yaml:"codec" json:"codec" help:"codec type json/msgpack" default:"msgpack"`
	PortShift             int                         `yaml:"port_shift" json:"port_shift" help:""`
	JoinAddr              string                      `yaml:"join_addr" json:"join_addr" help:"addr for raft cluster"`
	TryJoinTime           int                         `yaml:"try_join_time" json:"try_join_time" help:"join max times" default:"3"`
	JoinFile              string                      `yaml:"join_file" json:"join_file" help:"join file,for nomad service"`
	ConnectTimeoutMs      int                         `yaml:"connect_timeout_ms" json:"connect_timeout_ms" help:"connect timeout ms" default:"100"`
	Ver                   string                      `yaml:"ver" json:"ver" help:"version" default:"v1.3"`
	HealthCheckIntervalMs int                         `yaml:"health_check_interval_ms" json:"health_check_interval_ms" help:"heath check interval" default:"200"`
	CleanDeadServerS      int                         `yaml:"cleanup_dead_server_s" json:"cleanup_dead_server_s" help:"if >0 ,remove node from raft cluster when heath check timeout over cleanup_dead_server_s second"`
	RunChanNum            int                         `yaml:"run_chan_num" json:"run_chan_num" help:"logic chan process raft apply" default:"100"`
	Raft                  *store.RaftConfigure        `yaml:"raft" json:"raft"`
	Log                   *common.LogConfigure        `yaml:"log" json:"log"`
	Debug                 *DebugConfigure             `yaml:"debug" json:"debug"`
	Prometheus            *common.PrometheusConfigure `yaml:"prometheus" json:"prometheus"`
	Alerter               *common.AlerterConfigure    `yaml:"alerter" json:"alerter"`
}

func (c Configure) Help() string {
	return "base on hashicorp/raft logic framework"
}
