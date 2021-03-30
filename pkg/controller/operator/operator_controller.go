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

	deployv1alpha1 "github.com/hybridapp-io/ham-deploy/pkg/apis/deploy/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/dynamic"
	"k8s.io/klog"

	controllerruntime "sigs.k8s.io/controller-runtime"
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
	crdPlacementSubPath  = "core/placement"
	crdAssemblerSubPath  = "tools/assembler"
	crdDiscovererSubPath = "tools/discoverer"
)

// Add creates a new Operator Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	reconciler := &ReconcileOperator{client: mgr.GetClient(), scheme: mgr.GetScheme()}
	reconciler.dynamicClient = dynamic.NewForConfigOrDie(mgr.GetConfig())
	return reconciler
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

	// Watch for changes to secondary resource ReplicaSet and requeue the owner Operator
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
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
	dynamicClient dynamic.Interface
	client        client.Client
	scheme        *runtime.Scheme
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

	if instance.Status.Phase == "" {
		instance.Status.Phase = deployv1alpha1.PhasePending
	}

	// License must be accepted
	if !instance.Spec.LicenseSpec.Accept {
		klog.Warning("License was not accepted. (spec.license.accept = false)")

		instance.Status.Phase = deployv1alpha1.PhaseError
		instance.Status.Message = "License was not accepted"
		instance.Status.Reason = "LicenseAcceptFalse"
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			klog.Error("Failed to update status: ", err)
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// Reconcile Deployment
	deployment := &appsv1.Deployment{}
	deployment.Name = instance.Name
	deployment.Namespace = instance.Namespace
	result, err := controllerruntime.CreateOrUpdate(context.TODO(), r.client, deployment, func() error {
		r.mutateDeployment(instance, deployment)
		return controllerutil.SetControllerReference(instance, deployment, r.scheme)
	})
	if err != nil {
		klog.Error("Failed to reconcile Deployment: ", deployment.Namespace, "/", deployment.Name, ", error: ", err)
		return reconcile.Result{}, err
	}
	if result != controllerutil.OperationResultNone {
		klog.Info("Reconciled Deployment: ", deployment.Namespace, "/", deployment.Name, ", result: ", result)
	}

	// update instance status
	if instance.Status.Phase != deployv1alpha1.PhaseInstalled {
		instance.Status.Phase = deployv1alpha1.PhaseInstalled
		instance.Status.Message = ""
		instance.Status.Reason = ""
	}
	instance.Status.DeploymentStatus = deployment.Status.DeepCopy()
	if err = r.client.Status().Update(context.TODO(), instance); err != nil {
		klog.Error("Failed to update status: ", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileOperator) mutateDeployment(cr *deployv1alpha1.Operator, deployment *appsv1.Deployment) {
	if cr.Spec.Replicas == nil {
		deployment.Spec.Replicas = &deployv1alpha1.DefaultReplicas
	} else {
		deployment.Spec.Replicas = cr.Spec.Replicas
	}

	deployment.Spec.Template.Name = cr.Name
	deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{"app": cr.Name},
	}

	if cr.Labels == nil {
		deployment.Spec.Template.Labels = map[string]string{"app": cr.Name}
	} else {
		deployment.Spec.Template.Labels = cr.Labels
		deployment.Spec.Template.Labels["app"] = cr.Name
	}

	if cr.Annotations != nil {
		deployment.Spec.Template.Annotations = cr.Annotations
	}

	deployment.Spec.Template.Spec.ServiceAccountName = deployv1alpha1.DefaultServiceAccountName

	// inherit operator imagePullSecret, if available
	opns, err := k8sutil.GetOperatorNamespace()
	if err == nil {
		oppod, err := k8sutil.GetPod(context.TODO(), r.client, opns)

		if err == nil {
			ps := oppod.Spec.ImagePullSecrets
			for _, s := range ps {
				if !contains(deployment.Spec.Template.Spec.ImagePullSecrets, s) {
					deployment.Spec.Template.Spec.ImagePullSecrets = append(deployment.Spec.Template.Spec.ImagePullSecrets, s)
				}
			}
		}
	}

	deployment.Spec.Template.Spec.Containers = nil
	r.configPodByCoreSpec(cr.Spec.CoreSpec, deployment)
	r.configPodByToolsSpec(cr.Spec.ToolsSpec, deployment)
}

func (r *ReconcileOperator) configPodByCoreSpec(spec *deployv1alpha1.CoreSpec, deployment *appsv1.Deployment) {
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
		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *r.generateDeployableContainer(dospec))
	}

	// add placement container unless spec.CoreSpec.PlacementSpec.Enabled = false
	exists = spec != nil && spec.PlacementSpec != nil
	implied = spec == nil || spec.PlacementSpec == nil || spec.PlacementSpec.Enabled == nil

	if implied || *(spec.PlacementSpec.Enabled) {
		var pspec *deployv1alpha1.PlacementSpec

		if exists {
			pspec = spec.PlacementSpec
		} else {
			pspec = &deployv1alpha1.PlacementSpec{}
		}
		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *r.generatePlacementContainer(pspec))
	}
}

func (r *ReconcileOperator) configPodByToolsSpec(spec *deployv1alpha1.ToolsSpec, deployment *appsv1.Deployment) {
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
		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *r.generateAssemblerContainer(aaspec))
	}

	// add discoverer container only if spec.ToolsSpec.ResourceDiscovererSpec.Enabled =
	exists = spec != nil && spec.ResourceDiscovererSpec != nil && spec.ResourceDiscovererSpec.Enabled != nil

	if exists && *(spec.ResourceDiscovererSpec.Enabled) {
		rdspec := spec.ResourceDiscovererSpec

		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *r.generateDiscovererContainer(rdspec, deployment))
	}
}

//Clean up old replicaset
func (r *ReconcileOperator) removeLegacyReplicaSet(cr *deployv1alpha1.Operator) {
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
	}

	found := &appsv1.ReplicaSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return //No legacy instance
		} 

		err = r.client.Delete(context.TODO(), found)
		if err != nil {
			klog.Error("Failed to delete legacy replica, error:", err)
		}
}

func contains(s []v1.LocalObjectReference, e v1.LocalObjectReference) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
