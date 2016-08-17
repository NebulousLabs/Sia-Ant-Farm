package main

import (
	"fmt"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

type JobRunner struct {
	client         *api.Client
	errorlog       chan interface{}
	walletPassword string
}

// NewJobRunner creates a new job runner, using the provided api address and
// authentication password.  It expects the connected api to be newly
// initialized, and initializes a new wallet, for usage in the jobs.
func NewJobRunner(apiaddr string, authpassword string) (*JobRunner, error) {
	jr := &JobRunner{
		errorlog: make(chan interface{}),
		client:   api.NewClient(apiaddr, authpassword),
	}
	var walletParams api.WalletInitPOST
	err := jr.client.Post("/wallet/init", "", &walletParams)
	if err != nil {
		return nil, err
	}
	jr.walletPassword = walletParams.PrimarySeed
	return jr, nil
}

// gatewayConnectability will print an error to the log if the node has zero
// peers at any time.
func (j *JobRunner) gatewayConnectability() {
	for {
		time.Sleep(time.Second * 5)
		var info api.GatewayGET
		err := j.client.Get("/gateway", &info)
		if err != nil {
			j.errorlog <- fmt.Sprintf("Error in JobPeerConnectability: %v\n", err)
			return
		}
		if len(info.Peers) == 0 {
			j.errorlog <- "JobPeerConnectability: node has zero peers..."
		}
	}
}

// blockMining unlocks the wallet and mines some currency.  If more than 100
// seconds passes before the wallet has received some amount of currency, this
// job will print an error.
func (j *JobRunner) blockMining() {
	err := j.client.Post("/wallet/unlock", fmt.Sprintf("encryptionpassword=%s&dictionary=%s", j.walletPassword, "english"), nil)
	if err != nil {
		j.errorlog <- fmt.Sprintf("Error in renterContractFormation: %v\n", err)
		return
	}

	err = j.client.Get("/miner/start", nil)
	if err != nil {
		j.errorlog <- fmt.Sprintf("Error in renterContractFormation: %v\n", err)
		return
	}

	// Mine a block and wait for the confirmed funds to appear in the wallet.
	startTime := time.Now()
	for {
		if time.Since(startTime) > time.Second*100 {
			j.errorlog <- "it took too long to mine a block to use in renterContractFormation"
			return
		}
		var walletInfo api.WalletGET
		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			j.errorlog <- err
			return
		}
		if walletInfo.ConfirmedSiacoinBalance.Cmp(types.ZeroCurrency) > 0 {
			// We have mined a block and now have money, continue
			break
		}
	}
	j.errorlog <- "blockMining job succeeded"
}
