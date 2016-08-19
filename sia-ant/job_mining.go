package main

import (
	"fmt"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

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
		time.Sleep(time.Second)
	}
	j.errorlog <- "blockMining job succeeded"
}
