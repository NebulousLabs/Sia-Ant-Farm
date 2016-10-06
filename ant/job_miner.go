package ant

import (
	"fmt"
	"log"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

// blockMining unlocks the wallet and mines some currency.  If more than 100
// seconds passes before the wallet has received some amount of currency, this
// job will print an error.
func (j *jobRunner) blockMining() {
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

	var walletInfo api.WalletGET
	err = j.client.Get("/wallet", &walletInfo)
	if err != nil {
		log.Printf("[%v blockMining ERROR]: %v\n", j.siaDirectory, err)
		return
	}
	lastBalance := walletInfo.ConfirmedSiacoinBalance

	// Every 100 seconds, verify that the balance has increased.
	for {
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second * 100):
		}

		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			log.Printf("[%v blockMining ERROR]: %v\n", j.siaDirectory, err)
		}
		if walletInfo.ConfirmedSiacoinBalance.Cmp(lastBalance) > 0 {
			log.Printf("[%v SUCCESS] blockMining job succeeded", j.siaDirectory)
			lastBalance = walletInfo.ConfirmedSiacoinBalance
		} else {
			log.Printf("[%v blockMining ERROR]: it took too long to receive new funds in miner job\n", j.siaDirectory)
		}
	}
}
