package app

import (
	"os"
	"os/signal"
	"reflect"
	"strings"
	"sync"

	"gopkg.in/alecthomas/kingpin.v2"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"github.com/sirupsen/logrus"
)

type RealMain struct {
	apps []*MainApp
}

var realMain RealMain

// ./team team,match team --cfg=
func (th *RealMain) run(help string) {
	flagApp := kingpin.New(os.Args[0], help)
	appFlags := flagApp.Arg("apps", "").Required().Strings()
	config := flagApp.Flag("config", "path of config,if config is a dir,will use default app Name as config file").Default("./").ExistingFileOrDir()
	if cmd, err := flagApp.Parse(os.Args[1:]); err != nil {
		logrus.Fatal("flag parse err,%s", err.Error())
	} else {
		logrus.Infof("run %s", cmd)
	}
	logrus.Infof("RealMain.Run")
	appV := make([]IApp, 0)
	if len(*appFlags) == 0 {
		appV = createAllApp()
	} else {
		for _, a := range *appFlags {
			app := CreateApp(a)
			if app == nil {
				logrus.Fatalf("not found app %s", a)
			}
			appV = append(appV, app)
		}
	}
	var w sync.WaitGroup
	for _, app := range appV {
		w.Add(1)
		go func(app IApp) {
			defer func() {
				w.Done()
			}()
			var exitWait common.GracefulExit
			mainApp := NewMainApp(app, &exitWait)
			logrus.Infof("Run %s", mainApp.getNS())
			defer logrus.Infof("Exit %s", mainApp.getNS())
			if err := mainApp.Init(*config); err != nil {
				logrus.Fatalf("Init Failed %s,%s", reflect.TypeOf(app).String(), err.Error())
			}
			mainApp.Start()
			s := make(chan os.Signal, 1)
			signal.Notify(s, os.Kill, os.Interrupt)
			go func(mainApp *MainApp) {
				select {
				case _s := <-s:
					logrus.Infof("recv %s", _s.String())
					mainApp.Stop()
				}
			}(mainApp)
			exitWait.Wait()
		}(app)

	}
	w.Done()
}
func RunMain(help string) {
	defer func() {
		logrus.Info("RunMain exit")
	}()
	//realMain.run(help)
	appV := createAllApp()
	if len(appV) == 0 {
		logrus.Fatal("need register app")
	}
	if len(appV) > 1 {
		logrus.Fatal("only support register one app")
	}
	app := appV[0]
	var exitWait common.GracefulExit
	mainApp := NewMainApp(app, &exitWait)
	logrus.Infof("Run %s", mainApp.getNS())
	defer logrus.Infof("Exit %s", mainApp.getNS())
	flagApp := kingpin.New(os.Args[0], help)
	config := flagApp.Flag("config", "config file path").String()
	appConfig := app.Config()
	var appConfigFile *string
	if appConfig != nil {
		appConfigFile = flagApp.Flag(mainApp.getNS()+"_config", "sub app config file path(if needed)").String()
	}
	if err := mainApp.InitFlag(flagApp); err != nil {
		logrus.Fatalf("InitFlag err %s", err.Error())
	}
	if _, err := flagApp.Parse(os.Args[1:]); err != nil {
		flagApp.Usage(os.Args[1:])
		logrus.Fatalf("FlagParse err %s", err.Error())
	}
	ffts := make([]common.FileFft, 0)
	ffts = append(ffts, common.NewFileFft(*config, &mainApp.config, ""))
	if appConfig != nil {
		ffts = append(ffts, common.NewFileFft(*appConfigFile, appConfig, mainApp.getNS()))
	}
	args := make([]string, 0)
	for _, arg := range os.Args[1:] {
		//剔除
		if strings.Contains(arg, "--config") || strings.Contains(arg, "--"+mainApp.getNS()+"_config") {
			continue
		}
		args = append(args, arg)
	}
	if err := common.ReadFileAndParseFlagWithArgsSlice(ffts, args); err != nil {
		logrus.Fatalf("ReadFileAndParseFlagWithArgsSlice err %s", err.Error())
	}

	if err := mainApp.Init(""); err != nil {
		logrus.Fatalf("Init Failed %s,%s", reflect.TypeOf(app).String(), err.Error())
	}
	mainApp.Start()
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Kill, os.Interrupt)
	go func(mainApp *MainApp) {
		select {
		case _s := <-s:
			logrus.Infof("recv %s", _s.String())
			mainApp.Stop()
		}
	}(mainApp)
	exitWait.Wait()
}
func RunMainAll(help string) {
	defer func() {
		logrus.Infof("RunMainAll exit")
	}()
	logrus.Infof("RunMainAll.Run")
}
func init() {
	logrus.SetOutput(os.Stdout)
}
