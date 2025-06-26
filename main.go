/*
Copyright 2021 The KubeVela Authors.

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

// Package main is the entry point for the Terraform controller manager.
package main

import (
	"k8s.io/klog/v2/textlogger"
	"os"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/util/feature"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	terraformv1beta1 "github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = terraformv1beta1.AddToScheme(scheme)
	_ = v1beta2.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var syncPeriod time.Duration
	var namespace string
	var controllerNamespace string

	pflag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager, this will ensure there is only one active controller manager.")
	pflag.DurationVar(&syncPeriod, "informer-re-sync-interval", 10*time.Second, "controller shared informer lister full re-sync period")
	pflag.StringVar(&metricsAddr, "metrics-addr", ":38080", "The address the metric endpoint binds to.")
	pflag.StringVar(&namespace, "namespace", "", "Namespace to watch for resources, defaults to all namespaces")
	pflag.StringVar(&controllerNamespace, "controller-namespace", "", "Namespace to run the terraform jobs")
	feature.DefaultMutableFeatureGate.AddFlag(pflag.CommandLine)

	// embed klog
	klog.InitFlags(nil)
	pflag.Parse()

	cfg := textlogger.NewConfig()
	logger := textlogger.NewLogger(cfg)
	ctrl.SetLogger(logger)

	cacheOpts := ctrlcache.Options{
		SyncPeriod: &syncPeriod,
	}
	if namespace != "" {
		cacheOpts.DefaultNamespaces = map[string]ctrlcache.Config{
			namespace: {},
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "ce329a9c.core.oam.dev",
		Metrics: ctrlmetrics.Options{
			BindAddress: metricsAddr,
		},
		Scheme: scheme,
		Cache:  cacheOpts,
		WebhookServer: ctrlwebhook.NewServer(ctrlwebhook.Options{
			Port: 9443,
		}),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ConfigurationReconciler{
		Client:              mgr.GetClient(),
		ControllerNamespace: controllerNamespace,
		Log:                 ctrl.Log.WithName("controllers").WithName("Configuration"),
		Scheme:              mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Configuration")
		os.Exit(1)
	}
	if err = (&controllers.ProviderReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Provider"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Provider")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
