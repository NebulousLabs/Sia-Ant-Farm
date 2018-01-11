package ant

import (
	"fmt"

	"github.com/NebulousLabs/Sia/node/api"
	"github.com/NebulousLabs/Sia/sync"
)

// A jobRunner is used to start up jobs on the running Sia node.
type jobRunner struct {
	client         *api.Client
	walletPassword string
	siaDirectory   string
	tg             sync.ThreadGroup
}

// newJobRunner creates a new job runner, using the provided api address,
// authentication password, and sia directory.  It expects the connected api to
// be newly initialized, and initializes a new wallet, for usage in the jobs.
// siadirectory is used in logging to identify the job runner.
func newJobRunner(apiaddr string, authpassword string, siadirectory string) (*jobRunner, error) {
	jr := &jobRunner{
		client:       api.NewClient(apiaddr, authpassword),
		siaDirectory: siadirectory,
	}
	var walletParams api.WalletInitPOST
	err := jr.client.Post("/wallet/init", "", &walletParams)
	if err != nil {
		return nil, err
	}
	jr.walletPassword = walletParams.PrimarySeed

	err = jr.client.Post("/wallet/unlock", fmt.Sprintf("encryptionpassword=%s&dictionary=%s", jr.walletPassword, "english"), nil)
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
