package common

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type AlerterGRpcConfigure struct {
	Addr string `yaml:"addr" json:"addr" help:"addr for grpc"`
}
type AlerterHttpConfigure struct {
	Url string `yaml:"url" json:"url" default:"http://localhost:13001/alerter" help:"url for http post"`
}
type AlerterConfigure struct {
	Panic bool                 `yaml:"panic" json:"panic" help:"if set true,process will exit when panic" default:"true"`
	Type  string               `yaml:"type" json:"type" default:"http" help:"notify type"`
	Http  AlerterHttpConfigure `yaml:"http" json:"http"`
	GRpc  AlerterGRpcConfigure `yaml:"grpc" json:"grpc"`
}

type Alerter struct {
	config *AlerterConfigure
}

func NewAlerter(config *AlerterConfigure) *Alerter {
	return &Alerter{
		config: config,
	}
}

type AlerterMsg struct {
	Type string
	Msg  string
}

func (th *Alerter) Notify(typ, msg string) error {
	defer func() {
		if th.config.Panic {
			panic(msg)
		}
	}()
	if th.config.Type == "http" {
		b, _ := json.Marshal(AlerterMsg{
			Type: typ,
			Msg:  msg,
		})
		_, err := http.Post(th.config.Http.Url, "application/json", bytes.NewBuffer(b))
		return err
	}
	return nil
}
