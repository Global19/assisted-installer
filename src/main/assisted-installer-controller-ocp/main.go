package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/openshift/assisted-installer/src/k8s_client"
	"github.com/openshift/assisted-installer/src/utils"

	"github.com/kelseyhightower/envconfig"
	assistedinstallercontroller "github.com/openshift/assisted-installer/src/assisted_installer_controller"
	"github.com/openshift/assisted-installer/src/inventory_client"
	"github.com/openshift/assisted-installer/src/ops"
	"github.com/sirupsen/logrus"
)

var Options struct {
	ControllerConfig assistedinstallercontroller.ControllerConfig
}

func main() {
	logger := logrus.New()

	err := envconfig.Process("myapp", &Options)
	if err != nil {
		log.Fatal(err.Error())
	}

	kc, err := k8s_client.NewK8SClient("", logger)
	if err != nil {
		log.Fatalf("Failed to create k8 client %v", err)
	}

	logger.Infof("Start running assisted-controller with cluster-id %s, url %s",
		Options.ControllerConfig.ClusterID, Options.ControllerConfig.URL)

	err = kc.SetProxyEnvVars()
	if err != nil {
		log.Fatalf("Failed to set env vars for installer-controller pod %v", err)
	}

	client, err := inventory_client.CreateInventoryClient(Options.ControllerConfig.ClusterID,
		Options.ControllerConfig.URL, Options.ControllerConfig.PullSecretToken, Options.ControllerConfig.SkipCertVerification,
		Options.ControllerConfig.CACertPath, logger, utils.ProxyFromEnvVars)
	if err != nil {
		log.Fatalf("Failed to create inventory client %v", err)
	}

	assistedController := assistedinstallercontroller.NewController(logger,
		Options.ControllerConfig,
		ops.NewOps(logger, false),
		client,
		kc,
	)

	logger.Infof("Controller deployed on OCP cluster")

	var wg sync.WaitGroup
	var status assistedinstallercontroller.ControllerStatus
	var waitAndUpdateFunc = func() {
		assistedController.WaitAndUpdateNodesStatus(&status)
	}
	go approveCsrs(assistedController.ApproveCsrs, &wg)
	wg.Add(1)
	go waitAndUpdateNodesStatus(waitAndUpdateFunc)
	wg.Add(1)
	wg.Wait()
}

func waitAndUpdateNodesStatus(waitAndUpdateNodesStatusFunc func()) {
	for {
		waitAndUpdateNodesStatusFunc()
		time.Sleep(assistedinstallercontroller.GeneralWaitInterval)
	}
}

// Note: BMHs for day2 are currently not provided. Once added, CSRs approval can be skipped.
func approveCsrs(approveCsrsFunc func(context.Context, *sync.WaitGroup), wg *sync.WaitGroup) {
	// not cancelling approve to keep routine alive
	ctx := context.Background()
	approveCsrsFunc(ctx, wg)
}
