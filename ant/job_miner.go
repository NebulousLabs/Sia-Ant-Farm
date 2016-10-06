package ant

import (
	"log"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

// blockMining mines blocks until desiredBalance is reached. If desiredBalance
// is zero, blockMining will start the miner and return, mining indefinitely.
func (j *jobRunner) blockMining(desiredBalance types.Currency) {
	j.tg.Add()
	defer j.tg.Done()

	minerRunning := true
	err := j.client.Get("/miner/start", nil)
	if err != nil {
		log.Printf("[%v blockMining ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// If desiredBalance is zero, just return, leaving the miner running.
	runForever := desiredBalance.Cmp(types.ZeroCurrency) == 0
	if runForever {
		return
	}

	// Every 20 seconds, check if the balance has exceeded the desiredBalance. If
	// it has and the miner is running, the miner is throttled. If the desired
	// balance has not been reached and the miner is not running, the miner is
	// started.
	for {
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second * 20):
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
