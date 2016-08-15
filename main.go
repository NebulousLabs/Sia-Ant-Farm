package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

// NewSiad spawns a new siad process using os/exec.  siadPath is the path to
// Siad, passed directly to exec.Command.  An error is returned if starting
// siad fails, otherwise a pointer to siad's os.Cmd object is returned.
func NewSiad(siadPath string) (*exec.Cmd, error) {
	cmd := exec.Command(siadPath)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func main() {
	siadPath := flag.String("siad", "siad", "path to siad executable")
	runGateway := flag.Bool("gateway", false, "enable gateway test jobs")
	flag.Parse()

	// Construct a new siad instance
	siad, err := NewSiad(*siadPath)
	if err != nil {
		panic(err)
	}

	// Construct the job runner
	j := NewJobRunner()

	// Construct the signal channel and notify on it in the case of SIGINT
	// (ctrl-c)
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	// Concurrently print errors or kill siad and quit on ctrl-c
	go func() {
		for {
			select {
			case <-sigchan:
				siad.Process.Kill()
				return
			case err := <-j.errorlog:
				fmt.Printf("%v: %v\n", time.Now(), err)
			}
		}
	}()

	// Start up siad jobs
	if *runGateway {
		fmt.Println("running gateway connectability job...")
		go j.gatewayConnectability()
	}

	// Wait for the siad process to return an error.  Ignore the error if it's a
	// SIGKILL, since we issue the process SIGKILL on quit.
	err = siad.Wait()
	if err != nil && err.Error() != "signal: killed" {
		panic(err)
	}
}
