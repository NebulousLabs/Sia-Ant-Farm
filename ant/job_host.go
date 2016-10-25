package ant

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/modules"
	"github.com/NebulousLabs/Sia/types"
)

// keep track of failed contracts to suppress error logging from repeated
// contract failures
var (
	rejectedContracts map[types.FileContractID]modules.HostContract
	failedContracts   map[types.FileContractID]modules.HostContract
	expiredContracts  map[types.FileContractID]modules.HostContract
)

// checkContracts verifies that the API at client has no host contract
// obligation failures, using the experimental /x/ Contracts endpoint. Returns
// nil if there are no contract errors, otherwise returns an error describing
// the failure.
func checkContracts(client *api.Client) error {
	var hostContracts api.HostXcontractsGET
	if err := client.Get("/host/xcontracts", &hostContracts); err != nil {
		return err
	}
	var csg api.ConsensusGET
	if err := client.Get("/consensus", &csg); err != nil {
		return err
	}
	for _, contract := range hostContracts.Contracts {
		if contract.ContractFailed {
			if _, ok := failedContracts[contract.ID]; !ok {
				failedContracts[contract.ID] = contract
				return fmt.Errorf("host has failed contract: %v\n", contract)
			}
		}
		if contract.ContractRejected {
			if _, ok := rejectedContracts[contract.ID]; !ok {
				rejectedContracts[contract.ID] = contract
				return fmt.Errorf("host has rejected contract: %v\n", contract)
			}
		}
		if contract.WindowEndHeight < csg.Height && !contract.ContractSucceeded {
			if _, ok := expiredContracts[contract.ID]; !ok {
				expiredContracts[contract.ID] = contract
				return fmt.Errorf("blockheight has surpassed contract end height but did not succeed: %v\n", contract)
			}
		}
	}
	return nil
}

// jobHost unlocks the wallet, mines some currency, and starts a host offering
// storage to the ant farm.
func (j *jobRunner) jobHost() {
	j.tg.Add()
	defer j.tg.Done()

	rejectedContracts = make(map[types.FileContractID]modules.HostContract)
	expiredContracts = make(map[types.FileContractID]modules.HostContract)
	failedContracts = make(map[types.FileContractID]modules.HostContract)

	// Mine at least 50,000 SC
	desiredbalance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
	success := false
	for start := time.Now(); time.Since(start) < 5*time.Minute; time.Sleep(time.Second) {
		var walletInfo api.WalletGET
		err := j.client.Get("/wallet", &walletInfo)
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
	hostdir, err := ioutil.TempDir("", "hostdata")
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}
	defer os.RemoveAll(hostdir)

	// Add the storage folder.
	err = j.client.Post("/host/storage/folders/add", fmt.Sprintf("path=%s&size=30000000000", hostdir), nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// Announce the host to the network, retrying up to 5 times before reporting
	// failure and returning.
	success = false
	for try := 0; try < 5; try++ {
		err = j.client.Post("/host/announce", "", nil)
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
	err = j.client.Post("/host", "acceptingcontracts=true", nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// Poll the API for host settings, logging them out with `INFO` tags.  If
	// `StorageRevenue` decreases, log an ERROR. Check the contract status once
	// per iteration.
	maxRevenue := types.NewCurrency64(0)
	for {
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second * 15):
		}

		var hostInfo api.HostGET
		err = j.client.Get("/host", &hostInfo)
		if err != nil {
			log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		}

		// Print an error if there's a problem with the Host's contracts
		if err = checkContracts(j.client); err != nil {
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
