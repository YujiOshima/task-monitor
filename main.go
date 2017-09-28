package main

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"time"
)

func main() {
	cl, err := api.NewClient(api.Config{
		Address: "http://10.110.5.196:9090/api/v1/series",
	})
	promapi := v1.NewAPI(cl)
	curTime := time.Now()
	q := "match[]=nvml_power_watts"
	ret, err := promapi.Query(context.Background(), q, curTime)
	if err != nil {
		fmt.Printf("Error%v", err)
	}
	fmt.Printf("Prom Resp :%v\n", ret)

}
