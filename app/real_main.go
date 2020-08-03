package app

import (
	"flag"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

var appsNameFlag *string
var configFile *string

type RealMain struct {
	apps []*MainApp
}

var realMain RealMain

func (th *RealMain) run() {
	if !flag.Parsed() {
		flag.Parse()
	}
	appV := make([]IApp, 0)
	var configV []string = make([]string, 0)
	configV = append(configV, *configFile)
	if len(*appsNameFlag) == 0 {
		appV = createAllApp()
	} else {
		appSlice := strings.Split(*appsNameFlag, ",")
		configV = strings.Split(*configFile, ",")
		if len(appSlice) != len(configV) {
			logrus.Fatalf("need config file equal app num ")
		}
		for _, a := range appSlice {
			app := CreateApp(a)
			if app == nil {
				logrus.Fatalf("not found app %s", a)
			}
			appV = append(appV, app)
		}
	}
	var exitWait sync.WaitGroup
	for i, app := range appV {
		mainApp := NewMainApp(app, &exitWait)
		if rst := mainApp.Init(configV[i]); rst != 0 {
			logrus.Fatalf("Init Failed %s,%d", app, rst)
		}
		go func(a *MainApp) {
			a.Start()
		}(mainApp)
	}
	exitWait.Wait()
}
func RunMain() {
	realMain.run()
}
func init() {
	appsNameFlag = flag.String("apps", "", "run app name ,if null ,run all which register")
	configFile = flag.String("config", "", "config file path")

}