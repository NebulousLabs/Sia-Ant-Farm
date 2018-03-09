package ant

import (
	"fmt"
	"log"
	"time"

	"github.com/NebulousLabs/Sia/node/api"
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

		var walletGet api.WalletGET
		if err := j.client.Get("/wallet", &walletGet); err != nil {
			log.Printf("[%v jobSpender ERROR]: %v\n", j.siaDirectory, err)
			return
		}

		if walletGet.ConfirmedSiacoinBalance.Cmp(sendAmount) < 0 {
			continue
		}

		err := j.client.Post("/wallet/siacoins", fmt.Sprintf("amount=%v&destination=%v", sendAmount, sendAddress), nil)
		if err != nil {
			log.Printf("[%v jobSupplier ERROR]: %v\n", j.siaDirectory, err)
		}
	}
}
