/*
Copyright 2026 Moesaeah King.

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
	"cmp"
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hanoiv1alpha1 "github.com/MosesKing/k8s-hanoi-operator/api/v1alpha1"
)

var _ = Describe("TowerChallenge controller", func() {
	var (
		namespace  string
		reconciler *TowerChallengeReconciler
		counter    int
	)

	BeforeEach(func() {
		counter++
		namespace = fmt.Sprintf("hanoi-test-%d", counter)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		})).To(Succeed())
		reconciler = &TowerChallengeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	})

	newChallenge := func(name string, discs int32) *hanoiv1alpha1.TowerChallenge {
		tc := &hanoiv1alpha1.TowerChallenge{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       hanoiv1alpha1.TowerChallengeSpec{Discs: discs},
		}
		Expect(k8sClient.Create(ctx, tc)).To(Succeed())
		return tc
	}

	reconcileOnce := func(name string) {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
		})
		Expect(err).NotTo(HaveOccurred())
	}

	moveConfigMaps := func(name string) []corev1.ConfigMap {
		var list corev1.ConfigMapList
		Expect(k8sClient.List(ctx, &list,
			client.InNamespace(namespace),
			client.MatchingLabels{LabelChallenge: name},
		)).To(Succeed())
		slices.SortFunc(list.Items, func(a, b corev1.ConfigMap) int { return cmp.Compare(a.Name, b.Name) })
		return list.Items
	}

	getChallenge := func(name string) *hanoiv1alpha1.TowerChallenge {
		tc := &hanoiv1alpha1.TowerChallenge{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, tc)).To(Succeed())
		return tc
	}

	It("writes one owned ConfigMap per move and reports Solved", func() {
		tc := newChallenge("three", 3)
		reconcileOnce("three")

		cms := moveConfigMaps("three")
		Expect(cms).To(HaveLen(7))
		Expect(cms[0].Name).To(Equal("three-move-0001"))
		Expect(cms[0].Data).To(HaveKeyWithValue("move", "Move disc 1 from A to C"))
		Expect(cms[3].Data).To(HaveKeyWithValue("move", "Move disc 3 from A to C"))
		Expect(cms[6].Name).To(Equal("three-move-0007"))
		for _, cm := range cms {
			Expect(metav1.IsControlledBy(&cm, tc)).To(BeTrue(), "%s should be owned by the challenge", cm.Name)
		}

		got := getChallenge("three")
		Expect(got.Status.Phase).To(Equal("Solved"))
		Expect(got.Status.TotalMoves).To(Equal(int32(7)))
		Expect(got.Status.ObservedGeneration).To(Equal(got.Generation))
		Expect(meta.IsStatusConditionTrue(got.Status.Conditions, hanoiv1alpha1.ConditionReady)).To(BeTrue())
	})

	It("is idempotent: a second pass changes nothing", func() {
		newChallenge("idem", 3)
		reconcileOnce("idem")
		before := map[string]string{}
		for _, cm := range moveConfigMaps("idem") {
			before[cm.Name] = cm.ResourceVersion
		}
		statusBefore := getChallenge("idem").ResourceVersion

		reconcileOnce("idem")

		for _, cm := range moveConfigMaps("idem") {
			Expect(cm.ResourceVersion).To(Equal(before[cm.Name]), "%s was rewritten", cm.Name)
		}
		Expect(getChallenge("idem").ResourceVersion).To(Equal(statusBefore))
	})

	It("removes extra moves when discs go down and adds them when discs go up", func() {
		newChallenge("resize", 4)
		reconcileOnce("resize")
		Expect(moveConfigMaps("resize")).To(HaveLen(15))

		tc := getChallenge("resize")
		tc.Spec.Discs = 2
		Expect(k8sClient.Update(ctx, tc)).To(Succeed())
		reconcileOnce("resize")
		Expect(moveConfigMaps("resize")).To(HaveLen(3))
		Expect(getChallenge("resize").Status.TotalMoves).To(Equal(int32(3)))

		tc = getChallenge("resize")
		tc.Spec.Discs = 5
		Expect(k8sClient.Update(ctx, tc)).To(Succeed())
		reconcileOnce("resize")
		Expect(moveConfigMaps("resize")).To(HaveLen(31))
	})

	It("repairs a deleted or edited move", func() {
		newChallenge("repair", 3)
		reconcileOnce("repair")

		deleted := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "repair-move-0002", Namespace: namespace}, deleted)).To(Succeed())
		Expect(k8sClient.Delete(ctx, deleted)).To(Succeed())

		edited := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "repair-move-0004", Namespace: namespace}, edited)).To(Succeed())
		edited.Data["move"] = "Move disc 3 from A to B"
		Expect(k8sClient.Update(ctx, edited)).To(Succeed())

		reconcileOnce("repair")

		cms := moveConfigMaps("repair")
		Expect(cms).To(HaveLen(7))
		Expect(cms[1].Data).To(HaveKeyWithValue("move", "Move disc 2 from A to B"))
		Expect(cms[3].Data).To(HaveKeyWithValue("move", "Move disc 3 from A to C"))
	})

	It("never deletes a ConfigMap it does not own", func() {
		newChallenge("mine", 1)
		foreign := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      "someone-elses",
			Namespace: namespace,
			Labels:    map[string]string{LabelChallenge: "mine", LabelMove: "99"},
		}}
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())

		reconcileOnce("mine")

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(foreign), &corev1.ConfigMap{})).To(Succeed())
	})

	It("does nothing for a challenge that no longer exists", func() {
		reconcileOnce("ghost")
	})

	DescribeTable("the API rejects invalid disc counts",
		func(discs int32) {
			tc := &hanoiv1alpha1.TowerChallenge{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("invalid-%d", discs+100), Namespace: namespace},
				Spec:       hanoiv1alpha1.TowerChallengeSpec{Discs: discs},
			}
			err := k8sClient.Create(ctx, tc)
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected a validation error, got %v", err)
		},
		Entry("zero discs", int32(0)),
		Entry("negative discs", int32(-3)),
		Entry("more than the maximum", int32(hanoiv1alpha1.MaxDiscs+1)),
	)
})
