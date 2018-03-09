package ant

import (
	"log"
	"time"

	"github.com/NebulousLabs/Sia/types"
)

var (
	sendInterval = time.Second * 2
	sendAmount   = types.NewCurrency64(1000).Mul(types.SiacoinPrecision)
)

func (j *jobRunner) littleSupplier(sendAddress types.UnlockHash) {
	j.tg.Add()
	defer j.tg.Done()

	for {
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(sendInterval):
		}

		walletGet, err := j.client.WalletGet()
		if err != nil {
			log.Printf("[%v jobSpender ERROR]: %v\n", j.siaDirectory, err)
			return
		}

		if walletGet.ConfirmedSiacoinBalance.Cmp(sendAmount) < 0 {
			continue
		}

		_, err = j.client.WalletSiacoinsPost(sendAmount, sendAddress)
		if err != nil {
			log.Printf("[%v jobSupplier ERROR]: %v\n", j.siaDirectory, err)
		}
	}
}
