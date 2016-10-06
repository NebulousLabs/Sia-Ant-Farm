package ant

import (
	"os/exec"

	"github.com/NebulousLabs/Sia/types"
)

// AntConfig represents a configuration object passed to New(), used to
// configure a newly created Sia Ant.
type AntConfig struct {
	APIAddr      string `json:",omitempty"`
	RPCAddr      string `json:",omitempty"`
	HostAddr     string `json:",omitempty"`
	SiaDirectory string `json:",omitempty"`
	SiadPath     string
	Jobs         []string
}

// An Ant is a Sia Client programmed with network user stories. It executes
// these user stories and reports on their successfulness.
type Ant struct {
	APIAddr string
	RPCAddr string

	siad *exec.Cmd
	jr   *jobRunner

	// A variable to track which blocks + heights the sync detector has seen
	// for this ant. The map will just keep growing, but it shouldn't take up a
	// prohibitive amount of space.
	SeenBlocks map[types.BlockHeight]types.BlockID `json:"-"`
}

// New creates a new Ant using the configuration passed through `config`.
func New(config AntConfig) (*Ant, error) {
	// Construct the ant's Siad instance
	siad, err := newSiad(config.SiadPath, config.SiaDirectory, config.APIAddr, config.RPCAddr, config.HostAddr)
	if err != nil {
		return nil, err
	}

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

	return &Ant{
		APIAddr: config.APIAddr,
		RPCAddr: config.RPCAddr,

		siad: siad,
		jr:   j,

		SeenBlocks: make(map[types.BlockHeight]types.BlockID),
	}, nil
}

// Close releases all resources created by the ant, including the Siad
// subprocess.
func (a *Ant) Close() error {
	a.jr.Stop()
	stopSiad(a.APIAddr, a.siad.Process)
	return nil
}
