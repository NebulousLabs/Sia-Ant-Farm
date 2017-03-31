package ant

import (
	"fmt"
	"log"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

var (
	spendInterval  = time.Second * 30
	spendThreshold = types.NewCurrency64(5e4).Mul(types.SiacoinPrecision)
)

func (j *jobRunner) bigSpender() {
	j.tg.Add()
	defer j.tg.Done()

	for {
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(spendInterval):
		}

		var walletGet api.WalletGET
		if err := j.client.Get("/wallet", &walletGet); err != nil {
			log.Printf("[%v jobSpender ERROR]: %v\n", j.siaDirectory, err)
			return
		}

		if walletGet.ConfirmedSiacoinBalance.Cmp(spendThreshold) < 0 {
			continue
		}

		log.Printf("[%v jobSpender INFO]: sending a large transaction\n", j.siaDirectory)

		voidaddress := types.UnlockHash{}
		err := j.client.Post("/wallet/siacoins", fmt.Sprintf("amount=%v&destination=%v", spendThreshold, voidaddress), nil)
		if err != nil {
			log.Printf("[%v jobSpender ERROR]: %v\n", j.siaDirectory, err)
			continue
		}

		log.Printf("[%v jobSpender INFO]: large transaction send successful\n", j.siaDirectory)
	}
}
