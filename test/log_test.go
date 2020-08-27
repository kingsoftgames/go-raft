package test

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func Test_Log(t *testing.T) {
	genTestSingleYaml()
	singleAppTemplate(t, func() {
		for i := 0; i < 10; i++ {
			logrus.Infof("test info log %d", i)
			logrus.Errorf("test err log %d", i)
		}
	})
}
