package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

// NewSiad spawns a new siad process using os/exec.  siadPath is the path to
// Siad, passed directly to exec.Command.  An error is returned if starting
// siad fails, otherwise a pointer to siad's os.Cmd object is returned.  The
// data directory `datadir` is passed as siad's `--sia-directory`
func NewSiad(siadPath string, datadir string) (*exec.Cmd, error) {
	cmd := exec.Command(siadPath, "--sia-directory", datadir)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func main() {
	siadPath := flag.String("siad", "siad", "path to siad executable")
	runGateway := flag.Bool("gateway", false, "enable gateway test jobs")
	runMining := flag.Bool("mining", false, "enable mining test jobs")
	flag.Parse()

	// Create a new temporary directory for ephemeral data storage for this ant.
	datadir, err := ioutil.TempDir("", "sia-antfarm")
	if err != nil {
		panic(err)
	}
	defer func() {
		err := os.RemoveAll(datadir)
		if err != nil {
			panic(err)
		}
	}()

	// Construct a new siad instance
	siad, err := NewSiad(*siadPath, datadir)
	if err != nil {
		panic(err)
	}

	// Naively wait for the daemon to start.
	time.Sleep(time.Second)

	// Construct the job runner
	j, err := NewJobRunner("localhost:9980", "")
	if err != nil {
		panic(err)
	}

	// Construct the signal channel and notify on it in the case of SIGINT
	// (ctrl-c)
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	// Concurrently print errors or kill siad and quit on ctrl-c
	go func() {
		for {
			select {
			case <-sigchan:
				fmt.Println("Caught quit signal, quitting...")
				siad.Process.Kill()
				return
			case err := <-j.errorlog:
				fmt.Printf("%v: %v\n", time.Now(), err)
			}
		}
	}()

	fmt.Println("> Starting jobs...")

	// Start up siad jobs
	if *runGateway {
		fmt.Println(">> running gateway connectability job...")
		go j.gatewayConnectability()
	}
	if *runMining {
		fmt.Println(">> running mining job...")
		go j.blockMining()
	}

	// Wait for the siad process to return an error.  Ignore the error if it's a
	// SIGKILL, since we issue the process SIGKILL on quit.
	fmt.Println("> all jobs loaded.")
	err = siad.Wait()
	if err != nil && err.Error() != "signal: killed" {
		panic(err)
	}

}
