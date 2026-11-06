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
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hanoiv1alpha1 "github.com/MosesKing/k8s-hanoi-operator/api/v1alpha1"
	"github.com/MosesKing/k8s-hanoi-operator/internal/hanoi"
)

// Labels set on every ConfigMap the operator writes.
const (
	LabelChallenge = "hanoi.mosesking.github.io/challenge"
	LabelMove      = "hanoi.mosesking.github.io/move"
)

// TowerChallengeReconciler solves each TowerChallenge and keeps one ConfigMap
// per move in the challenge's namespace.
type TowerChallengeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=hanoi.mosesking.github.io,resources=towerchallenges,verbs=get;list;watch
// +kubebuilder:rbac:groups=hanoi.mosesking.github.io,resources=towerchallenges/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile makes the ConfigMaps in the namespace match the solution for
// spec.discs: it creates or repairs one ConfigMap per move, deletes moves that
// are no longer part of the solution, and reports the result in status.
// It is idempotent: running it twice in a row changes nothing the second time.
func (r *TowerChallengeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var tc hanoiv1alpha1.TowerChallenge
	if err := r.Get(ctx, req.NamespacedName, &tc); err != nil {
		// Deleted: the API server garbage-collects the owned ConfigMaps.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	moves := hanoi.Solve(int(tc.Spec.Discs))

	changed, err := r.applyMoves(ctx, &tc, moves)
	if err != nil {
		meta.SetStatusCondition(&tc.Status.Conditions, metav1.Condition{
			Type:               hanoiv1alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             hanoiv1alpha1.ReasonApplyFailed,
			Message:            err.Error(),
			ObservedGeneration: tc.Generation,
		})
		tc.Status.Phase = "Solving"
		if statusErr := r.Status().Update(ctx, &tc); statusErr != nil {
			log.Error(statusErr, "failed to record failure in status")
		}
		return ctrl.Result{}, err
	}

	stale := tc.Status.ObservedGeneration != tc.Generation || tc.Status.Phase != "Solved"
	if !changed && !stale {
		return ctrl.Result{}, nil
	}

	now := metav1.Now()
	tc.Status.Phase = "Solved"
	tc.Status.TotalMoves = int32(len(moves))
	tc.Status.ObservedGeneration = tc.Generation
	tc.Status.SolvedAt = &now
	meta.SetStatusCondition(&tc.Status.Conditions, metav1.Condition{
		Type:               hanoiv1alpha1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             hanoiv1alpha1.ReasonSolved,
		Message:            fmt.Sprintf("%d discs solved in %d moves", tc.Spec.Discs, len(moves)),
		ObservedGeneration: tc.Generation,
	})
	if err := r.Status().Update(ctx, &tc); err != nil {
		// A conflict means the object changed meanwhile; returning the error
		// requeues the request and the next pass works on fresh data.
		return ctrl.Result{}, err
	}

	log.Info("solved", "discs", tc.Spec.Discs, "moves", len(moves))
	return ctrl.Result{}, nil
}

// applyMoves writes one ConfigMap per move and removes ConfigMaps for moves
// beyond the current solution. It reports whether anything changed.
func (r *TowerChallengeReconciler) applyMoves(ctx context.Context, tc *hanoiv1alpha1.TowerChallenge, moves []hanoi.Move) (bool, error) {
	changed := false

	for i, m := range moves {
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      MoveConfigMapName(tc.Name, i+1),
			Namespace: tc.Namespace,
		}}
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
			if cm.Labels == nil {
				cm.Labels = map[string]string{}
			}
			cm.Labels[LabelChallenge] = tc.Name
			cm.Labels[LabelMove] = strconv.Itoa(i + 1)
			cm.Data = map[string]string{
				"move": m.String(),
				"disc": strconv.Itoa(m.Disc),
				"from": m.From,
				"to":   m.To,
			}
			return controllerutil.SetControllerReference(tc, cm, r.Scheme)
		})
		if err != nil {
			return changed, fmt.Errorf("move %d: %w", i+1, err)
		}
		if op != controllerutil.OperationResultNone {
			changed = true
		}
	}

	// Remove moves left over from a larger puzzle (e.g. discs went from 4 to 3).
	var existing corev1.ConfigMapList
	if err := r.List(ctx, &existing,
		client.InNamespace(tc.Namespace),
		client.MatchingLabels{LabelChallenge: tc.Name},
	); err != nil {
		return changed, fmt.Errorf("listing move ConfigMaps: %w", err)
	}
	for i := range existing.Items {
		cm := &existing.Items[i]
		n, err := strconv.Atoi(cm.Labels[LabelMove])
		if err == nil && n >= 1 && n <= len(moves) {
			continue
		}
		if !metav1.IsControlledBy(cm, tc) {
			continue // never delete what we do not own
		}
		if err := r.Delete(ctx, cm); client.IgnoreNotFound(err) != nil {
			return changed, fmt.Errorf("deleting %s: %w", cm.Name, err)
		}
		changed = true
	}

	return changed, nil
}

// MoveConfigMapName returns the ConfigMap name for move n, zero-padded so that
// `kubectl get configmaps` lists the moves in order.
func MoveConfigMapName(challenge string, n int) string {
	return fmt.Sprintf("%s-move-%04d", challenge, n)
}

// SetupWithManager sets up the controller with the Manager. It reacts to spec
// changes on challenges and to any change on the ConfigMaps it owns, so a
// deleted or edited move is repaired.
func (r *TowerChallengeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hanoiv1alpha1.TowerChallenge{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.ConfigMap{}).
		Named("towerchallenge").
		Complete(r)
}
