package store

type RaftConfigure struct {
	NodeId           string `yaml:"node_id" json:"node_id" help:"raft node name" default:"n"`
	Addr             string `yaml:"addr" json:"addr" help:"addr for raft" default:"127.0.0.1:18300"`
	StoreInMem       bool   `yaml:"store_in_mem" json:"store_in_mem" help:"store in mem" default:"true"`                        //是否落地，false落地，true不落地
	StoreDir         string `yaml:"store_dir" json:"store_dir" help:"store director" default:"./"`                              //如果StoreInMem为true，这个参数无效
	LogCacheCapacity int    `yaml:"log_cache_capacity" json:"log_cache_capacity" help:"raft store cache in memory" default:"0"` //如果大于0，那么logStore使用 LogStoreCache
	Bootstrap        bool   `yaml:"bootstrap" json:"bootstrap" help:"node start as bootstrap" default:"true"`
	BootstrapExpect  int    `yaml:"bootstrap_expect" json:"bootstrap_expect" help:"node expect ,only nodeNum>=bootstrap_expect raft will begin vote"`
	RunChanNum       int    `yaml:"run_chan_num" json:"run_chan_num" help:"logic chan process raft apply" default:"100"`
	RaftApplyHash    bool   `yaml:"raft_apply_hash" json:"raft_apply_hash" help:"if true,open process logic chan"`
	KeyExpireS       int    `yaml:"key_expire_s" json:"key_expire_s" help:"key auto expire seconds"`
}
