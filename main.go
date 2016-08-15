package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
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
	flag.Parse()
	siad, err := NewSiad(*siadPath)
	if err != nil {
		panic(err)
	}

	// Construct the logger channel
	logchan := make(chan interface{})

	// Construct the signal channel
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	go func() {
		for {
			select {
			case <-sigchan:
				siad.Process.Kill()
				return
			case msg := <-logchan:
				fmt.Println(msg)
			}
		}
	}()

	// Start up a Peer Connectability job
	go JobPeerConnectability(logchan)

	// Wait for the siad process to return an error.  Ignore the error if it's a
	// SIGKILL, since we issue the process SIGKILL on quit.
	err = siad.Wait()
	if err != nil && err.Error() != "signal: killed" {
		panic(err)
	}
}
