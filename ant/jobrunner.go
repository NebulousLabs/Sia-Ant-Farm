package ant

import (
	"github.com/NebulousLabs/Sia/node/api/client"
	"github.com/NebulousLabs/Sia/sync"
)

// A jobRunner is used to start up jobs on the running Sia node.
type jobRunner struct {
	client         *client.Client
	walletPassword string
	siaDirectory   string
	tg             sync.ThreadGroup
}

// newJobRunner creates a new job runner, using the provided api address,
// authentication password, and sia directory.  It expects the connected api to
// be newly initialized, and initializes a new wallet, for usage in the jobs.
// siadirectory is used in logging to identify the job runner.
func newJobRunner(apiaddr string, authpassword string, siadirectory string) (*jobRunner, error) {
	client := client.New(apiaddr)
	client.Password = authpassword
	jr := &jobRunner{
		client:       client,
		siaDirectory: siadirectory,
	}
	walletParams, err := jr.client.WalletInitPost("", false)
	if err != nil {
		return nil, err
	}
	jr.walletPassword = walletParams.PrimarySeed

	err = jr.client.WalletUnlockPost(jr.walletPassword)
	if err != nil {
		return nil, err
	}

	return jr, nil
}

// Stop signals all running jobs to stop and blocks until the jobs have
// finished stopping.
func (j *jobRunner) Stop() {
	j.tg.Stop()
}
