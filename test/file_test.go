package test

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/app"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/common"
)

func Test_FileWatch(t *testing.T) {
	exit := make(chan struct{})
	file := "cache/filewatch"
	ioutil.WriteFile(file, []byte("init"), os.ModePerm)
	var watch common.FileWatch
	watch.Add(file, func(name string) {
		content, _ := ioutil.ReadFile(name)
		logrus.Infof("filewatch %s,%s", name, string(content))
		if string(content) == "exit" {
			watch.Stop()
			exit <- struct{}{}
		}
	})
	watch.Start()
	ioutil.WriteFile(file, []byte("begin"), os.ModePerm)
	time.Sleep(1 * time.Second)
	ioutil.WriteFile(file, []byte("exit"), os.ModePerm)
	<-exit
}

func Test_Help(t *testing.T) {
	app.RunMain()
}

func Test_Track(t *testing.T) {
	common.OpenDebugGracefulExit()
	t.Logf(common.GetStack(5))
}
