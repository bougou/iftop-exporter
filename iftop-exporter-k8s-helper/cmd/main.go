/*
Copyright 2024 Bougou.

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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/bougou/iftop-exporter/iftop-exporter-k8s-helper/internal/controller"
	"github.com/bougou/iftop-exporter/iftop-exporter-k8s-helper/internal/utils"
	"github.com/kr/pretty"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

type selectorsFlag []string

func (i *selectorsFlag) String() string {
	return "list of selectors"
}

func (i *selectorsFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var rootfs string
	var namespaces string
	var selectors selectorsFlag
	var dynamicDir string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&rootfs, "rootfs", "/", "The path of rootfs")
	flag.StringVar(&namespaces, "namespaces", "", "The namespaces to watch")
	flag.Var(&selectors, "selectors", "list of selectors")
	flag.StringVar(&dynamicDir, "dynamic-dir", "/var/run/iftop-exporter/dynamic", "The iftop-exporter dynamic dir to store interface info.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	pretty.Println("selector flag:", selectors, len(selectors))

	ss, err := utils.ParseSelectors(selectors)
	if err != nil {
		fmt.Printf("parse selectors found\n")
		os.Exit(1)
	}
	pretty.Println(ss)
	if len(ss) == 0 {
		fmt.Printf("Error: zero selectors found\n")
		os.Exit(1)
	}

	// The loop would make sure the iftop-exporter-k8s-helper begin to execute its logic
	// until the iftop-exporter Manager starts successfully.
	// The iftop-exporter Manager would create the watching file when it starts successfully
	// which means it is ready to watch fsnotify events from the dynamicDir.
	for {
		watchingFile := filepath.Join(dynamicDir, ".watching")
		ok, err := checkFileExists(watchingFile)
		if err != nil {
			fmt.Printf("Error: check the existence of watching file failed, err: %s\n", err)
			os.Exit(1)
		}

		if !ok {
			fmt.Printf("Waiting for watching file\n")
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Printf("Found watching file, continue\n")
		break
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	var namespaceList []string
	for _, ns := range strings.Split(namespaces, ",") {
		_ns := strings.TrimSpace(ns)
		if _ns != "" {
			namespaceList = append(namespaceList, _ns)
		}
	}

	defaultNamespaces := make(map[string]cache.Config)
	if len(namespaceList) == 0 {
		// cluster-level
		defaultNamespaces = nil
	} else {
		for _, ns := range namespaceList {
			defaultNamespaces[ns] = cache.Config{}
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "7649dd71.bougou.cn",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,

		Cache: cache.Options{
			DefaultNamespaces: defaultNamespaces,
		},
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.PodReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		Rootfs:     rootfs,
		Selectors:  ss,
		DynamicDir: dynamicDir,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Pod")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func checkFileExists(item string) (bool, error) {
	info, err := os.Stat(item)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// item exists
	return !info.IsDir(), nil
}
