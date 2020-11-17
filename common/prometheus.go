package common

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/common/version"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const metricsPath = "/metrics"

var (
	collectorFactory = make(map[string]func(host interface{}) (Collector, error))
)

type PromCollector struct {
	CheckWork
	collectors         map[string]Collector
	register           *prometheus.Registry
	scrapeDurationDesc *prometheus.Desc
	scrapeSuccessDesc  *prometheus.Desc
	server             *http.Server
}

func (th *PromCollector) Describe(cn chan<- *prometheus.Desc) {
	cn <- th.scrapeDurationDesc
	cn <- th.scrapeSuccessDesc
}
func (th *PromCollector) Collect(ch chan<- prometheus.Metric) {
	var w sync.WaitGroup
	for name, c := range th.collectors {
		w.Add(1)
		go func(name string, c Collector) {
			th.execute(name, c, ch)
			w.Done()
		}(name, c)
	}
	w.Wait()
}
func (th *PromCollector) Init(host interface{}, goFunc GoFunc, config *PrometheusConfigure) error {
	if len(config.Addr) == 0 {
		return nil
	}
	if th.collectors != nil {
		logrus.Errorf("prometheus collector has inited")
		return nil
	}
	th.scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, "scrape", "collector_duration_seconds"),
		config.Namespace+"_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	th.scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, "scrape", "collector_success"),
		config.Namespace+"_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
	th.register = prometheus.NewRegistry()
	th.register.MustRegister(version.NewCollector("cmn_exporter"))
	th.collectors = map[string]Collector{}
	for name, f := range collectorFactory {
		c, err := f(host)
		if err != nil {
			return fmt.Errorf("prometheus factory %s err %s", name, err.Error())
		}
		th.collectors[name] = c
	}
	if config.IncludeExporterMetrics {
		th.register.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{
			Namespace: config.Namespace,
		}))
		th.register.MustRegister(prometheus.NewGoCollector())
	}
	th.register.MustRegister(th)
	http.Handle(metricsPath, promhttp.InstrumentMetricHandler(
		th.register, promhttp.HandlerFor(th.register, promhttp.HandlerOpts{}),
	))
	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		if th.Check() {
			writer.WriteHeader(200)
		} else {
			writer.WriteHeader(403)
		}
	})
	goFunc.Go(func() {
		th.server = &http.Server{Addr: config.Addr, Handler: nil}
		logrus.Infof("Prometheus http listen %s", config.Addr)
		if err := th.server.ListenAndServe(); err != nil {
			logrus.Errorf("prometheus listen err %s", err.Error())
		}
	})
	return nil
}
func (th *PromCollector) Stop() {
	if th.server != nil {
		th.server.Close()
	}
}

var ErrNoData = errors.New("collector returned no data")

func (th *PromCollector) execute(name string, c Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64
	if err != nil {
		if err == ErrNoData {
			logrus.Error("msg", "collector returned no data", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		} else {
			logrus.Error("msg", "collector failed", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		}
		success = 0
	} else {
		logrus.Debug("msg", "collector succeeded", "name", name, "duration_seconds", duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(th.scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(th.scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

// Collector is the interface a collector has to implement.
type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Update(ch chan<- prometheus.Metric) error
}

func RegisterCollector(collector string, factory func(interface{}) (Collector, error)) {
	logrus.Infof("Collector Reg: %s", collector)
	collectorFactory[collector] = factory
}
