package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

// jobHost unlocks the wallet, mines some currency, and starts a host offering
// storage to the ant farm.
func (j *JobRunner) jobHost() {
	err := j.client.Post("/wallet/unlock", fmt.Sprintf("encryptionpassword=%s&dictionary=%s", j.walletPassword, "english"), nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR: %v\n", j.siaDirectory, err)
		return
	}

	err = j.client.Get("/miner/start", nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR: %v\n", j.siaDirectory, err)
		return
	}

	// Mine at least 50,000 SC
	desiredbalance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
	balance := types.NewCurrency64(0)
	for {
		var walletInfo api.WalletGET
		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
			return
		}
		if balance.Cmp(desiredbalance) > 0 {
			break
		}
		balance = walletInfo.ConfirmedSiacoinBalance
		time.Sleep(time.Second)
	}

	// Create a temporary folder for hosting
	hostdir, err := ioutil.TempDir("", "hostdata")
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}
	defer os.RemoveAll(hostdir)

	// For now, hard code some sane host settings for this job.  In the future,
	// this job can take args to determine these settings.
	err = j.client.Post("/host/storage/folders/add", fmt.Sprintf("path=%s&size=30000000000"), nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// Announce the host to the network
	err = j.client.Post("/host/announce", "", nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	maxRevenue := types.NewCurrency64(0)
	for {
		var hostInfo api.HostGET
		err = j.client.Get("/host", &hostInfo)
		if err != nil {
			log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
			return
		}
		log.Printf("[%v jobHost INFO]: %v", j.siaDirectory, hostInfo.NetworkMetrics)

		// Print an error if storage revenue has decreased
		if hostInfo.FinancialMetrics.StorageRevenue.Cmp(maxRevenue) > 0 {
			maxRevenue = hostInfo.FinancialMetrics.StorageRevenue
		} else {
			// Storage revenue has decreased!
			log.Printf("[%v jobHost ERROR]: StorageRevenue decreased!  was %v is now %v\n", j.siaDirectory, maxRevenue, hostInfo.FinancialMetrics.StorageRevenue)
		}

		time.Sleep(time.Second * 5)
	}
}
