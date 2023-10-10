package main

import (
	"flag"
	"log"
	"time"

	configpkg "github.com/vladstarr/failover/pkg/config"
	"github.com/vladstarr/failover/pkg/failover"
)

func main() {
	config := configpkg.Config

	flag.Parse()

	log.Print("Starting failover...")
	log.Printf(`

				Working on nodes with labels: %s
				Selected master pods with labels: %s
				Selected slave pod in namespace %s with labels %s
				Failover pool node label: %s
	`, *config.NodeSelector,
		*config.MasterPodSelector,
		*config.SlavePodNamespace,
		*config.SlavePodSelector,
		*config.FailoverPoolLabel)

	for {
		failover.Run()
		time.Sleep(*config.SleepInterval)
	}
}
