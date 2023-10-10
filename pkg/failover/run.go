package failover

import (
	"log"

	configpkg "github.com/vladstarr/failover/pkg/config"
)

var config = configpkg.Config

// Run performs main failover logic
func Run() {
	nodes, err := getNodes(*config.NodeSelector)

	if err != nil {
		log.Fatalf("Failed to get nodes: %v", err)
	} else if len(nodes.Items) == 0 {
		log.Fatalf("No nodes found with selector %s", *config.NodeSelector)
	}

	for _, node := range nodes.Items {

		masterPod, err := getMasterPod(&node, *config.MasterPodSelector)
		if err != nil {
			log.Fatalf("Failed to get master pod: %v", err)
		}

		slavePod, err := getSlavePod(&node, *config.SlavePodNamespace, *config.SlavePodSelector, *config.FailoverPoolLabel)
		if err != nil {
			log.Fatalf("Failed to get slave pods: %v", err)
		}

		nodeInPool := isNodeInPool(&node, *config.FailoverPoolLabel)

		// check if master pod is absent or not ready, or node is not ready
		if (masterPod == nil || !isReadyPod(masterPod) || !isReadyNode(&node)) && nodeInPool {

			if err := removeNodeFromPool(&node, *config.FailoverPoolLabel); err != nil {
				log.Printf("Failed to remove node %s from failover pool: %v", node.Name, err)
			} else {
				log.Printf("Removed node %s from failover pool", node.Name)
			}

			// if there is failover pod on the node, perform failover
			if slavePod != nil {
				deployment, err := getDeploymentByPod(slavePod, *config.SlavePodNamespace)
				if err != nil {
					log.Printf("Failed to get deployment of pod %s: %v", slavePod.Name, err)
				}
				if deployment != nil {
					log.Printf("Restarting deployment %s/%s", deployment.Namespace, deployment.Name)
					if err := restartDeployment(deployment, *config.SlavePodNamespace); err != nil {
						log.Printf("Failed to restart deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
					}
				}
			}
		}

		// check if master pod appeared or became ready
		if (masterPod != nil && isReadyPod(masterPod) && isReadyNode(&node)) && !nodeInPool {
			if err := addNodeToPool(&node, *config.FailoverPoolLabel); err != nil {
				log.Printf("Failed to add node %s to failover pool: %v", node.Name, err)
			} else {
				log.Printf("Added node %s to failover pool", node.Name)
			}
		}

		if *config.LogEveryRun {
			// log every run
			log.Printf(logStr(&node, masterPod, slavePod))
		}
	}
}
