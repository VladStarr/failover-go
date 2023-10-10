package failover

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/vladstarr/failover/pkg/client"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	coreClient = client.GetClient().CoreV1()
	appsClient = client.GetClient().AppsV1()
)

// GetCurrentNamespace returns current namespace
func getCurrentNamespace() (string, error) {
	namespaceBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}
	return string(namespaceBytes), nil
}

// GetNodes returns list of nodes matching specified selector string
func getNodes(selector string) (*v1.NodeList, error) {
	nodeLabels, err := labels.Parse(selector)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse node labels: %v", err)
	}

	nodes, err := coreClient.Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: nodeLabels.String(),
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to get list of nodes: %v", err)
	}

	return nodes, nil
}

// isNodeInPool performs check if the node is in failover pool
func isNodeInPool(node *v1.Node, failoverLabel string) bool {
	readyKey := strings.Split(failoverLabel, "=")[0]
	if _, ok := node.Labels[readyKey]; ok {
		return true
	}
	return false
}

// isReadyNode returns true if node's kubelet reporting healthy status
func isReadyNode(node *v1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == v1.NodeReady && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// getMasterPod returns master pod if it is found on node
func getMasterPod(node *v1.Node, selector string) (*v1.Pod, error) {
	// ensure pod is running on node we are watching
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", node.Name)

	podLabels, err := labels.Parse(selector)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse master pod selector: %v", err)
	}

	namespace, err := getCurrentNamespace()
	if err != nil {
		return nil, fmt.Errorf("Failed to get current namespace: %v", err)
	}

	pods, err := coreClient.Pods(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fieldSelector,
		LabelSelector: podLabels.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to filter master pods: %v", err)
	}
	if len(pods.Items) == 0 {
		return nil, nil
	}

	return &pods.Items[0], nil
}

// getSlavePod returns slave pods to which failover mechanism is applied
func getSlavePod(node *v1.Node, namespace string, selector string, nodeSelector string) (*v1.Pod, error) {
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", node.Name)

	podLabels, err := labels.Parse(selector)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse slave pod selector: %v", err)
	}

	pods, err := coreClient.Pods(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fieldSelector,
		LabelSelector: podLabels.String(),
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to list slave pods: %v", err)
	}

	if len(pods.Items) == 0 {
		return nil, nil
	}

	// get pods that are actually using failover pool nodeSelector
	nodeSelectorKey := strings.Split(nodeSelector, "=")[0]
	for _, pod := range pods.Items {
		_, ok := pod.Spec.NodeSelector[nodeSelectorKey]
		if ok {
			return &pod, nil
		}
	}

	return nil, nil
}

// isReadyPod returns true if pod is not terminating and is ready, otherwise returns false
func isReadyPod(pod *v1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if pod.DeletionTimestamp == nil && c.Type == v1.PodReady && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// getDeploymentByPod returns deployment object based on pod ownerRef
func getDeploymentByPod(pod *v1.Pod, namespace string) (*appsv1.Deployment, error) {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			// get deployment name by trimming pod's pod-template-hash from it's ownerRef.Name
			podTemplateHash := pod.Labels["pod-template-hash"]
			deploymentName := strings.TrimSuffix(ownerRef.Name, fmt.Sprintf("-%s", podTemplateHash))

			deployment, err := appsClient.Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("Failed to get pod's deployment: %v", err)
			}
			return deployment, nil
		}
	}
	return nil, nil
}

// restartDeployment adds annotation to deployment object notifying controller to restart it
func restartDeployment(deployment *appsv1.Deployment, namespace string) error {

	annotations := deployment.Spec.Template.Annotations

	if annotations == nil {
		annotations = make(map[string]string)
	}

	// the same as `kubectl rollout restart`
	annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().String()

	if _, err := appsClient.Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("Failed to restart deployment: %v", err)
	}
	return nil
}

// addNodeToPool adds node to failover pool by labeling it
func addNodeToPool(node *v1.Node, failoverLabel string) error {
	key := strings.Split(failoverLabel, "=")[0]
	val := strings.Split(failoverLabel, "=")[1]

	node.Labels[key] = val

	if _, err := coreClient.Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("Error updating node: %v", err)
	}

	return nil
}

// removeNodeFromPool removes node from failover pool by removing label
func removeNodeFromPool(node *v1.Node, failoverLabel string) error {
	key := strings.Split(failoverLabel, "=")[0]

	if _, ok := node.Labels[key]; ok {
		delete(node.Labels, key)
	}

	_, err := coreClient.Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Error updating node: %v", err)
	}

	return nil
}

// logStr returns human-readable log for every run
func logStr(node *v1.Node, masterPod *v1.Pod, slavePod *v1.Pod) string {
	masterPodName := ""
	slavePodName := ""
	nodeName := ""
	ready := false

	if node != nil {
		nodeName = node.Name
	}
	if masterPod != nil {
		masterPodName = masterPod.Name
		ready = isReadyPod(masterPod) && isReadyNode(node)
	}
	if slavePod != nil {
		slavePodName = slavePod.Name
	}

	return fmt.Sprintf("node=%s masterPod=%s ready=%s slavePod=%s", nodeName, masterPodName, strconv.FormatBool(ready), slavePodName)
}
