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
	deployv1alpha1 "github.com/hybridapp-io/ham-deploy/pkg/apis/deploy/v1alpha1"
	"github.com/hybridapp-io/ham-deploy/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func (r *ReconcileOperator) configContainerByGenericSpec(spec *deployv1alpha1.GenericContainerSpec, ctn *corev1.Container) *corev1.Container {
	if spec == nil {
		return ctn
	}

	// apply new values from cr if not nil
	if spec.Image != nil {
		ctn.Image = *spec.Image
	}

	if spec.Name != nil {
		ctn.Name = *spec.Name
	}

	if spec.ImagePullPolicy != nil {
		ctn.ImagePullPolicy = *spec.ImagePullPolicy
	}

	if spec.Resources != nil {
		ctn.Resources = *spec.Resources
	}

	if spec.Command != nil {
		ctn.Command = spec.Command
	}

	if spec.Args != nil {
		ctn.Args = spec.Args
	}

	runAsNonRoot := true
	ctn.SecurityContext = &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		RunAsNonRoot: &runAsNonRoot,
	}

	return ctn
}

func (r *ReconcileOperator) generateDeployableContainer(spec *deployv1alpha1.DeployableOperatorSpec) *corev1.Container {
	if spec == nil {
		return nil
	}

	envwatchns := corev1.EnvVar{Name: deployv1alpha1.ContainerEnvVarKeyWATCHNAMESPACE, Value: ""}
	envpn := corev1.EnvVar{
		Name: deployv1alpha1.ContainerEnvVarKeyPODNAME, ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	}
	envon := corev1.EnvVar{Name: deployv1alpha1.ContainerEnvVarKeyOPERATORNAME, Value: deployv1alpha1.DefaultDeployableContainerName}

	ctn := &corev1.Container{
		Name:            deployv1alpha1.DefaultDeployableContainerName,
		Image:           deployv1alpha1.DefaultDeployableContainerImage,
		ImagePullPolicy: deployv1alpha1.DefaultDeployableContainerImagePullPolicy,
		Resources:       deployv1alpha1.DefaultDeployableContainerResources,
		Command:         deployv1alpha1.DefaultDeployableContainerCommand,
		Env: []corev1.EnvVar{
			envwatchns,
			envpn,
			envon,
		},
	}

	// install crds for deployable operator if missing
	_ = utils.CheckAndInstallCRDs(r.dynamicClient, crdRootPath+crdDeployableSubPath)

	ctn = r.configContainerByGenericSpec(&spec.GenericContainerSpec, ctn)

	return ctn
}

func (r *ReconcileOperator) generateAssemblerContainer(spec *deployv1alpha1.ApplicationAssemblerSpec) *corev1.Container {
	envwatchns := corev1.EnvVar{Name: deployv1alpha1.ContainerEnvVarKeyWATCHNAMESPACE, Value: ""}
	envpn := corev1.EnvVar{
		Name: deployv1alpha1.ContainerEnvVarKeyPODNAME, ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	}
	envon := corev1.EnvVar{Name: deployv1alpha1.ContainerEnvVarKeyOPERATORNAME, Value: deployv1alpha1.DefaultAssemblerContainerName}

	ctn := &corev1.Container{
		Name:            deployv1alpha1.DefaultAssemblerContainerName,
		Image:           deployv1alpha1.DefaultAssemblerContainerImage,
		ImagePullPolicy: deployv1alpha1.DefaultAssemblerContainerImagePullPolicy,
		Resources:       deployv1alpha1.DefaultAssemblerContainerResources,
		Command:         deployv1alpha1.DefaultAssemblerContainerCommand,
		Env: []corev1.EnvVar{
			envwatchns,
			envpn,
			envon,
		},
	}

	// install crds for deployable operator if missing
	_ = utils.CheckAndInstallCRDs(r.dynamicClient, crdRootPath+crdAssemblerSubPath)

	ctn = r.configContainerByGenericSpec(&spec.GenericContainerSpec, ctn)

	return ctn
}

func (r *ReconcileOperator) generateDiscovererContainer(spec *deployv1alpha1.ResourceDiscovererSpec, rs *appsv1.ReplicaSet) *corev1.Container {
	envwatchns := corev1.EnvVar{Name: deployv1alpha1.ContainerEnvVarKeyWATCHNAMESPACE, Value: ""}
	envpn := corev1.EnvVar{
		Name: deployv1alpha1.ContainerEnvVarKeyPODNAME, ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	}

	envon := corev1.EnvVar{Name: deployv1alpha1.ContainerEnvVarKeyOPERATORNAME, Value: deployv1alpha1.DefaultDiscovererContainerName}

	// apply required env var cluster name and cluster namespace
	var cn, cns string
	if spec.ClusterName != nil {
		cn = *spec.ClusterName
	}
	envcn := corev1.EnvVar{Name: deployv1alpha1.ContainerEnvVarKeyCLUSTERNAME, Value: cn}

	if spec.ClusterNamespace != nil {
		cns = *spec.ClusterNamespace
	}
	envcns := corev1.EnvVar{Name: deployv1alpha1.ContainerEnvVarKeyCLUSTERNAMESPACE, Value: cns}

	ctn := &corev1.Container{
		Name:            deployv1alpha1.DefaultDiscovererContainerName,
		Image:           deployv1alpha1.DefaultDiscovererContainerImage,
		ImagePullPolicy: deployv1alpha1.DefaultDiscovererContainerImagePullPolicy,
		Resources:       deployv1alpha1.DefaultDiscovererContainerResources,
		Command:         deployv1alpha1.DefaultDiscovererContainerCommand,
		Env: []corev1.EnvVar{
			envwatchns,
			envpn,
			envon,
			envcn,
			envcns,
		},
	}

	// install crds for deployable operator if missing
	_ = utils.CheckAndInstallCRDs(r.dynamicClient, crdRootPath+crdDiscovererSubPath)

	// apply new values from cr if not nil
	ctn = r.configContainerByGenericSpec(&spec.GenericContainerSpec, ctn)

	// hub config setting
	if spec.HubConnectionConfig != nil {
		// kubeconfig
		if spec.HubConnectionConfig.KubeConfig != nil {
			envkc := corev1.EnvVar{
				Name:  deployv1alpha1.ContainerEnvVarKeyHUBKUBECONFIG,
				Value: *spec.HubConnectionConfig.KubeConfig,
			}
			ctn.Env = append(ctn.Env, envkc)
		}

		if spec.HubConnectionConfig != nil {
			// set up volume
			secvol := corev1.Volume{
				Name: deployv1alpha1.DefaultPodVolumeNameHubConnection,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: spec.HubConnectionConfig.SecretRef.Name,
					},
				},
			}
			rs.Spec.Template.Spec.Volumes = append(rs.Spec.Template.Spec.Volumes, secvol)
			// set up mount

			volm := corev1.VolumeMount{
				MountPath: spec.HubConnectionConfig.MountPath,
				Name:      secvol.Name,
			}
			ctn.VolumeMounts = append(ctn.VolumeMounts, volm)
		}
	}

	return ctn
}
