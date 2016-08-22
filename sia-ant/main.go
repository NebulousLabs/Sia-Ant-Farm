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
// data directory `datadir` is passed as siad's `--sia-directory`.
func NewSiad(siadPath string, datadir string, apiAddr string, rpcAddr string, hostAddr string) (*exec.Cmd, error) {
	cmd := exec.Command(siadPath, "--sia-directory", datadir, "--api-addr", apiAddr, "--rpc-addr", rpcAddr, "--host-addr", hostAddr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func main() {
	siadPath := flag.String("siad", "siad", "path to siad executable")
	apiAddr := flag.String("api-addr", "localhost:9980", "api address to bind siad")
	rpcAddr := flag.String("rpc-addr", "localhost:9981", "rpc address to bind siad")
	hostAddr := flag.String("host-addr", "localhost:9982", "host address to bind siad")
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
	siad, err := NewSiad(*siadPath, datadir, *apiAddr, *rpcAddr, *hostAddr)
	if err != nil {
		panic(err)
	}

	// Naively wait for the daemon to start.
	time.Sleep(time.Second)

	// Construct the job runner
	j, err := NewJobRunner(*apiAddr, "")
	if err != nil {
		panic(err)
	}

	// Construct the signal channel and notify on it in the case of SIGINT
	// (ctrl-c) or SIGKILL
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, os.Kill)

	go func() {
		<-sigchan
		siad.Process.Kill()
	}()

	fmt.Println("> Starting jobs...")

	// Start up selected jobs
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
