/*
Copyright 2022 The Kubernetes Authors.

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

package workload

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	kueue "sigs.k8s.io/kueue/api/v1alpha1"
)

func TestPodRequests(t *testing.T) {
	cases := map[string]struct {
		spec         corev1.PodSpec
		wantRequests Requests
	}{
		"core": {
			spec: corev1.PodSpec{
				Containers: containersForRequests(
					map[corev1.ResourceName]string{
						corev1.ResourceCPU:    "10m",
						corev1.ResourceMemory: "1Ki",
					},
					map[corev1.ResourceName]string{
						corev1.ResourceCPU:              "5m",
						corev1.ResourceEphemeralStorage: "1Ki",
					},
				),
				InitContainers: containersForRequests(
					map[corev1.ResourceName]string{
						corev1.ResourceCPU:    "10m",
						corev1.ResourceMemory: "1Ki",
					},
					map[corev1.ResourceName]string{
						corev1.ResourceMemory: "2Ki",
					},
				),
				Overhead: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("0.1"),
				},
			},
			wantRequests: Requests{
				corev1.ResourceCPU:              115,
				corev1.ResourceMemory:           2048,
				corev1.ResourceEphemeralStorage: 1024,
			},
		},
		"extended": {
			spec: corev1.PodSpec{
				Containers: containersForRequests(
					map[corev1.ResourceName]string{
						"ex.com/gpu": "2",
					},
					map[corev1.ResourceName]string{
						"ex.com/gpu": "1",
					},
				),
				InitContainers: containersForRequests(
					map[corev1.ResourceName]string{
						"ex.com/ssd": "1",
					},
					map[corev1.ResourceName]string{
						"ex.com/gpu": "1",
						"ex.com/ssd": "1",
					},
				),
			},
			wantRequests: Requests{
				"ex.com/gpu": 3,
				"ex.com/ssd": 1,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotRequests := podRequests(&tc.spec)
			if diff := cmp.Diff(tc.wantRequests, gotRequests); diff != "" {
				t.Errorf("podRequests returned unexpected requests (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestNewInfo(t *testing.T) {
	qw := &kueue.QueuedWorkload{
		Spec: kueue.QueuedWorkloadSpec{
			Pods: []kueue.PodSet{
				{
					Name: "driver",
					Spec: corev1.PodSpec{
						Containers: containersForRequests(
							map[corev1.ResourceName]string{
								corev1.ResourceCPU:    "10m",
								corev1.ResourceMemory: "512Ki",
							}),
					},
					Count: 1,
					AssignedFlavors: map[corev1.ResourceName]string{
						corev1.ResourceCPU: "on-demand",
					},
				},
				{
					Name: "workers",
					Spec: corev1.PodSpec{
						Containers: containersForRequests(
							map[corev1.ResourceName]string{
								corev1.ResourceCPU:    "5m",
								corev1.ResourceMemory: "1Mi",
								"ex.com/gpu":          "1",
							}),
					},
					Count: 3,
				},
			},
		},
	}
	info := NewInfo(qw)
	wantRequests := []PodSetResources{
		{
			Name: "driver",
			Requests: Requests{
				corev1.ResourceCPU:    10,
				corev1.ResourceMemory: 512 * 1024,
			},
			Flavors: map[corev1.ResourceName]string{
				corev1.ResourceCPU: "on-demand",
			},
		},
		{
			Name: "workers",
			Requests: Requests{
				corev1.ResourceCPU:    15,
				corev1.ResourceMemory: 3 * 1024 * 1024,
				"ex.com/gpu":          3,
			},
		},
	}
	if diff := cmp.Diff(info.TotalRequests, wantRequests); diff != "" {
		t.Errorf("NewInfo returned unexpected total requests (-want,+got):\n%s", diff)
	}
}

func containersForRequests(requests ...map[corev1.ResourceName]string) []corev1.Container {
	containers := make([]corev1.Container, len(requests))
	for i, r := range requests {
		rl := make(corev1.ResourceList, len(r))
		for name, val := range r {
			rl[name] = resource.MustParse(val)
		}
		containers[i].Resources = corev1.ResourceRequirements{
			Requests: rl,
		}
	}
	return containers
}