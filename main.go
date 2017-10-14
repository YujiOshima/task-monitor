package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"log"
	"net/http"
	"sync"
	"time"
)

var watchPodMetricsList = map[string]string{
	"CPU":      "container_cpu_usage_seconds_total",
	"MEMORY":   "container_memory_working_set_bytes",
	"NW Res":   "container_network_receive_bytes_total",
	"NW Trans": "container_network_transmit_bytes_total",
}

var watchGPUMetricsList = map[string]string{
	"GPU PERCENT": "nvml_gpu_percent",
	"GPU MEMORY":  "nvml_memory_used",
	"GPU WATTS":   "nvml_power_watts",
}
var watchNodeMetricsList = make(map[string]string)
var gPUPodBinding = make(map[string]string)

var (
	promAddress  string
	taskName     string
	pollInterval int
)

type TaskWatcher struct {
	taskName  string
	promCL    promv1.API
	StopCh    chan bool
	mux       *sync.Mutex
	apiRouter *mux.Router
}

func NewTaskWatcher(name string, promAddr string) (*TaskWatcher, error) {
	promCL, err := promapi.NewClient(promapi.Config{
		Address: fmt.Sprintf("http://%s", promAddr),
	})
	if err != nil {
		return nil, err
	}
	t := &TaskWatcher{
		taskName:  name,
		promCL:    promv1.NewAPI(promCL),
		StopCh:    make(chan bool),
		mux:       new(sync.Mutex),
		apiRouter: mux.NewRouter().StrictSlash(true),
	}
	t.apiRouter.HandleFunc("/", t.index)
	t.apiRouter.HandleFunc("/api/metrics/gpu/{UUID}/{pod}", t.addGPUPodBinding).Methods("POST")
	return t, nil
}

func (tw *TaskWatcher) TaskName() string {
	return tw.taskName
}

func (tw *TaskWatcher) AddNewPodEMtrics(key string, value string) error {
	defer tw.mux.Unlock()
	tw.mux.Lock()
	watchPodMetricsList[key] = value
	return nil
}

func (tw *TaskWatcher) AddNewNodeEMtrics(key string, value string) error {
	defer tw.mux.Unlock()
	tw.mux.Lock()
	watchNodeMetricsList[key] = value
	return nil
}
func (tw *TaskWatcher) AddNewGPUEMtrics(key string, value string) error {
	defer tw.mux.Unlock()
	tw.mux.Lock()
	watchNodeMetricsList[key] = value
	return nil
}

func (tw *TaskWatcher) getPodMetrics(list map[string]string, curTime time.Time) {
	for k, v := range list {
		q := "sum(rate(" + v + "{pod_name=~\"^" + tw.taskName + ".*\"}[1m]))by(pod_name)"
		ret, err := tw.promCL.Query(context.Background(), q, curTime)
		if err != nil {
			log.Printf(k+"\nError %v\n", err)
		} else {
			log.Printf(k+"\n%s\n", ret.String())
		}
	}
}

func (tw *TaskWatcher) getNodeMetrics(list map[string]string, curTime time.Time) {
	for k, v := range list {
		q := v + "{}"
		ret, err := tw.promCL.Query(context.Background(), q, curTime)
		if err != nil {
			log.Printf(k+"\nError %v\n", err)
		} else {
			log.Printf(k+"\n%s\n", ret.String())
		}
	}
}

func (tw *TaskWatcher) getGPUMetrics(list map[string]string, binding map[string]string, curTime time.Time) {
	for k, v := range list {
		for u, p := range binding {
			q := v + "{device_uuid=~\"^" + u + "\"}"
			log.Print(q)
			ret, err := tw.promCL.Query(context.Background(), q, curTime)
			if err != nil {
				log.Printf(k+" "+p+"\nError %v\n", err)
			} else {
				log.Printf(k+" in Pod "+p+" : %s\n", ret.String())
			}
		}
	}
}

func (tw *TaskWatcher) watch() {
	for {
		curTime := time.Now()
		tw.mux.Lock()
		if len(watchPodMetricsList) > 0 {
			log.Printf("AddNewPodEMtrics : %v\n", watchPodMetricsList)
			tw.getPodMetrics(watchPodMetricsList, curTime)
		}
		if len(watchNodeMetricsList) > 0 {
			tw.getNodeMetrics(watchNodeMetricsList, curTime)
		}
		if len(watchGPUMetricsList) > 0 && len(gPUPodBinding) > 0 {
			tw.getGPUMetrics(watchGPUMetricsList, gPUPodBinding, curTime)
		}
		tw.mux.Unlock()
		<-time.After(time.Duration(pollInterval) * time.Second)
	}
}
func (tw *TaskWatcher) Start() error {
	go tw.watch()
	err := http.ListenAndServe(":18080", tw.apiRouter)
	log.Fatal(err)
	return err
}

func (tw *TaskWatcher) index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Task Watcher")
}

func (tw *TaskWatcher) addGPUPodBinding(w http.ResponseWriter, r *http.Request) {
	defer tw.mux.Unlock()
	vars := mux.Vars(r)
	log.Printf("vars %v\n", vars)
	log.Printf("GPU UUID %s bind to Pod %s\n", vars["UUID"], vars["pod"])
	tw.mux.Lock()
	gPUPodBinding[vars["UUID"]] = vars["pod"]
	fmt.Fprintln(w, "OK")
}

func init() {
	flag.StringVar(&promAddress, "addr", "localhost:9090", "Prometheus server address")
	flag.StringVar(&taskName, "task", "tfrun", "Task Name")
	flag.IntVar(&pollInterval, "poll", 1, "Monitor Polling Interval")
	flag.Parse()
}

func main() {
	t, err := NewTaskWatcher(taskName, promAddress)
	if err != nil {
		log.Fatalf("Fail init task watcher : %v\n", err)
	}
	t.Start()
}
