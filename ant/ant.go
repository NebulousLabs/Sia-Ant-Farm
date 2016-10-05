package ant

import (
	"os/exec"
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

	return &Ant{
		APIAddr: config.APIAddr,
		RPCAddr: config.RPCAddr,

		siad: siad,
		jr:   j,
	}, nil
}

// Close releases all resources created by the ant, including the Siad
// subprocess.
func (a *Ant) Close() error {
	a.jr.Stop()
	stopSiad(a.APIAddr, a.siad.Process)
	return nil
}
