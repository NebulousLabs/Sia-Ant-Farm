package main

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/build"
	"github.com/NebulousLabs/Sia/crypto"
	"github.com/NebulousLabs/Sia/types"
)

const (
	// downloadFileFrequency defines how frequently the renter job downloads
	// files from the network.
	downloadFileFrequency = uploadFileFrequency * 3 / 2

	// initialBalanceWarningTimeout defines how long the renter will wait
	// before reporting to the user that the required inital balance has not
	// been reached.
	initialBalanceWarningTimeout = time.Minute * 10

	// setAllowanceWarningTimeout defines how long the renter will wait before
	// reporting to the user that the allowance has not yet been set
	// successfully.
	setAllowanceWarningTimeout = time.Minute * 2

	// uploadFileFrequency defines how frequently the renter job uploads files
	// to the network.
	uploadFileFrequency = time.Second * 240

	// uploadTimeout defines the maximum time allowed for an upload operation to
	// complete, ie for an upload to reach 100%.
	maxUploadTime = time.Minute * 10

	// renterAllowancePeriod defines the block duration of the renter's allowance
	renterAllowancePeriod = 100

	// uploadFileSize defines the size of the test files to be uploaded.  Test
	// files are filled with random data.
	uploadFileSize = 500e3
)

var (
	// renterAllowance defines the number of coins that the renter has to
	// spend.
	renterAllowance = types.NewCurrency64(20e3).Mul(types.SiacoinPrecision)

	// requiredInitialBalance sets the number of coins that the renter requires
	// before uploading will begin.
	requiredInitialBalance = types.NewCurrency64(100e3).Mul(types.SiacoinPrecision)
)

// renterFile stores the location and checksum of a file active on the renter.
type renterFile struct {
	merkleRoot crypto.Hash
	sourceFile string
}

// renterJob contains statefulness that is used to drive the renter. Most
// importantly, it contains a list of files that the renter is currently
// uploading to the network.
type renterJob struct {
	files []renterFile

	jr *JobRunner
	mu sync.Mutex
}

// randFillFile will append 'size' bytes to the input file, returning the
// merkle root of the bytes that were appended. For whatever reason,
// rand.Reader is really slow. This will be substantially faster for large
// files.
func randFillFile(f *os.File, size uint64) (crypto.Hash, error) {
	// Get some initial entropy which will be used to guarantee randomness for
	// the file.
	initialEntropy := make([]byte, crypto.HashSize)
	_, err := rand.Read(initialEntropy)
	if err != nil {
		return crypto.Hash{}, err
	}

	// Sanity check - the next bit of code assumes that crypto.SegmentSize is
	// 2x crypto.HashSize. If that's not the case, panic.
	if crypto.HashSize*2 != crypto.SegmentSize {
		build.Critical("randFillFile written for different constants", crypto.HashSize, crypto.SegmentSize)
	}

	var progress uint64
	t := crypto.NewTree()
	for progress < size {
		firstHalf := crypto.HashAll(progress, initialEntropy)
		secondHalf := crypto.HashAll(progress+1, initialEntropy)
		full := append(firstHalf[:], secondHalf[:]...)

		// Truncate 'full' if we're at the last bit of data and there's less
		// than crypto.SegmentSize bytes left to write.
		if size-progress < crypto.SegmentSize {
			full = full[:size-progress]
		}

		// Push the rand data into the merkle tree.
		t.PushObject(full)

		// Write the rand data to the file.
		_, err = f.Write(full)
		if err != nil {
			return crypto.Hash{}, err
		}

		progress += crypto.SegmentSize
	}

	return t.Root(), nil
}

// permanentDownloader is a function that continuously runs for the renter job,
// downloading a file at random every 400 seconds.
func (r *renterJob) permanentDownloader() {
	// Wait for the first file to be uploaded before starting the download
	// loop.
	for {
		// Download a file.
		r.download()

		select {
		case <-r.jr.tg.StopChan():
			return
		case <-time.After(downloadFileFrequency):
		}
	}
}

// permanentUploader is a function that continuously runs for the renter job,
// uploading a 500MB file every 240 seconds (10 blocks). The renter should have
// already set an allowance.
func (r *renterJob) permanentUploader() {
	// Make the source files directory
	os.Mkdir(filepath.Join(r.jr.siaDirectory, "renterSourceFiles"), 0700)
	for {
		// Upload a file.
		//
		// TODO: Consider having this return an error, and then performing the
		// logging from here instead of doing the logging in the upload
		// function.
		r.upload()

		// Wait a while between upload attempts.
		select {
		case <-r.jr.tg.StopChan():
			return
		case <-time.After(uploadFileFrequency):
		}
	}
}

// download will download a random file from the network.
func (r *renterJob) download() {
	r.jr.tg.Add()
	defer r.jr.tg.Done()

	// Download a random file from the renter's file list
	var renterFiles api.RenterFiles
	if err := r.jr.client.Get("/renter/files", &renterFiles); err != nil {
		log.Printf("[ERROR] [renter] [%v]: error calling /renter/files: %v\n", r.jr.siaDirectory, err)
		return
	}

	// Filter out files which are not available.
	availableFiles := renterFiles.Files[:0]
	for _, file := range renterFiles.Files {
		if file.Available {
			availableFiles = append(availableFiles, file)
		}
	}

	// Do nothing if there are not any files to be downloaded.
	if len(availableFiles) == 0 {
		return
	}

	// Download a file at random.
	randindex, _ := crypto.RandIntn(len(availableFiles))
	fileToDownload := availableFiles[randindex]

	// Use ioutil.TempFile to get a random temporary filename.
	f, err := ioutil.TempFile("", "antfarm-renter")
	if err != nil {
		log.Printf("[ERROR] [renter] [%v]: failed to create temporary file for download: %v\n", r.jr.siaDirectory, err)
		return
	}
	destPath, _ := filepath.Abs(f.Name())
	os.Remove(destPath)

	log.Printf("[INFO] [renter] [%v] downloading %v to %v", r.jr.siaDirectory, fileToDownload.SiaPath, destPath)

	if err = r.jr.client.Post(fmt.Sprintf("/renter/download/%v", fileToDownload.SiaPath), fmt.Sprintf("destination=%v", destPath), nil); err != nil {
		log.Printf("[ERROR] [renter] [%v]: failed in call to /renter/download: %v\n", r.jr.siaDirectory, err)
		return
	}

	// isFileInDownloads grabs the files currently being downloaded by the
	// renter and returns bool `true` if fileToDownload exists in the
	// download list.
	isFileInDownloads := func() bool {
		var renterDownloads api.RenterDownloadQueue
		if err = r.jr.client.Get("/renter/downloads", &renterDownloads); err != nil {
			log.Printf("[ERROR] [renter] [%v]: call to /renter/downloads failed: %v\n", r.jr.siaDirectory, err)
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
		case <-r.jr.tg.StopChan():
			break
		case <-time.After(time.Second):
		}

		if isFileInDownloads() {
			success = true
			break
		}
	}
	if !success {
		log.Printf("[ERROR] [renter] [%v]: file %v did not appear in the renter download list after 1 minute\n", r.jr.siaDirectory, fileToDownload.SiaPath)
		return
	}

	// Wait for the file to be finished downloading, with a timeout of 15 minutes.
	success = false
	for start := time.Now(); time.Since(start) < 15*time.Minute; {
		select {
		case <-r.jr.tg.StopChan():
			break
		case <-time.After(time.Second):
		}

		if !isFileInDownloads() {
			success = true
			break
		}
	}
	if !success {
		log.Printf("[ERROR] [renter] [%v]: file %v did not complete downloading after 15 minutes\n", r.jr.siaDirectory, fileToDownload.SiaPath)
		return
	}
	log.Printf("[INFO] [renter] [%v]: successfully downloaded %v to %v\n", r.jr.siaDirectory, fileToDownload.SiaPath, destPath)
}

// upload will upload a file to the network. If the api reports that there are
// more than 10 files successfully uploaded, then a file is deleted at random.
func (r *renterJob) upload() {
	r.jr.tg.Add()
	defer r.jr.tg.Done()

	/*
		if i >= 10 {
			randindex, err := crypto.RandIntn(len(files))
			if err != nil {
				log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
				return
			}
			if err = j.client.Post(fmt.Sprintf("/renter/delete/%v", files[randindex]), "", nil); err != nil {
				log.Printf("[%v jobStorageRenter ERROR]: %v\n", j.siaDirectory, err)
				return
			}
			log.Printf("[%v jobStorageRenter INFO]: successfully deleted file\n", j.siaDirectory)
			os.Remove(files[randindex])
			files = append(files[:randindex], files[randindex+1:]...)
		}
	*/

	// Generate some random data to upload. The file needs to be closed before
	// the upload to the network starts, so this code is wrapped in a func such
	// that a `defer Close()` can be used on the file.
	log.Printf("[INFO] [renter] [%v] File upload preparation beginning.\n", r.jr.siaDirectory)
	var sourcePath string
	var merkleRoot crypto.Hash
	success := func() bool {
		f, err := ioutil.TempFile(filepath.Join(r.jr.siaDirectory, "renterSourceFiles"), "renterFile")
		if err != nil {
			log.Printf("[ERROR] [renter] [%v] Unable to open tmp file for renter source file: %v\n", r.jr.siaDirectory, err)
			return false
		}
		defer f.Close()
		sourcePath, _ = filepath.Abs(f.Name())

		// Fill the file with random data.
		merkleRoot, err = randFillFile(f, uploadFileSize)
		if err != nil {
			log.Printf("[ERROR] [renter] [%v] Unable to fill file with randomness: %v\n", r.jr.siaDirectory, err)
			return false
		}
		return true
	}()
	if !success {
		return
	}

	// use the sourcePath with its leading slash stripped for the sia path
	siapath := sourcePath[1:]

	// Add the file to the renter.
	rf := renterFile{
		merkleRoot: merkleRoot,
		sourceFile: sourcePath,
	}
	r.mu.Lock()
	r.files = append(r.files, rf)
	r.mu.Unlock()
	log.Printf("[INFO] [renter] [%v] File upload preparation complete, beginning file upload.\n", r.jr.siaDirectory)

	// Upload the file to the network.
	if err := r.jr.client.Post(fmt.Sprintf("/renter/upload/%v", siapath), fmt.Sprintf("source=%v", sourcePath), nil); err != nil {
		log.Printf("[ERROR] [renter] [%v] Unable to upload file to network: %v", r.jr.siaDirectory, err)
		return
	}
	log.Printf("[INFO] [renter] [%v] /renter/upload call completed successfully.  Waiting for the upload to complete\n", r.jr.siaDirectory)

	// Block until the upload has reached 100%.
	success = false
	for start := time.Now(); time.Since(start) < maxUploadTime; {
		select {
		case <-r.jr.tg.StopChan():
			return
		case <-time.After(time.Second * 5):
		}

		var rfg api.RenterFiles
		if err := r.jr.client.Get("/renter/files", &rfg); err != nil {
			log.Printf("[ERROR] [renter] [%v]: error calling /renter/files: %v\n", r.jr.siaDirectory, err)
			return
		}

		uploadProgress := 0.0
		for _, file := range rfg.Files {
			if file.SiaPath == siapath {
				uploadProgress = file.UploadProgress
			}
		}
		log.Printf("[INFO] [renter] [%v]: upload progress: %v%%\n", r.jr.siaDirectory, uploadProgress)
		if uploadProgress == 100 {
			success = true
			break
		}
	}
	if success {
		log.Printf("[INFO] [renter] [%v]: file has been successfully uploaded to 100%.\n", r.jr.siaDirectory)
	} else {
		log.Printf("[ERROR] [renter] [%v]: file with siapath %v could not be fully uploaded after 10 minutes.=n", r.jr.siaDirectory, siapath)
	}
}

// storageRenter unlocks the wallet, mines some currency, sets an allowance
// using that currency, and uploads some files.  It will periodically try to
// download or delete those files, printing any errors that occur.
func (j *JobRunner) storageRenter() {
	j.tg.Add()
	defer j.tg.Done()

	// Unlock the wallet and begin mining to earn enough coins for uploading.
	err := j.client.Post("/wallet/unlock", fmt.Sprintf("encryptionpassword=%s&dictionary=%s", j.walletPassword, "english"), nil)
	if err != nil {
		log.Printf("[ERROR] [renter] [%v] Trouble when unlocking wallet: %v\n", j.siaDirectory, err)
		return
	}
	err = j.client.Get("/miner/start", nil)
	if err != nil {
		log.Printf("[ERROR] [renter] [%v] Trouble when starting the miner: %v\n", j.siaDirectory, err)
		return
	}

	// Block until a minimum threshold of coins have been mined.
	start := time.Now()
	var walletInfo api.WalletGET
	log.Printf("[INFO] [renter] [%v] Blocking until wallet is sufficiently full\n", j.siaDirectory)
	for walletInfo.ConfirmedSiacoinBalance.Cmp(requiredInitialBalance) < 0 {
		// Log an error if the time elapsed has exceeded the warning threshold.
		if time.Since(start) > initialBalanceWarningTimeout {
			log.Printf("[ERROR] [renter] [%v] Minimum balance for allowance has not been reached. Time elapsed: %v\n", j.siaDirectory, time.Since(start))
		}

		// Wait before trying to get the balance again.
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second * 15):
		}

		// Update the wallet balance.
		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			log.Printf("[ERROR] [renter] [%v] Trouble when calling /wallet: %v\n", j.siaDirectory, err)
		}
	}
	log.Printf("[INFO] [renter] [%v] Wallet filled successfully. Blocking until allowance has been set.\n", j.siaDirectory)

	// Block until a renter allowance has successfully been set.
	start = time.Now()
	for {
		log.Printf("[DEBUG] [renter] [%v] Attempting to set allowance.\n", j.siaDirectory)
		err := j.client.Post("/renter", fmt.Sprintf("funds=%v&period=%v", renterAllowance, renterAllowancePeriod), nil)
		log.Printf("[DEBUG] [renter] [%v] Allowance attempt complete: %v\n", j.siaDirectory, err)
		if err == nil {
			// Success, we can exit the loop.
			break
		}
		if err != nil && time.Since(start) > setAllowanceWarningTimeout {
			log.Printf("[ERROR] [renter] [%v] Trouble when setting renter allowance: %v\n", j.siaDirectory, err)
		}

		// Wait a bit before trying again.
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second * 15):
		}
	}
	log.Printf("[INFO] [renter] [%v] Renter allowance has been set successfully.\n", j.siaDirectory)

	// Spawn the uploader and downloader threads.
	rj := renterJob{
		jr: j,
	}
	go rj.permanentUploader()
	go rj.permanentDownloader()
}
