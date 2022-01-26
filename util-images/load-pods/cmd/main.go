/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	goflag "flag"
	"fmt"
	"time"

	flag "github.com/spf13/pflag"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/kubernetes"
	flowcontrol "k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog"
)

var (
	object       = flag.String("object", "", "Object for which load will be generated.")
	qps          = flag.Float64("qps", 0.5, "QPS")
	responseType = flag.String("response-type", "application/json", "Response type from api-server")
	namespace    = flag.String("namespace", "", "namespace where objects to be listed live")
)

func main() {
	initFlagsAndKlog()

	config, err := newConfig()
	if err != nil {
		klog.Fatal(err.Error())
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err.Error())
	}

	if !validateResponseType(*responseType) {
		klog.Fatal("Response type is not supported")
	}

	klog.Info("Starting worker\n")
	worker(client, *object, *namespace, *responseType, *qps)
}

func initFlagsAndKlog() {
	klogFlags := goflag.NewFlagSet("klog", goflag.ExitOnError)
	klog.InitFlags(klogFlags)
	flag.CommandLine.AddGoFlagSet(klogFlags)
	flag.Parse()
}

func validateResponseType(responseType string) bool {
	acceptedTypes := []string{"application/json", "application/yaml", "application/vnd.kubernetes.protobuf"}
	for _, t := range acceptedTypes {
		if responseType == t {
			return true
		}
	}
	return false
}

func makeRequest(c kubernetes.Interface, url, responseType string) {
	klog.V(4).Infof("Worker sends request\n")

	r := c.CoreV1().RESTClient().Get().RequestURI(url).SetHeader("Accept", responseType).Do(context.Background())
	_, err := r.Get()
	if err != nil {
		klog.Warningf("Got error when requesting \"%s\": %v", url, err)
	} else {
		klog.V(4).Infof("Succesfuly fetched resources for \"%s\"", url)
	}
}

func worker(client kubernetes.Interface, object, namespace, responeType string, qps float64) {
	duration := time.Duration(float64(int64(time.Second)) / qps)
	ticker := time.NewTicker(duration)
	fmt.Println(object)
	url := fmt.Sprintf("api/v1/namespaces/%s/%s", namespace, object)
	for {
		go makeRequest(client, url, responeType)
		<-ticker.C
	}
}

func newConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	config.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
	return config, nil
}
