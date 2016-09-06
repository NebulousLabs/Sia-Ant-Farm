package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/crypto"
	"github.com/NebulousLabs/Sia/types"
)

// storageRenter unlocks the wallet, mines some currency, sets an allowance
// using that currency, and uploads some files.  It will periodically try to
// download those files, printing any errors that occur.
func (j *JobRunner) storageRenter() {
	j.tg.Add()
	defer j.tg.Done()

	err := j.client.Post("/wallet/unlock", fmt.Sprintf("encryptionpassword=%s&dictionary=%s", j.walletPassword, "english"), nil)
	if err != nil {
		log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	err = j.client.Get("/miner/start", nil)
	if err != nil {
		log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
		return
	}

	// Mine at least 100,000 SC
	desiredbalance := types.NewCurrency64(100000).Mul(types.SiacoinPrecision)
	success := false
	for start := time.Now(); time.Since(start) < 5*time.Minute; {
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second):
		}

		var walletInfo api.WalletGET
		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
			return
		}
		if walletInfo.ConfirmedSiacoinBalance.Cmp(desiredbalance) > 0 {
			success = true
			break
		}
	}
	if !success {
		log.Printf("[%v jobStorageRenter ERROR]: timeout: could not mine enough currency after 5 minutes\n", j.siaDirectory)
		return
	}

	// Set an initial 50ksc allowance
	allowance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
	if err := j.client.Post("/renter", fmt.Sprintf("funds=%v&period=100", allowance), nil); err != nil {
		log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
	}

	// Every 1000 seconds, set a new allowance.
	go func() {
		for {
			select {
			case <-j.tg.StopChan():
				return
			case <-time.After(time.Second * 1000):
			}
			func() {
				j.tg.Add()
				defer j.tg.Done()

				// set an allowance of 50k SC
				allowance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
				if err := j.client.Post("/renter", fmt.Sprintf("funds=%v&period=100", allowance), nil); err != nil {
					log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}
			}()
		}
	}()

	// Every 120 seconds, upload a 500MB file.  After ten files, delete one file
	// at random each iteration.
	go func() {
		var files []string

		// Clean up by deleting any created files when this goroutine returns.
		defer func() {
			for _, file := range files {
				os.Remove(file)
			}
		}()

		for i := 0; ; i++ {
			select {
			case <-j.tg.StopChan():
				return
			case <-time.After(time.Second * 120):
			}
			func() {
				j.tg.Add()
				defer j.tg.Done()

				if i >= 10 {
					randindex, err := crypto.RandIntn(len(files))
					if err != nil {
						log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
						return
					}
					if err = j.client.Post(fmt.Sprintf("/renter/delete/%v", files[randindex]), "", nil); err != nil {
						log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
					}
					files = append(files[:randindex], files[randindex+1:]...)
				}

				// Generate some random data to upload
				f, err := ioutil.TempFile("", "antfarm-renter")
				if err != nil {
					log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}

				_, err = io.CopyN(f, rand.Reader, 500e6)
				if err != nil {
					log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}

				// Upload the random data
				if err = j.client.Post(fmt.Sprintf("/renter/upload/%v", f.Name()), fmt.Sprintf("source=%v", f.Name()), nil); err != nil {
					log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}

				files = append(files, f.Name())
			}()
		}
	}()

	// Every 200 seconds, verify that not more than the allowance has been spent.
	go func() {
		var renterInfo api.RenterGET
		if err := j.client.Get("/renter", &renterInfo); err != nil {
			log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
		}

		var walletInfo api.WalletGET
		if err := j.client.Get("/wallet", &walletInfo); err != nil {
			log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
		}

		initialBalance := walletInfo.ConfirmedSiacoinBalance

		for {
			select {
			case <-j.tg.StopChan():
				return
			case <-time.After(time.Second * 200):
			}
			func() {
				j.tg.Add()
				defer j.tg.Done()

				if err = j.client.Get("/wallet", &walletInfo); err != nil {
					log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}

				spent := initialBalance.Sub(walletInfo.ConfirmedSiacoinBalance)
				if spent.Cmp(renterInfo.Settings.Allowance.Funds) > 0 {
					log.Printf("[%v jobStorageRenter ERROR: spent more than allowance: spent %v, allowance %v\n", j.siaDirectory, spent, renterInfo.Settings.Allowance.Funds)
				}
			}()
		}
	}()
}
