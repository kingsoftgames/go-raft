package test

import (
	"testing"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
	"github.com/hashicorp/raft"
)

func genTestClusterAppGrpcYamlFloor() {
	leaderYaml = genYaml(leaderYaml, true, 0, false)
	followerYaml = genYaml(followerYaml, false, 1, false)
	follower2Yaml = genYaml(follower2Yaml, false, 2, false)
}

func clusterAppRestart(t *testing.T) {
	var exitWait common.GracefulExit
	appLeader := app.NewMainApp(app.CreateApp("test"), &exitWait)
	if rst := appLeader.Init(leaderYaml); rst != 0 {
		t.Errorf("appLeader Init error,%d", rst)
		return
	}
	appLeader.GetStore().OnStateChg.Add(func(i interface{}) {
		if i.(raft.RaftState) == raft.Leader {
			appFollower := app.NewMainApp(app.CreateApp("test"), &exitWait)
			appFollower.OnLeaderChg.Add(func(i interface{}) {
				appFollower2 := app.NewMainApp(app.CreateApp("test"), &exitWait)
				appFollower2.OnLeaderChg.Add(func(i interface{}) {
					go func() {
						common.NewTimer(2*time.Second, func() {
							t.Logf("Stop follower2")
							appFollower2.Stop()
							appFollower2.Stopped()

							appFollower2 = app.NewMainApp(app.CreateApp("test"), &exitWait)
							appFollower2.OnLeaderChg.Add(func(i interface{}) {
								t.Logf("Test Ok")
							})
						})
						//appLeader.Stop()
						//appFollower.Stop()
						//appFollower2.Stop()
					}()
				})
				if rst := appFollower2.Init(follower2Yaml); rst != 0 {
					t.Errorf("appFollower2 Init error,%d", rst)
					appLeader.Stop()
					appFollower.Stop()
					return
				}
				appFollower2.Start()
			})
			if rst := appFollower.Init(followerYaml); rst != 0 {
				t.Errorf("appFollower Init error,%d", rst)
				appLeader.Stop()
				return
			}
			appFollower.Start()
		}
	})
	appLeader.Start()
	exitWait.Wait()
}

func Test_Floor(t *testing.T) {
	genTestClusterAppGrpcYamlFloor()
	clusterAppRestart(t)
}
