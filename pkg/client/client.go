package client

import (
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	clientset *kubernetes.Clientset
	config    *rest.Config
	err       error
)

func GetClient() *kubernetes.Clientset {
	config, err = rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error reading in-cluster k8s config: %v", err)
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating client from in-cluster config: %v", err)
	}
	return clientset
}
