package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeovninformer "github.com/alauda/kube-ovn/pkg/client/informers/externalversions"
	"github.com/alauda/kube-ovn/pkg/daemon"
	"github.com/alauda/kube-ovn/pkg/ovs"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/klog"
	"k8s.io/sample-controller/pkg/signals"
)

func main() {
	defer klog.Flush()
	go gc()

	config, err := daemon.ParseFlags()
	if err != nil {
		klog.Fatalf("parse config failed %v", err)
	}

	if config.EnableMirror {
		if err = daemon.InitMirror(config); err != nil {
			klog.Fatalf("failed to init mirror nic, %v", err)
		}
	}

	if err = daemon.InitNodeGateway(config); err != nil {
		klog.Fatalf("init node gateway failed %v", err)
	}

	stopCh := signals.SetupSignalHandler()
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(config.KubeClient, 0,
		kubeinformers.WithTweakListOptions(func(listOption *v1.ListOptions) {
			listOption.FieldSelector = fmt.Sprintf("spec.nodeName=%s", config.NodeName)
			listOption.AllowWatchBookmarks = true
		}))
	kubeovnInformerFactory := kubeovninformer.NewSharedInformerFactoryWithOptions(config.KubeOvnClient, 0)
	ctl, err := daemon.NewController(config, kubeInformerFactory, kubeovnInformerFactory)
	if err != nil {
		klog.Fatalf("create controller failed %v", err)
	}
	kubeInformerFactory.Start(stopCh)
	kubeovnInformerFactory.Start(stopCh)
	go ctl.Run(stopCh)
	go func() {
		klog.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%d", config.PprofPort), nil))
	}()
	daemon.RunServer(config)
}

func gc() {
	for {
		ovs.CleanLostInterface()
		time.Sleep(60 * time.Second)
	}
}
