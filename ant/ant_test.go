package ant

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

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

type DaemonVersion struct {
	Version string `json:"version"`
}

func TestUpgradeAnt(t *testing.T) {
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

		UpgradePath: []string{
			"1.1.0",
			"1.1.1",
			"1.1.2",
		},
		UpgradeDelay: 10,
		UpgradeDir:   "../binary-upgrades",
	}

	ant, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	defer ant.Close()

	seenVersions := make(map[string]bool)
	success := false

	for start := time.Now(); time.Since(start) < time.Minute*10; time.Sleep(time.Second * 3) {
		c := api.NewClient("localhost:31337", "")
		dv := DaemonVersion{}
		c.Get("/daemon/version", &dv)
		seenVersions[dv.Version] = true

		hasAllVersions := true
		for _, version := range config.UpgradePath {
			if _, ok := seenVersions[version]; !ok {
				hasAllVersions = false
			}
		}

		if hasAllVersions {
			success = true
			break
		}
	}

	if !success {
		t.Fatal("ant did not follow the upgrade path")
	}
}
