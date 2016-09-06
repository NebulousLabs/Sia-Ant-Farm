package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/crypto"
	"github.com/NebulousLabs/Sia/types"
)

type SiaConstants struct {
	GenesisTimestamp      types.Timestamp   `json:"genesistimestamp"`
	BlockSizeLimit        uint64            `json:"blocksizelimit"`
	BlockFrequency        types.BlockHeight `json:"blockfrequency"`
	TargetWindow          types.BlockHeight `json:"targetwindow"`
	MedianTimestampWindow uint64            `json:"mediantimestampwindow"`
	FutureThreshold       types.Timestamp   `json:"futurethreshold"`
	SiafundCount          types.Currency    `json:"siafundcount"`
	SiafundPortion        *big.Rat          `json:"siafundportion"`
	MaturityDelay         types.BlockHeight `json:"maturitydelay"`

	InitialCoinbase uint64 `json:"initialcoinbase"`
	MinimumCoinbase uint64 `json:"minimumcoinbase"`

	RootTarget types.Target `json:"roottarget"`
	RootDepth  types.Target `json:"rootdepth"`

	MaxAdjustmentUp   *big.Rat `json:"maxadjustmentup"`
	MaxAdjustmentDown *big.Rat `json:"maxadjustmentdown"`

	SiacoinPrecision types.Currency `json:"siacoinprecision"`
}

// storageRenter unlocks the wallet, mines some currency, sets an allowance
// using that currency, and uploads some files.  It will periodically try to
// download or delete those files, printing any errors that occur.
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

	// Set an allowance period of 100 blocks.  After this period elapses, set a
	// new allowance.
	var daemonConstants SiaConstants
	if err := j.client.Get("/daemon/constants", &daemonConstants); err != nil {
		log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
		return
	}

	// allowancePeriod is the number of blocks to set the allowance for,
	// allowanceFrequency is the frequency at which to set subsequent new
	// allowances.
	allowancePeriod := 100
	allowanceFrequency := time.Duration(int(daemonConstants.BlockFrequency)*allowancePeriod) * time.Second

	// Set an initial 50ksc allowance
	allowance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
	if err := j.client.Post("/renter", fmt.Sprintf("funds=%v&period=%v", allowance, allowancePeriod), nil); err != nil {
		log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
	}

	// Set a 50ksc allowance every allowanceFrequency seconds.
	go func() {
		for {
			select {
			case <-j.tg.StopChan():
				return
			case <-time.After(allowanceFrequency):
			}
			func() {
				j.tg.Add()
				defer j.tg.Done()

				// TODO: verify that spending does not exceed the set allowance.

				// set an allowance of 50k SC
				allowance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
				if err := j.client.Post("/renter", fmt.Sprintf("funds=%v&period=%v", allowance, allowancePeriod), nil); err != nil {
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
					os.Remove(files[randindex])
					files = append(files[:randindex], files[randindex+1:]...)
				}

				// Generate some random data to upload
				f, err := ioutil.TempFile("", "antfarm-renter")
				if err != nil {
					log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}
				files = append(files, f.Name())

				_, err = io.CopyN(f, rand.Reader, 500e6)
				if err != nil {
					log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}

				// Upload the random data
				if err = j.client.Post(fmt.Sprintf("/renter/upload/%v", f.Name()), fmt.Sprintf("source=%v", f.Name()), nil); err != nil {
					log.Printf("[%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}
			}()
		}
	}()

	// Every 200 seconds, download a file.  Verify that the download call
	// succeeds correctly, the file is placed in the download list, and the file
	// is removed from the download list, indicating successful download
	// completion.
	go func() {
		for {
			select {
			case <-j.tg.StopChan():
				return
			case <-time.After(time.Second * 200):
			}

			func() {
				j.tg.Add()
				defer j.tg.Done()

				// Download a random file from the renter's file list
				var renterFiles api.RenterFiles
				if err := j.client.Get("/renter/files", &renterFiles); err != nil {
					log.Printf("%v jobStorageRenter ERROR: %v\n", j.siaDirectory, err)
				}

				// Filter out files which are not available.
				availableFiles := renterFiles.Files[:0]
				for _, file := range renterFiles.Files {
					if file.Available {
						availableFiles = append(availableFiles, file)
					}
				}

				// Download a file at random.
				randindex, _ := crypto.RandIntn(len(availableFiles))
				fileToDownload := availableFiles[randindex]

				f, err := ioutil.TempFile("", "antfarm-renter")
				if err != nil {
					log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
				}
				defer os.Remove(f.Name())

				if err = j.client.Post(fmt.Sprintf("/renter/download/%v", fileToDownload.SiaPath), fmt.Sprintf("destination=%v", f.Name()), nil); err != nil {
					log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
					return
				}

				// isFileInDownloads grabs the files currently being downloaded by the
				// renter and returns bool `true` if fileToDownload exists in the
				// download list.
				isFileInDownloads := func() bool {
					var renterDownloads api.RenterDownloadQueue
					if err = j.client.Get("/renter/downloads", &renterDownloads); err != nil {
						log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
					}

					hasFile := false
					for _, download := range renterDownloads.Downloads {
						if download.SiaPath == fileToDownload.SiaPath {
							hasFile = true
						}
					}

					return hasFile
				}

				// Wait for the file to appear in the download list
				success := false
				for start := time.Now(); time.Since(start) < 1*time.Minute; {
					select {
					case <-j.tg.StopChan():
						break
					case <-time.After(time.Second):
					}

					if isFileInDownloads() {
						success = true
						break
					}
				}
				if !success {
					log.Printf("[%v jobStorageRenter ERROR]: file %v did not appear in the renter download list\n", j.siaDirectory, fileToDownload.SiaPath)
					return
				}

				// Wait for the file to be finished downloading, with a timeout of 15 minutes.
				success = false
				for start := time.Now(); time.Since(start) < 15*time.Minute; {
					select {
					case <-j.tg.StopChan():
						break
					case <-time.After(time.Second):
					}

					if !isFileInDownloads() {
						success = true
						break
					}
				}
				if !success {
					log.Printf("[%v jobStorageRenter ERROR]: file %v did not complete downloading\n", j.siaDirectory, fileToDownload.SiaPath)
				}
			}()
		}
	}()
}
