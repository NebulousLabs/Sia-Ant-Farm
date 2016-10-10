package main

import (
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/NebulousLabs/Sia-Ant-Farm/ant"
)

type (
	// AntfarmConfig contains the fields to parse and use to create a sia-antfarm.
	AntfarmConfig struct {
		ListenAddress string
		AntConfigs    []ant.AntConfig
		AutoConnect   bool

		// ExternalFarms is a slice of net addresses representing the API addresses
		// of other antFarms to connect to.
		ExternalFarms []string
	}

	// antFarm defines the 'antfarm' type. antFarm orchestrates a collection of
	// ants and provides an API server to interact with them.
	antFarm struct {
		apiListener net.Listener

		// ants is a slice of Ants in this antfarm.
		ants []*ant.Ant

		// externalAnts is a slice of externally connected ants, that is, ants that
		// are connected to this antfarm but managed by another antfarm.
		externalAnts []*ant.Ant
		router       *httprouter.Router
	}
)

// createAntfarm creates a new antFarm given the supplied AntfarmConfig
func createAntfarm(config AntfarmConfig) (*antFarm, error) {
	// clear old antfarm data before creating an antfarm
	os.RemoveAll("./antfarm-data")
	os.MkdirAll("./antfarm-data", 0700)

	farm := &antFarm{}

	// start up each ant process with its jobs
	ants, err := startAnts(config.AntConfigs...)
	if err != nil {
		return nil, err
	}
	farm.ants = ants
	defer func() {
		if err != nil {
			farm.Close()
		}
	}()

	// if the AutoConnect flag is set, use connectAnts to bootstrap the network.
	if config.AutoConnect {
		if err = connectAnts(ants...); err != nil {
			return nil, err
		}
	}
	// connect the external antFarms
	for _, address := range config.ExternalFarms {
		if err = farm.connectExternalAntfarm(address); err != nil {
			return nil, err
		}
	}
	// start up the api server listener
	farm.apiListener, err = net.Listen("tcp", config.ListenAddress)
	if err != nil {
		return nil, err
	}

	// construct the router and serve the API.
	farm.router = httprouter.New()
	farm.router.GET("/ants", farm.getAnts)

	return farm, nil
}

// allAnts returns all ants, external and internal, associated with this
// antFarm.
func (af *antFarm) allAnts() []*ant.Ant {
	return append(af.ants, af.externalAnts...)
}

// connectExternalAntfarm connects the current antfarm to an external antfarm,
// using the antfarm api at externalAddress.
func (af *antFarm) connectExternalAntfarm(externalAddress string) error {
	res, err := http.DefaultClient.Get("http://" + externalAddress + "/ants")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var externalAnts []*ant.Ant
	err = json.NewDecoder(res.Body).Decode(&externalAnts)
	if err != nil {
		return err
	}
	af.externalAnts = append(af.externalAnts, externalAnts...)
	return connectAnts(af.allAnts()...)
}

// ServeAPI serves the antFarm's http API.
func (af *antFarm) ServeAPI() error {
	http.Serve(af.apiListener, af.router)
	return nil
}

// permanentSyncMonitor checks that all ants in the antFarm are on the same
// blockchain.
func (af *antFarm) permanentSyncMonitor() {
	// Give 30 seconds for everything to start up.
	time.Sleep(time.Second * 30)

	// Every 20 seconds, list all consensus groups.
	for {
		time.Sleep(time.Second * 20)
		groups, err := antConsensusGroups(af.allAnts()...)
		if err != nil {
			log.Println("error checking sync status of antfarm: ", err)
			continue
		}
		if len(groups) == 1 {
			log.Println("Ants are synchronized.")
		} else {
			log.Println("Ants split into multiple groups, displaying")
			for i, group := range groups {
				if i != 0 {
					log.Println()
				}
				log.Println("Group ", i+1)
				for _, a := range group {
					log.Println(a.APIAddr)
				}
			}
		}
	}
}

// getAnts is a http handler that returns the ants currently running on the
// antfarm.
func (af *antFarm) getAnts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := json.NewEncoder(w).Encode(af.ants)
	if err != nil {
		http.Error(w, "error encoding ants", 500)
	}
}

// Close signals all the ants to stop and waits for them to return.
func (af *antFarm) Close() error {
	if af.apiListener != nil {
		af.apiListener.Close()
	}
	for _, ant := range af.ants {
		ant.Close()
	}
	return nil
}
