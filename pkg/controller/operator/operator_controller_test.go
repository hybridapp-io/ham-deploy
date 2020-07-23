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
	"testing"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	deployv1alpha1 "github.com/hybridapp-io/ham-deploy/pkg/apis/deploy/v1alpha1"
)

const interval = time.Second * 1

var (
	request = types.NamespacedName{
		Name:      "test",
		Namespace: "default",
	}

	acceptLicense = deployv1alpha1.LicenseSpec{
		Accept: true,
	}

	refuseLicense = deployv1alpha1.LicenseSpec{
		Accept: false,
	}

	defaultContainerNumber = 2
	single                 = 1
	falsevalue             = false
	truevalue              = true
	clusterName            = "default"
	clusterNameSpace       = "default"
)

func TestReconcile(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	rec := newReconciler(mgr)
	g.Expect(add(mgr, rec)).To(Succeed())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	deploy := &deployv1alpha1.Operator{}
	deploy.Name = request.Name
	deploy.Namespace = request.Namespace
	deploy.Spec.LicenseSpec = &acceptLicense

	g.Expect(c.Create(context.TODO(), deploy)).To(Succeed())

	rs := &appsv1.ReplicaSet{}
	rsKey := types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}

	time.Sleep(interval)
	g.Expect(c.Get(context.TODO(), rsKey, rs)).To(Succeed())
	g.Expect(len(rs.Spec.Template.Spec.Containers) == defaultContainerNumber).To(BeTrue())

	// delete replicaset should trigger recreation
	g.Expect(c.Delete(context.TODO(), rs)).To(Succeed())
	time.Sleep(interval)
	g.Expect(c.Get(context.TODO(), rsKey, rs)).To(Succeed())

	// delete the deploy first
	g.Expect(c.Get(context.TODO(), request, deploy)).To(Succeed())
	g.Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
	time.Sleep(interval)

	err = c.Get(context.TODO(), request, deploy)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())

	// test api server does not respect delete by ownerreference, so replicaset won't be automatically deleted
	// can only test delete replicaset after deploy wont trigger recreation
	g.Expect(c.Delete(context.TODO(), rs)).To(Succeed())
	time.Sleep(interval)

	err = c.Get(context.TODO(), rsKey, rs)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())
}

func TestDiscoverer(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	rec := newReconciler(mgr)
	g.Expect(add(mgr, rec)).To(Succeed())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// disable deployable container and assembler container to focus on discoverer
	deploy := &deployv1alpha1.Operator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Name,
			Namespace: request.Namespace,
		},
		Spec: deployv1alpha1.OperatorSpec{
			LicenseSpec: &acceptLicense,
			CoreSpec: &deployv1alpha1.CoreSpec{
				DeployableOperatorSpec: &deployv1alpha1.DeployableOperatorSpec{
					GenericContainerSpec: deployv1alpha1.GenericContainerSpec{
						Enabled: &falsevalue,
					},
				},
			},
			ToolsSpec: &deployv1alpha1.ToolsSpec{
				ApplicationAssemblerSpec: &deployv1alpha1.ApplicationAssemblerSpec{
					GenericContainerSpec: deployv1alpha1.GenericContainerSpec{
						Enabled: &falsevalue,
					},
				},
				ResourceDiscovererSpec: &deployv1alpha1.ResourceDiscovererSpec{
					GenericContainerSpec: deployv1alpha1.GenericContainerSpec{
						Enabled: &truevalue,
					},
					ClusterName:      &clusterName,
					ClusterNamespace: &clusterNameSpace,
				},
			},
		},
	}

	g.Expect(c.Create(context.TODO(), deploy)).To(Succeed())

	rs := &appsv1.ReplicaSet{}
	rsKey := types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}

	time.Sleep(interval)
	g.Expect(c.Get(context.TODO(), rsKey, rs)).To(Succeed())
	g.Expect(len(rs.Spec.Template.Spec.Containers) == single).To(BeTrue())

	// delete the deploy first
	g.Expect(c.Get(context.TODO(), request, deploy)).To(Succeed())
	g.Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
	time.Sleep(interval)

	err = c.Get(context.TODO(), request, deploy)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())

	g.Expect(c.Delete(context.TODO(), rs)).To(Succeed())
	time.Sleep(interval)

	err = c.Get(context.TODO(), rsKey, rs)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())
}

func TestRefuseLicense(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	rec := newReconciler(mgr)
	g.Expect(add(mgr, rec)).To(Succeed())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	deploy := &deployv1alpha1.Operator{}
	deploy.Name = request.Name
	deploy.Namespace = request.Namespace
	deploy.Spec.LicenseSpec = &refuseLicense

	g.Expect(c.Create(context.TODO(), deploy)).To(Succeed())

	rs := &appsv1.ReplicaSet{}
	rsKey := types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}

	err = c.Get(context.TODO(), rsKey, rs)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())

	g.Expect(c.Get(context.TODO(), request, deploy)).To(Succeed())
	g.Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
	time.Sleep(interval)

	err = c.Get(context.TODO(), request, deploy)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())
}

func TestPodChange(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	rec := newReconciler(mgr)
	g.Expect(add(mgr, rec)).To(Succeed())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	deploy := &deployv1alpha1.Operator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Name,
			Namespace: request.Namespace,
		},
		Spec: deployv1alpha1.OperatorSpec{
			LicenseSpec: &acceptLicense,
			CoreSpec: &deployv1alpha1.CoreSpec{
				DeployableOperatorSpec: &deployv1alpha1.DeployableOperatorSpec{
					GenericContainerSpec: deployv1alpha1.GenericContainerSpec{
						Enabled: &truevalue,
					},
				},
			},
			ToolsSpec: &deployv1alpha1.ToolsSpec{
				ApplicationAssemblerSpec: &deployv1alpha1.ApplicationAssemblerSpec{
					GenericContainerSpec: deployv1alpha1.GenericContainerSpec{
						Enabled: &truevalue,
					},
				},
			},
		},
	}

	g.Expect(c.Create(context.TODO(), deploy)).To(Succeed())

	rs := &appsv1.ReplicaSet{}
	rsKey := types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}

	time.Sleep(interval)
	g.Expect(c.Get(context.TODO(), rsKey, rs)).To(Succeed())
	g.Expect(len(rs.Spec.Template.Spec.Containers) == defaultContainerNumber).To(BeTrue())

	//disable a container
	g.Expect(c.Get(context.TODO(), request, deploy)).To(Succeed())
	deploy.Spec.CoreSpec.DeployableOperatorSpec.GenericContainerSpec.Enabled = &falsevalue
	g.Expect(c.Update(context.TODO(), deploy)).To(Succeed())

	time.Sleep(interval)
	g.Expect(c.Get(context.TODO(), rsKey, rs)).To(Succeed())
	g.Expect(len(rs.Spec.Template.Spec.Containers) == single).To(BeTrue())

	// delete the deploy first
	g.Expect(c.Get(context.TODO(), request, deploy)).To(Succeed())
	g.Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
	time.Sleep(interval)

	err = c.Get(context.TODO(), request, deploy)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())

	g.Expect(c.Delete(context.TODO(), rs)).To(Succeed())
	time.Sleep(interval)

	err = c.Get(context.TODO(), rsKey, rs)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())
}

func TestReplicaNumberChange(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	rec := newReconciler(mgr)
	g.Expect(add(mgr, rec)).To(Succeed())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	deploy := &deployv1alpha1.Operator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Name,
			Namespace: request.Namespace,
		},
		Spec: deployv1alpha1.OperatorSpec{
			LicenseSpec: &acceptLicense,
			CoreSpec: &deployv1alpha1.CoreSpec{
				DeployableOperatorSpec: &deployv1alpha1.DeployableOperatorSpec{
					GenericContainerSpec: deployv1alpha1.GenericContainerSpec{
						Enabled: &truevalue,
					},
				},
			},
			ToolsSpec: &deployv1alpha1.ToolsSpec{
				ApplicationAssemblerSpec: &deployv1alpha1.ApplicationAssemblerSpec{
					GenericContainerSpec: deployv1alpha1.GenericContainerSpec{
						Enabled: &truevalue,
					},
				},
			},
		},
	}

	g.Expect(c.Create(context.TODO(), deploy)).To(Succeed())

	rs := &appsv1.ReplicaSet{}
	rsKey := types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}

	time.Sleep(interval)
	g.Expect(c.Get(context.TODO(), rsKey, rs)).To(Succeed())
	g.Expect(*rs.Spec.Replicas == deployv1alpha1.DefaultReplicas).To(BeTrue())

	//increase replica count
	replicas := int32(3)
	g.Expect(c.Get(context.TODO(), request, deploy)).To(Succeed())
	deploy.Spec.Replicas = &replicas
	g.Expect(c.Update(context.TODO(), deploy)).To(Succeed())

	time.Sleep(interval)
	g.Expect(c.Get(context.TODO(), rsKey, rs)).To(Succeed())
	g.Expect(*rs.Spec.Replicas == replicas).To(BeTrue())

	// delete the deploy first
	g.Expect(c.Get(context.TODO(), request, deploy)).To(Succeed())
	g.Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
	time.Sleep(interval)

	err = c.Get(context.TODO(), request, deploy)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())

	g.Expect(c.Delete(context.TODO(), rs)).To(Succeed())
	time.Sleep(interval)

	err = c.Get(context.TODO(), rsKey, rs)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())
}
