package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/config"
	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/log"
	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/process"
	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/proxy"
	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/uitls/certificate"
	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/vault"
)

func main() {
	logger := log.New()
	ctx, cancelFn := context.WithCancel(context.Background())

	defer func() {
		r := recover()
		if r != nil {
			logger.Fatalf("Recovered: %+v", r)
		}
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		select {
		case <-signalCh:
			cancelFn()
		}
	}()

	start(ctx, logger)
}

func start(ctx context.Context, logger log.Logger) {
	logger.Info("Creating certificates CA")
	ca := certificate.NewCA()
	err := ca.Initialize()
	if err != nil {
		logger.WithError(err).Fatal("Error while creating certificates CA")
	}

	logger.Info("Creating configuration file")
	confFile, err := config.CreateFile(ca)
	if err != nil {
		logger.WithError(err).Fatal("Error while creating configuration file")
	}

	logger.Info("Creating Vault process")
	vaultProc, err := process.New(ctx, logger, confFile)
	if err != nil {
		logger.WithError(err).Fatal("Error while creating new Vault process")
	}

	terminateVaultProc := func() {
		err := vaultProc.Terminate()
		if err != nil {
			logger.WithError(err).Error("Error while terminating Vault process")
		}
	}

	defer func() {
		r := recover()
		if r != nil {
			logger.Errorf("Panic recovered, stopping Vault process: %+v", r)
			terminateVaultProc()

			panic(r)
		}
	}()

	logger.Info("Starting Vault process")
	waitCh := vaultProc.Start()

	logger.Info("Parsing Vault URL")
	vaultURL, err := vaultProc.URL()
	if err != nil {
		terminateVaultProc()
		logger.WithError(err).Fatal("Error while parsing Vault URL")
	}

	logger.Info("Initializing Vault")
	vaultInitializer := vault.NewInitializer(vaultURL, ca)
	details, err := vaultInitializer.Initialize()
	if err != nil {
		terminateVaultProc()
		logger.WithError(err).Fatal("Error while initializing Vault")
	}

	logger.Info("Initializing Vault proxy")
	prx, err := proxy.New(logger, details, ca)
	if err != nil {
		terminateVaultProc()
		logger.WithError(err).Fatal("Error while initializing Vault proxy")
	}

	logger.Info("Starting Vault proxy")
	prxListener, prxWaitCh := prx.Start()

	logger.WithField("address", prxListener.Addr().String()).Info("Vault proxy listening on")

	select {
	case err = <-waitCh:
		if err != nil {
			logger.WithError(err).Fatal("Error while executing Vault process")
		}
	case err = <-prxWaitCh:
		if err != nil {
			terminateVaultProc()
			logger.WithError(err).Fatal("Error while executing Vault proxy")
		}
	}
}
