# go-raft
基于 hashicorp/raft 一致性去中心化逻辑框架

gen raft code from proto
```
go install git.shiyou.kingsoft.com/infra/go-raft/cmd/protoc-gen-go-raft
protoc *.proto  --go_out ./ --go-grpc_out=./ --go-raft_out=./
```
