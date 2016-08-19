package main

import (
	"github.com/NebulousLabs/Sia/api"
)

// A JobRunner is used to start up jobs on the running Sia node.
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
