package ant

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/NebulousLabs/Sia/modules"
	"github.com/NebulousLabs/Sia/types"
)

// jobHost unlocks the wallet, mines some currency, and starts a host offering
// storage to the ant farm.
func (j *jobRunner) jobHost() {
	j.tg.Add()
	defer j.tg.Done()

	// Mine at least 50,000 SC
	desiredbalance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
	success := false
	for start := time.Now(); time.Since(start) < 5*time.Minute; time.Sleep(time.Second) {
		walletInfo, err := j.client.WalletGet()
		if err != nil {
			log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
			return
		}
		if walletInfo.ConfirmedSiacoinBalance.Cmp(desiredbalance) > 0 {
			success = true
			break
		}
	}
	if !success {
		log.Printf("[%v jobHost ERROR]: timeout: could not mine enough currency after 5 minutes\n", j.siaDirectory)
		return
	}

	// Create a temporary folder for hosting
	hostdir, _ := filepath.Abs(filepath.Join(j.siaDirectory, "hostdata"))
	os.MkdirAll(hostdir, 0700)

	// Add the storage folder.
	size := modules.SectorSize * 4096
	err := j.client.HostStorageFoldersAddPost(hostdir, size)
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// Announce the host to the network, retrying up to 5 times before reporting
	// failure and returning.
	success = false
	for try := 0; try < 5; try++ {
		err = j.client.HostAnnouncePost()
		if err != nil {
			log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		} else {
			success = true
			break
		}
		time.Sleep(time.Second * 5)
	}
	if !success {
		log.Printf("[%v jobHost ERROR]: could not announce after 5 tries.\n", j.siaDirectory)
		return
	}
	log.Printf("[%v jobHost INFO]: succesfully performed host announcement\n", j.siaDirectory)

	// Accept contracts
	err = j.client.HostAcceptingContractsPost(true)
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// Poll the API for host settings, logging them out with `INFO` tags.  If
	// `StorageRevenue` decreases, log an ERROR.
	maxRevenue := types.NewCurrency64(0)
	for {
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second * 15):
		}

		hostInfo, err := j.client.HostGet()
		if err != nil {
			log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		}

		// Print an error if storage revenue has decreased
		if hostInfo.FinancialMetrics.StorageRevenue.Cmp(maxRevenue) >= 0 {
			maxRevenue = hostInfo.FinancialMetrics.StorageRevenue
		} else {
			// Storage revenue has decreased!
			log.Printf("[%v jobHost ERROR]: StorageRevenue decreased!  was %v is now %v\n", j.siaDirectory, maxRevenue, hostInfo.FinancialMetrics.StorageRevenue)
		}
	}
}
