package ant

import (
	"log"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

// blockMining unlocks the wallet and mines some currency.  If more than 100
// seconds passes before the wallet has received some amount of currency, this
// job will print an error.
func (j *jobRunner) blockMining(desiredBalance types.Currency) {
	j.tg.Add()
	defer j.tg.Done()

	minerRunning := true
	err := j.client.Get("/miner/start", nil)
	if err != nil {
		log.Printf("[%v blockMining ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// Every 100 seconds, verify that the balance has increased.
	// if the balance is above desiredBalance, throttle the miner.
	// if desiredBalance is zero, the miner runs forever.
	runForever := desiredBalance.Cmp(types.ZeroCurrency) == 0

	for {
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second * 100):
		}

		if runForever {
			continue
		}

		var walletInfo api.WalletGET
		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			log.Printf("[%v blockMining ERROR]: %v\n", j.siaDirectory, err)
			return
		}

		haveDesiredBalance := walletInfo.ConfirmedSiacoinBalance.Cmp(desiredBalance) > 0
		if !minerRunning && !haveDesiredBalance {
			log.Printf("[%v miner INFO]: not enough currency, starting the miner\n", j.siaDirectory)
			minerRunning = true
			if err = j.client.Get("/miner/start", nil); err != nil {
				log.Printf("[%v miner ERROR]: %v\n", j.siaDirectory, err)
				return
			}
		} else if minerRunning && haveDesiredBalance {
			log.Printf("[%v miner INFO]: mined enough currency, stopping the miner\n", j.siaDirectory)
			minerRunning = false
			if err = j.client.Get("/miner/stop", nil); err != nil {
				log.Printf("[%v miner ERROR]: %v\n", j.siaDirectory, err)
				return
			}
		}
	}
}
