package main

import (
	"fmt"
	"log"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

// blockMining unlocks the wallet and mines some currency.  If more than 100
// seconds passes before the wallet has received some amount of currency, this
// job will print an error.
func (j *JobRunner) blockMining() {
	j.tg.Add()
	defer j.tg.Done()

	err := j.client.Post("/wallet/unlock", fmt.Sprintf("encryptionpassword=%s&dictionary=%s", j.walletPassword, "english"), nil)
	if err != nil {
		log.Printf("[%v blockMining ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	err = j.client.Get("/miner/start", nil)
	if err != nil {
		log.Printf("[%v blockMining ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	for {
		var walletInfo api.WalletGET
		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			log.Printf("[%v blockMining ERROR]: %v\n", j.siaDirectory, err)
			return
		}
		initialBalance := walletInfo.ConfirmedSiacoinBalance
		// allow 100 seconds for mined funds to appear in the miner's wallet
		success := false
		for start := time.Now(); time.Since(start) < 100*time.Second; {
			select {
			case <-j.tg.StopChan():
				return
			case <-time.After(time.Second):
			}

			err = j.client.Get("/wallet", &walletInfo)
			if err != nil {
				log.Printf("[%v blockMining ERROR]: %v\n", j.siaDirectory, err)
			}
			if walletInfo.ConfirmedSiacoinBalance.Cmp(initialBalance) > 0 {
				// We have mined a block and now have money, continue
				success = true
				break
			}
		}
		if !success {
			log.Printf("[%v blockMining ERROR]: it took too long to receive new funds in miner job\n", j.siaDirectory)
		} else {
			log.Printf("[%v SUCCESS] blockMining job succeeded", j.siaDirectory)
		}
	}
}
