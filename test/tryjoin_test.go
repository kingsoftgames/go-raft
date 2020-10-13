package test

import (
	"io/ioutil"
	"os"
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

func Test_JoinFile(t *testing.T) {
	filename := "cache/join_addr.txt"
	content := "127.0.0.1:18330\n127.0.0.1:18331"
	ioutil.WriteFile(filename, []byte(content), os.ModePerm)
	leaderYaml = genYamlBase(leaderYaml, true, 0, true, func(configure *common.Configure) {
		configure.JoinFile = ""
	})
	followerYaml = genYamlBase(followerYaml, false, 1, true, func(configure *common.Configure) {
		configure.JoinFile = filename
	})
	clusterAppTemplate(t, func() {
		t.Logf("Test_JoinFile Success")
	})
}
