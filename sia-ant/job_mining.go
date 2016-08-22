package main

import (
	"fmt"
	"log"
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
		log.Printf("[blockMining ERROR]: %v\n", err)
		return
	}

	err = j.client.Get("/miner/start", nil)
	if err != nil {
		log.Printf("[blockMining ERROR]: %v\n", err)
		return
	}

	// Mine a block and wait for the confirmed funds to appear in the wallet.
	success := false
	for start := time.Now(); time.Since(start) < 100*time.Second; time.Sleep(time.Second) {
		var walletInfo api.WalletGET
		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			log.Printf("[blockMining ERROR]: %v\n", err)
			return
		}
		if walletInfo.ConfirmedSiacoinBalance.Cmp(types.ZeroCurrency) > 0 {
			// We have mined a block and now have money, continue
			success = true
			break
		}
	}
	if !success {
		log.Print("[blockMining ERROR]: it took too long to mine a block to use in blockMining")
	} else {
		log.Print("[SUCCESS] blockMining job succeeded")
	}
}
