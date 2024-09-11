/*
Copyright 2024 Bougou.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"

	ctutils "github.com/bougou/go-container-utils"
	"github.com/bougou/iftop-exporter/iftop-exporter-k8s-helper/internal/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// registeredPodInterfaces holds interface names of each pod (belong to the node) when pod becomes running and be cleaned when pod deleted.
//
// key is podKey, value is list of interfaces (node-side) of the pod.
var registeredPodInterfaces = make(map[string][]string)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Rootfs     string
	Selectors  utils.Selectors
	DynamicDir string
}

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Pod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// TODO(user): your logic here

	pod := &corev1.Pod{}
	podKey := podKeyFromReq(req)

	if err := r.Get(ctx, req.NamespacedName, pod); err != nil {
		// log.Error(err, "unable to fetch pod")

		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "unable to fetch Pod")
			return ctrl.Result{}, err
		}

		// Now `client.IgnoreNotFound` return nil, means the object has been deleted
		log.Info(fmt.Sprintf("Pod deleted (%s)", podKey))
		if err := r.cleanInterfaces(podKey); err != nil {
			log.Error(err, "clear pod interfaces")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !r.Selectors.Hit(pod.Labels) {
		log.V(2).Info(fmt.Sprintf("pod ignored (%s)", podKey))
		return ctrl.Result{}, nil
	}

	if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info(fmt.Sprintf("Pod deleting (%s)", podKey))
		if err := r.cleanInterfaces(podKey); err != nil {
			log.Error(err, "clear pod interfaces")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if _, ok := registeredPodInterfaces[podKey]; ok {
		log.V(2).Info(fmt.Sprintf("pod already handled (%s)", podKey))
		return ctrl.Result{}, nil
	}

	if !utils.IsPodRunning(pod) && !utils.IsPodTerminating(pod) {
		log.V(2).Info(fmt.Sprintf("pod status is not Running or Terminating (%s) (%s)", podKey, utils.PodStatus(pod)))
		return ctrl.Result{}, nil
	}

	nodeName := os.Getenv("NODE_NAME")
	if utils.PodNodeName(pod) != nodeName {
		log.V(2).Info(fmt.Sprintf("pod not on this node (%s) (%s)", podKey, nodeName))
		return ctrl.Result{}, nil
	}

	log.Info(fmt.Sprintf("pod phase status (%s) (%s)", podKey, utils.PodStatus(pod)))

	if err := r.setInterfaces(ctx, podKey, pod); err != nil {
		return ctrl.Result{}, fmt.Errorf("set interfaces failed, err: %s", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}

type InterfaceInfo struct {
	Owner                  string `json:"owner,omitempty"`
	ContainerInterfaceName string `json:"container_interface_name,omitempty"`
	NodeInterfaceName      string `json:"node_interface_name,omitempty"`
}

func podKeyFromReq(req ctrl.Request) string {
	return req.NamespacedName.String()
}

func (r *PodReconciler) setInterfaces(ctx context.Context, podKey string, pod *corev1.Pod) error {
	log := log.FromContext(ctx)

	var containerID string
	for _, containerStatus := range pod.Status.ContainerStatuses {
		containerID = containerStatus.ContainerID
		if containerID != "" {
			break
		}
	}

	if containerID == "" {
		log.V(2).Info(fmt.Sprintf("pod (%s) not found container id", podKey))
		return nil
	}

	container, err := ctutils.NewContainer(containerID)
	if err != nil {
		return fmt.Errorf("pod (%s) get container (%s) failed: %s", podKey, containerID, err)
	}
	container.WithHostRoot(r.Rootfs)

	interfacesMapping, err := container.GetInterfacesNodeMapping()
	if err != nil {
		return fmt.Errorf("pod (%s) get container interfaces node mapping (%s) failed: %s", podKey, containerID, err)
	}
	for interfaceContainer, interfaceNode := range interfacesMapping {
		interfaceInfo := InterfaceInfo{
			Owner:                  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
			ContainerInterfaceName: interfaceContainer,
			NodeInterfaceName:      interfaceNode,
		}

		v, err := json.MarshalIndent(interfaceInfo, "", "  ")
		if err != nil {
			return fmt.Errorf("pod json marshal failed (%s): %s", podKey, err)
		}
		v = append(v, '\n')

		fileName := filepath.Join(r.DynamicDir, interfaceNode)
		if err := os.WriteFile(fileName, v, os.ModePerm); err != nil {
			return fmt.Errorf("pod (%s) write interface info file (%s) failed: %s", podKey, fileName, err)
		}
		log.Info(fmt.Sprintf("write file (%s) succeeded", fileName))

		if _, exists := registeredPodInterfaces[podKey]; !exists {
			registeredPodInterfaces[podKey] = make([]string, 0)
		}

		registeredPodInterfaces[podKey] = append(registeredPodInterfaces[podKey], interfaceNode)
	}

	return nil
}

func (r *PodReconciler) cleanInterfaces(podKey string) error {
	interfaces, ok := registeredPodInterfaces[podKey]
	if !ok {
		return nil
	}

	for _, intf := range interfaces {
		fileName := filepath.Join(r.DynamicDir, intf)

		ok, err := checkFileExists(fileName)
		if err != nil {
			return fmt.Errorf("check file exists failed (%s), err: %s", fileName, err)
		}

		if ok {
			if err := os.Remove(fileName); err != nil {
				return fmt.Errorf("pod (%s) remove interface info file (%s) failed: %s", podKey, fileName, err)
			}
		}

		delete(registeredPodInterfaces, podKey)
	}

	return nil
}

func checkFileExists(item string) (bool, error) {
	info, err := os.Stat(item)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// item exists
	return !info.IsDir(), nil
}
