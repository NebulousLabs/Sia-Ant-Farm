package main

import (
	"log"
	"os"
	"sync"
	"time"
)

type (
	// AntfarmConfig contains the fields to parse and use to create a sia-antfarm.
	AntfarmConfig struct {
		AntConfigs    []AntConfig
		AutoConnect   bool
		ExternalFarms []string
	}

	// antFarm defines the 'antfarm' type. antFarm orchestrates a collection of
	// ants and provides an API server to interact with them.
	antFarm struct {
		wg   sync.WaitGroup
		ants []*Ant
	}
)

// createAntfarm creates a new antFarm given the supplied AntfarmConfig
func createAntfarm(config AntfarmConfig) (*antFarm, error) {
	// clear old antfarm data before creating an antfarm
	os.RemoveAll("./antfarm-data")

	farm := &antFarm{}

	// start up each ant process with its jobs
	ants, err := startAnts(config.AntConfigs...)
	if err != nil {
		return nil, err
	}
	farm.ants = ants
	farm.wg.Add(len(ants))
	go func() {
		for _, ant := range ants {
			ant.Wait()
			farm.wg.Done()
		}
	}()

	// if the AutoConnect flag is set, use connectAnts to bootstrap the network.
	if config.AutoConnect {
		if err = connectAnts(ants...); err != nil {
			return nil, err
		}
	}

	go farm.permanentSyncMonitor()

	return farm, nil
}

// permanentSyncMonitor checks that all ants in the antFarm are on the same
// blockchain.
func (af *antFarm) permanentSyncMonitor() {
	// Give 30 seconds for everything to start up.
	time.Sleep(time.Second * 30)

	// Every 20 seconds, list all consensus groups.
	for {
		time.Sleep(time.Second * 20)
		groups, err := antConsensusGroups(af.ants...)
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
				for _, ant := range group {
					log.Println(ant.apiaddr)
				}
			}
		}
	}
}

// Close signals all the ants to stop and waits for them to return.
func (af *antFarm) Close() error {
	for _, ant := range af.ants {
		ant.Process.Signal(os.Interrupt)
	}
	af.wg.Wait()
	return nil
}
