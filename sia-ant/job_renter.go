package main

import (
	"fmt"
	"log"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

// jobStorageRenter unlocks the wallet, mines some currency, sets an allowance
// using that currency, and uploads some files.  It will periodically try to
// download those files, printing any errors that occur.
func (j *JobRunner) jobStorageRenter() error {
	err := j.client.Post("/wallet/unlock", fmt.Sprintf("encryptionpassword=%s&dictionary=%s", j.walletPassword, "english"), nil)
	if err != nil {
		log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
		return err
	}

	err = j.client.Get("/miner/start", nil)
	if err != nil {
		log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
		return err
	}

	// Mine at least 50,000 SC
	desiredbalance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
	balance := types.NewCurrency64(0)
	for {
		var walletInfo api.WalletGET
		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
			return err
		}
		if balance.Cmp(desiredbalance) > 0 {
			break
		}
		balance = walletInfo.ConfirmedSiacoinBalance
		time.Sleep(time.Second)
	}

	err = j.client.Post("/renter", fmt.Sprintf("funds=%v&period=1000", balance.Div64(2)), nil)
	if err != nil {
		log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
		return err
	}

	// TODO: file upload, download, verification

	return nil
}
