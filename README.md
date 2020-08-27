# ppx-app
基于 hashicorp/raft 一致性去中心化逻辑框架

go install git.shiyou.kingsoft.com/infra/go-raft/cmd/protoc-gen-ppx
protoc *.proto  --go_out ./ --go-grpc_out=./ --ppx_out=./