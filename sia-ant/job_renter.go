package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	mrand "math/rand"
	"os"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

// storageRenter unlocks the wallet, mines some currency, sets an allowance
// using that currency, and uploads some files.  It will periodically try to
// download those files, printing any errors that occur.
func (j *JobRunner) storageRenter() {
	done := make(chan struct{})
	defer close(done)

	j.tg.OnStop(func() {
		<-done
	})

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
			j.tg.Add()

			select {
			case <-j.tg.StopChan():
				j.tg.Done()
				return
			case <-time.After(time.Second * 1000):
			}

			// set an allowance of 50kSC + a random offset from 0-10ksc
			allowance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
			if err := j.client.Post("/renter", fmt.Sprintf("funds=%v&period=100", allowance), nil); err != nil {
				log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
			}

			j.tg.Done()
		}
	}()

	// Every 120 seconds, upload a 500MB file.  Delete one file at random once every 10 files.
	go func() {
		var files []string
		for i := 0; ; i++ {
			j.tg.Add()

			select {
			case <-j.tg.StopChan():
				j.tg.Done()
				return
			case <-time.After(time.Second * 120):
			}

			// Every 10 files, delete one file at random.
			if i%10 == 0 {
				randindex := mrand.Intn(len(files))
				if err := j.client.Post(fmt.Sprintf("/renter/delete/%v", files[randindex]), "", nil); err != nil {
					log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}
				files = append(files[:randindex], files[randindex+1:]...)
			}

			// Generate some random data to upload
			f, err := ioutil.TempFile("", "antfarm-renter")
			if err != nil {
				log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
			}
			defer os.Remove(f.Name())

			_, err = io.CopyN(f, rand.Reader, 500000000)
			if err != nil {
				log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
			}

			// Upload the random data
			if err = j.client.Post(fmt.Sprintf("/renter/upload/%v", f.Name()), fmt.Sprintf("source=%v", f.Name()), nil); err != nil {
				log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
			}

			files = append(files, f.Name())

			j.tg.Done()
		}
	}()
}
