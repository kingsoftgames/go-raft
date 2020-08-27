package test

import (
	"testing"

	"git.shiyou.kingsoft.com/infra/go-raft/common"
)

func Test_tryJoin(t *testing.T) {
	genTestClusterAppGrpcYaml()
	followerYaml = genYamlBase(followerYaml, false, 1, true, func(configure *common.Configure) {
		configure.JoinAddr = "127.0.0.1:1831"
	})
	clusterAppTemplate(t, func() {
		t.Logf("Test_tryJoin")
	})
}
