// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"context"
	"reflect"

	deployv1alpha1 "github.com/hybridapp-io/ham-deploy/pkg/apis/deploy/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	crdRootPath          = "/usr/local/etc/hybridapp/crds/"
	crdDeployableSubPath = "core/deployable"
	crdAssemblerSubPath  = "tools/assembler"
	crdDiscovererSubPath = "tools/discoverer"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Operator Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileOperator{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("deployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Operator
	err = c.Watch(&source.Kind{Type: &deployv1alpha1.Operator{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner Operator
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &deployv1alpha1.Operator{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileOperator implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileOperator{}

// ReconcileOperator reconciles a Operator object
type ReconcileOperator struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Operator object and makes changes based on the state read
// and what is in the Operator.Spec
func (r *ReconcileOperator) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	klog.Info("Reconciling Operator: ", request)

	// Fetch the Operator instance
	instance := &deployv1alpha1.Operator{}

	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue

			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// License must be accepted
	if !instance.Spec.LicenseSpec.Accept {
		klog.Error("spec.license.accept = false")

		instance.Status.Phase = deployv1alpha1.PhaseError
		instance.Status.Message = "License was not accepted"
		instance.Status.Reason = "LicenseAcceptFalse"
		updateErr := r.client.Status().Update(context.TODO(), instance)
		if updateErr != nil {
			klog.Error("Failed to update status: ", updateErr)
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	instance.Status.Phase = deployv1alpha1.PhasePending
	instance.Status.Message = "License was accepted"
	instance.Status.Reason = "LicenseAcceptTrue"
	updateErr := r.client.Status().Update(context.TODO(), instance)
	if updateErr != nil {
		klog.Error("Failed to update status: ", updateErr)
		return reconcile.Result{}, err
	}

	// Define a new Pod object
	pod := r.newPodForCR(instance)

	// Set Operator instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)

	if err != nil && errors.IsNotFound(err) {
		klog.Info("Creating a new Pod Namespace:", pod.Namespace, " Name:", pod.Name)
		err = r.client.Create(context.TODO(), pod)

		if err != nil {
			klog.Error("Failed to create new pod, error:", err)
			return reconcile.Result{}, err
		}

		instance.Status.Phase = deployv1alpha1.PhaseInstalled
		instance.Status.Message = ""
		instance.Status.Reason = ""
		updateErr := r.client.Status().Update(context.TODO(), instance)
		if updateErr != nil {
			klog.Error("Failed to update status: ", updateErr)
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Pod already exists - try to update
	uptodate := isEqualPod(found, pod)

	if !uptodate {
		err = r.client.Delete(context.TODO(), found)
		if err != nil {
			klog.Error("Failed to delete existing pod with error:", err)
		}

		return reconcile.Result{}, err
	}

	// update deployment status
	instance.Status.PodStatus = found.Status.DeepCopy()
	err = r.client.Status().Update(context.TODO(), found)

	return reconcile.Result{}, err
}

func (r *ReconcileOperator) createBasicPod(cr *deployv1alpha1.Operator) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
		},
	}

	pod.Spec.ServiceAccountName = deployv1alpha1.DefaultPodServiceAccountName

	// inherit operator settings if possible
	opns, err := k8sutil.GetOperatorNamespace()
	if err == nil {
		oppod, err := k8sutil.GetPod(context.TODO(), r.client, opns)

		if err == nil {
			oppod.Spec.Containers = nil
			oppod.Spec.DeepCopyInto(&pod.Spec)
		}
	}

	return pod
}

func (r *ReconcileOperator) configPodByCoreSpec(spec *deployv1alpha1.CoreSpec, pod *corev1.Pod) *corev1.Pod {
	var exists, implied bool

	// add deployable container unless spec.CoreSpec.DeployableOperatorSpec.Enabled = false
	exists = spec != nil && spec.DeployableOperatorSpec != nil
	implied = spec == nil || spec.DeployableOperatorSpec == nil || spec.DeployableOperatorSpec.Enabled == nil

	if implied || *(spec.DeployableOperatorSpec.Enabled) {
		var dospec *deployv1alpha1.DeployableOperatorSpec

		if exists {
			dospec = spec.DeployableOperatorSpec
		} else {
			dospec = &deployv1alpha1.DeployableOperatorSpec{}
		}

		pod.Spec.Containers = append(pod.Spec.Containers, *r.generateDeployableContainer(dospec, pod))
	}

	return pod
}

func (r *ReconcileOperator) configPodByToolsSpec(spec *deployv1alpha1.ToolsSpec, pod *corev1.Pod) *corev1.Pod {
	var exists, implied bool

	// add assembler container unless spec.ToolsSpec.ApplicationAssemblerSpec.Enabled = false
	exists = spec != nil && spec.ApplicationAssemblerSpec != nil
	implied = spec == nil || spec.ApplicationAssemblerSpec == nil || spec.ApplicationAssemblerSpec.Enabled == nil

	if implied || *(spec.ApplicationAssemblerSpec.Enabled) {
		var aaspec *deployv1alpha1.ApplicationAssemblerSpec

		if exists {
			aaspec = spec.ApplicationAssemblerSpec
		} else {
			aaspec = &deployv1alpha1.ApplicationAssemblerSpec{}
		}

		pod.Spec.Containers = append(pod.Spec.Containers, *r.generateAssemblerContainer(aaspec, pod))
	}

	// add discoverer container only if spec.ToolsSpec.ResourceDiscovererSpec.Enabled =
	exists = spec != nil && spec.ResourceDiscovererSpec != nil && spec.ResourceDiscovererSpec.Enabled != nil

	if exists && *(spec.ResourceDiscovererSpec.Enabled) {
		rdspec := spec.ResourceDiscovererSpec

		pod.Spec.Containers = append(pod.Spec.Containers, *r.generateDiscovererContainer(rdspec, pod))
	}

	return pod
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func (r *ReconcileOperator) newPodForCR(cr *deployv1alpha1.Operator) *corev1.Pod {
	pod := r.createBasicPod(cr)

	pod = r.configPodByCoreSpec(cr.Spec.CoreSpec, pod)
	pod = r.configPodByToolsSpec(cr.Spec.ToolsSpec, pod)

	return pod
}

func isEqualPod(oldpod, newpod *corev1.Pod) bool {
	if !isEqualVolumes(oldpod.Spec.Volumes, newpod.Spec.Volumes) {
		return false
	}

	// compare containers
	oldctnmap := make(map[string]*corev1.Container)
	for _, ctn := range oldpod.Spec.Containers {
		oldctnmap[ctn.Name] = ctn.DeepCopy()
	}

	for _, ctn := range newpod.Spec.Containers {
		octn, ok := oldctnmap[ctn.Name]
		if !ok {
			return false
		}

		if !isEqualContainer(octn, ctn.DeepCopy()) {
			return false
		}

		delete(oldctnmap, ctn.Name)
	}

	return len(oldctnmap) == 0
}

func isEqualVolumes(oldvols, newvols []corev1.Volume) bool {
	// compare volumns
	volmap := make(map[string]*corev1.Volume)
	for _, vol := range oldvols {
		volmap[vol.Name] = vol.DeepCopy()
	}

	for _, vol := range newvols {
		if oldvol, ok := volmap[vol.Name]; !ok {
			return false
		} else if !reflect.DeepEqual(*oldvol, vol) {
			return false
		}

		delete(volmap, vol.Name)
	}

	// if all new volumes are added, we're good. ignore the volumes generated by system
	return true
}

func isEqualContainer(oldctn, newctn *corev1.Container) bool {
	if (oldctn == newctn) || (oldctn == nil && newctn == nil) {
		return true
	}

	if oldctn == nil || newctn == nil {
		return false
	}

	if oldctn.Name != newctn.Name {
		return false
	}

	if oldctn.Image != newctn.Image {
		return false
	}

	if !isEqualStringArray(oldctn.Command, newctn.Command) {
		return false
	}

	volmtmap := make(map[string]*corev1.VolumeMount)
	for _, volm := range oldctn.VolumeMounts {
		volmtmap[volm.Name] = volm.DeepCopy()
	}

	for _, volm := range newctn.VolumeMounts {
		if oldvolm, ok := volmtmap[volm.Name]; !ok {
			return false
		} else if !reflect.DeepEqual(oldvolm, volm) {
			return false
		}
	}

	return isEqualStringArray(oldctn.Args, newctn.Args)
}

func isEqualStringArray(sa1, sa2 []string) bool {
	if sa1 == nil && sa2 == nil {
		return true
	}

	if sa1 == nil || sa2 == nil {
		return false
	}

	samap1 := make(map[string]string)
	for _, s := range sa1 {
		samap1[s] = s
	}

	for _, s := range sa2 {
		if _, ok := samap1[s]; !ok {
			return false
		}

		delete(samap1, s)
	}

	return len(samap1) == 0
}
