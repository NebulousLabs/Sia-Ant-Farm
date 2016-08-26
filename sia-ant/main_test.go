package main

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/NebulousLabs/Sia/api"
)

// TestNewSiad tests that NewSiad creates a reachable Sia API
func TestNewSiad(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	datadir, err := ioutil.TempDir("", "sia-testing")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)

	siad, err := NewSiad("siad", datadir, "localhost:9990", "localhost:0", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer siad.Process.Kill()

	c := api.NewClient("localhost:9990", "")
	if err := c.Get("/consensus", nil); err != nil {
		t.Fatal(err)
	}
}
