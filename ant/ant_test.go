package ant

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

func TestNewAnt(t *testing.T) {
	datadir, err := ioutil.TempDir("", "testing-data")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)
	config := AntConfig{
		APIAddr:      "localhost:31337",
		RPCAddr:      "localhost:31338",
		HostAddr:     "localhost:31339",
		SiaDirectory: datadir,
		SiadPath:     "siad",
	}

	ant, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	defer ant.Close()

	c := api.NewClient("localhost:31337", "")
	if err = c.Get("/consensus", nil); err != nil {
		t.Fatal(err)
	}
}

func TestStartJob(t *testing.T) {
	datadir, err := ioutil.TempDir("", "testing-data")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)

	config := AntConfig{
		APIAddr:      "localhost:31337",
		RPCAddr:      "localhost:31338",
		HostAddr:     "localhost:31339",
		SiaDirectory: datadir,
		SiadPath:     "siad",
	}

	ant, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	defer ant.Close()

	// nonexistent job should throw an error
	err = ant.StartJob("thisjobdoesnotexist")
	if err == nil {
		t.Fatal("StartJob should return an error with a nonexistent job")
	}
}

func TestWalletAddress(t *testing.T) {
	datadir, err := ioutil.TempDir("", "testing-data")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)

	config := AntConfig{
		APIAddr:      "localhost:31337",
		RPCAddr:      "localhost:31338",
		HostAddr:     "localhost:31339",
		SiaDirectory: datadir,
		SiadPath:     "siad",
	}

	ant, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	defer ant.Close()

	addr, err := ant.WalletAddress()
	if err != nil {
		t.Fatal(err)
	}
	blankaddr := types.UnlockHash{}
	if *addr == blankaddr {
		t.Fatal("WalletAddress returned an empty address")
	}
}
