package app

import (
	"sync/atomic"

	"git.shiyou.kingsoft.com/infra/go-raft/common"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	subsystem = "raft"
)

type raftCollector struct {
	app       *MainApp
	namespace string
}

func (th *raftCollector) Update(ch chan<- prometheus.Metric) error {
	ch <- prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   th.namespace,
		Subsystem:   subsystem,
		Name:        "size",
		Help:        "Raft KV Size",
		ConstLabels: nil,
	}, func() float64 {
		return float64(th.app.GetStore().GetMSize())
	})
	ch <- prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   th.namespace,
		Subsystem:   subsystem,
		Name:        "index",
		Help:        "Raft LastIndex",
		ConstLabels: nil,
	}, func() float64 {
		return float64(th.app.GetStore().GetRaft().LastIndex())
	})
	ch <- prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   th.namespace,
		Subsystem:   subsystem,
		Name:        "query",
		Help:        "Total query counts elapsed time",
		ConstLabels: nil,
	}, func() float64 {
		cnt := atomic.LoadUint64(&th.app.queryCnt)
		atomic.StoreUint64(&th.app.queryCnt, 0)
		return float64(cnt)
	})
	return nil
}
func newRaftCollector(host interface{}) (common.Collector, error) {
	app := host.(*MainApp)
	namespace := app.getNS()
	return &raftCollector{
		app:       app,
		namespace: namespace,
	}, nil
}

func init() {
	common.RegisterCollector(subsystem, newRaftCollector)
}
