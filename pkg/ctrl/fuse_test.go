/*
Copyright 2021 The Fluid Authors.

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

package ctrl

import (
	"context"
	"testing"

	datav1alpha1 "github.com/fluid-cloudnative/fluid/api/v1alpha1"
	"github.com/fluid-cloudnative/fluid/pkg/ddc/base"
	"github.com/fluid-cloudnative/fluid/pkg/utils/fake"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestCheckFuseHealthy(t *testing.T) {
	runtimeInputs := []*datav1alpha1.JindoRuntime{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hbase",
				Namespace: "fluid",
			},
			Spec: datav1alpha1.JindoRuntimeSpec{
				Replicas: 3, // 2
			},
			Status: datav1alpha1.RuntimeStatus{
				CurrentFuseNumberScheduled: 2,
				DesiredFuseNumberScheduled: 3,
				FusePhase:                  "Ready",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hadoop",
				Namespace: "fluid",
			},
			Spec: datav1alpha1.JindoRuntimeSpec{
				Replicas: 2,
			},
			Status: datav1alpha1.RuntimeStatus{
				CurrentFuseNumberScheduled: 3,
				DesiredFuseNumberScheduled: 2,

				FusePhase: "NotReady",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "obj",
				Namespace: "fluid",
			},
			Spec: datav1alpha1.JindoRuntimeSpec{
				Replicas: 2,
			},
			Status: datav1alpha1.RuntimeStatus{
				CurrentFuseNumberScheduled: 2,
				DesiredFuseNumberScheduled: 2,

				FusePhase: "NotReady",
			},
		},
	}

	podList := &corev1.PodList{
		Items: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hbase-jindofs-fuse-0",
				Namespace: "big-data",
				Labels:    map[string]string{"a": "b"},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodFailed,
				Conditions: []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}},
			},
		}},
	}

	dataSetInputs := []*datav1alpha1.Dataset{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hbase",
				Namespace: "fluid",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hadoop",
				Namespace: "fluid",
			},
		},
	}

	dsInputss := []*appsv1.DaemonSet{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hbase-jindofs-fuse",
				Namespace: "big-data",
			},
			Spec: appsv1.DaemonSetSpec{},
			Status: appsv1.DaemonSetStatus{
				NumberUnavailable: 0,
				NumberReady:       1,
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hadoop-jindofs-fuse",
				Namespace: "fluid",
			},
			Spec: appsv1.DaemonSetSpec{},
			Status: appsv1.DaemonSetStatus{
				NumberUnavailable: 1,
				NumberReady:       1,
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "obj-jindofs-fuse",
				Namespace: "fluid",
			},
			Spec: appsv1.DaemonSetSpec{},
			Status: appsv1.DaemonSetStatus{
				NumberUnavailable: 1,
				NumberReady:       1,
			},
		},
	}

	objs := []runtime.Object{}

	for _, runtimeInput := range runtimeInputs {
		objs = append(objs, runtimeInput.DeepCopy())
	}
	for _, dataSetInput := range dataSetInputs {
		objs = append(objs, dataSetInput.DeepCopy())
	}
	for _, dsInputs := range dsInputss {
		objs = append(objs, dsInputs.DeepCopy())
	}

	for _, pod := range podList.Items {
		objs = append(objs, &pod)
	}

	// objs = append(objs, podList)

	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = datav1alpha1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)
	fakeClient := fake.NewFakeClientWithScheme(s, objs...)
	testCases := []struct {
		caseName  string
		name      string
		namespace string
		Phase     datav1alpha1.RuntimePhase
		fuse      *appsv1.DaemonSet
		TypeValue bool
		isErr     bool
	}{
		{
			caseName:  "Healthy",
			name:      "hbase",
			namespace: "fluid",
			fuse: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hbase-jindofs-fuse",
					Namespace: "big-data",
				},
				Spec: appsv1.DaemonSetSpec{},
				Status: appsv1.DaemonSetStatus{
					NumberUnavailable: 0,
				},
			},
			Phase: datav1alpha1.RuntimePhaseReady,

			isErr: false,
		},
		{
			caseName:  "Unhealthy",
			name:      "hadoop",
			namespace: "fluid",
			fuse: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hadoop-jindofs-fuse",
					Namespace: "fluid",
				},
				Spec: appsv1.DaemonSetSpec{},
				Status: appsv1.DaemonSetStatus{
					NumberUnavailable: 1,
				},
			},
			Phase: datav1alpha1.RuntimePhaseNotReady,
			isErr: true,
		},
	}
	for _, testCase := range testCases {

		runtimeInfo, err := base.BuildRuntimeInfo(testCase.name, testCase.namespace, "jindo", datav1alpha1.TieredStore{})
		if err != nil {
			t.Errorf("testcase %s failed due to %v", testCase.name, err)
		}

		var runtime *datav1alpha1.JindoRuntime = &datav1alpha1.JindoRuntime{}

		err = fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: testCase.namespace,
			Name:      testCase.name,
		}, runtime)

		if err != nil {
			t.Errorf("testCase %s sync replicas failed,err:%v", testCase.caseName, err)
		}

		ds := &appsv1.DaemonSet{}
		err = fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: testCase.fuse.Namespace,
			Name:      testCase.fuse.Name,
		}, ds)
		if err != nil {
			t.Errorf("sync replicas failed,err:%s", err.Error())
		}

		h := BuildHelper(runtimeInfo, fakeClient, log.NullLogger{})

		err = h.CheckFuseHealthy(record.NewFakeRecorder(300),
			runtime, runtime.Status, ds)

		if testCase.isErr == (err == nil) {
			t.Errorf("check fuse's healthy failed,err:%v", err)
		}

		err = fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: testCase.namespace,
			Name:      testCase.name,
		}, runtime)

		if err != nil {
			t.Errorf("check fuse's healthy failed,err:%s", err.Error())
		}

		if runtime.Status.FusePhase != testCase.Phase {
			t.Errorf("testcase %s is failed, expect phase %v, got %v", testCase.caseName,
				testCase.Phase,
				runtime.Status.FusePhase)
		}

	}
}
