# ppx-app
基于 hashicorp raft 的一个高并发高可用的去中心化逻辑框架

go install git.shiyou.kingsoft.com/WANGXU13/ppx-app/cmd/protoc-gen-ppx
protoc *.proto  --go_out ./ --go-grpc_out=./ --ppx_out=./