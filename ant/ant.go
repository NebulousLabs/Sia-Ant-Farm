package ant

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"path"
	"runtime"
	"time"

	"github.com/NebulousLabs/Sia/sync"
	"github.com/NebulousLabs/Sia/types"
	"github.com/NebulousLabs/go-upnp"
)

// AntConfig represents a configuration object passed to New(), used to
// configure a newly created Sia Ant.
type AntConfig struct {
	APIAddr         string `json:",omitempty"`
	RPCAddr         string `json:",omitempty"`
	HostAddr        string `json:",omitempty"`
	SiaDirectory    string `json:",omitempty"`
	Name            string `json:",omitempty"`
	SiadPath        string
	Jobs            []string
	DesiredCurrency uint64

	// UpgradePath is a slice of strings, where each string is a version to
	// upgrade to.
	UpgradePath []string

	// UpgradeDelay is the number of seconds to wait between upgrades.
	UpgradeDelay int

	// UpgradeDir is the directory in which the `siad` binaries used to upgrade
	// are stored. This directory should be laid out as `dir/version-platform-arch/siad`, an
	// example directory is provided in this repo under `binary-upgrades`.
	UpgradeDir string
}

// An Ant is a Sia Client programmed with network user stories. It executes
// these user stories and reports on their successfulness.
type Ant struct {
	Config AntConfig

	siad *exec.Cmd
	jr   *jobRunner

	// A variable to track which blocks + heights the sync detector has seen
	// for this ant. The map will just keep growing, but it shouldn't take up a
	// prohibitive amount of space.
	SeenBlocks map[types.BlockHeight]types.BlockID `json:"-"`

	tg sync.ThreadGroup
}

// clearPorts discovers the UPNP enabled router and clears the ports used by an
// ant before the ant is started.
func clearPorts(config AntConfig) error {
	rpcaddr, err := net.ResolveTCPAddr("tcp", config.RPCAddr)
	if err != nil {
		return err
	}

	hostaddr, err := net.ResolveTCPAddr("tcp", config.HostAddr)
	if err != nil {
		return err
	}

	upnprouter, err := upnp.Discover()
	if err != nil {
		return err
	}

	err = upnprouter.Clear(uint16(rpcaddr.Port))
	if err != nil {
		return err
	}

	err = upnprouter.Clear(uint16(hostaddr.Port))
	if err != nil {
		return err
	}

	return nil
}

// upgraderThread is a goroutine that waits `UpgradeDelay` and upgrades to the
// next version in the ant's `UpgradePath`. Once the final version is reached,
// upgrading is stopped.
func (a *Ant) upgraderThread() {
	err := a.tg.Add()
	if err != nil {
		return
	}
	defer a.tg.Done()

	for _, version := range a.Config.UpgradePath {
		select {
		case <-time.After(time.Second * time.Duration(a.Config.UpgradeDelay)):
		case <-a.tg.StopChan():
			return
		}

		log.Printf("upgrading %v to %v...\n", a.Config.Name, version)

		stopSiad(a.Config.APIAddr, a.siad.Process)

		newSiadPath := path.Join(a.Config.UpgradeDir, fmt.Sprintf("%v-%v-%v", version, runtime.GOOS, runtime.GOARCH), "siad")

		siad, err := newSiad(newSiadPath, a.Config.SiaDirectory, a.Config.APIAddr, a.Config.RPCAddr, a.Config.HostAddr)
		if err != nil {
			log.Printf("error starting siad after upgrade: %v\n", err)
			continue
		}

		a.siad = siad
	}
}

// New creates a new Ant using the configuration passed through `config`.
func New(config AntConfig) (*Ant, error) {
	var err error
	// unforward the ports required for this ant
	err = clearPorts(config)
	if err != nil {
		log.Printf("error clearing upnp ports for ant: %v\n", err)
	}

	// Construct the ant's Siad instance
	siad, err := newSiad(config.SiadPath, config.SiaDirectory, config.APIAddr, config.RPCAddr, config.HostAddr)
	if err != nil {
		return nil, err
	}

	// Ensure siad is always stopped if an error is returned.
	defer func() {
		if err != nil {
			stopSiad(config.APIAddr, siad.Process)
		}
	}()

	j, err := newJobRunner(config.APIAddr, "", config.SiaDirectory)
	if err != nil {
		return nil, err
	}

	for _, job := range config.Jobs {
		switch job {
		case "miner":
			go j.blockMining()
		case "host":
			go j.jobHost()
		case "renter":
			go j.storageRenter()
		case "gateway":
			go j.gatewayConnectability()
		}
	}

	if config.DesiredCurrency != 0 {
		go j.balanceMaintainer(types.SiacoinPrecision.Mul64(config.DesiredCurrency))
	}

	a := &Ant{
		Config: config,

		siad: siad,
		jr:   j,

		SeenBlocks: make(map[types.BlockHeight]types.BlockID),
	}

	if len(config.UpgradePath) > 0 {
		go a.upgraderThread()
	}

	return a, nil
}

// Close releases all resources created by the ant, including the Siad
// subprocess.
func (a *Ant) Close() error {
	a.tg.Stop()
	a.jr.Stop()
	stopSiad(a.Config.APIAddr, a.siad.Process)
	return nil
}

// BlockHeight returns the highest block height seen by the ant.
func (a *Ant) BlockHeight() types.BlockHeight {
	height := types.BlockHeight(0)
	for h := range a.SeenBlocks {
		if h > height {
			height = h
		}
	}
	return height
}
