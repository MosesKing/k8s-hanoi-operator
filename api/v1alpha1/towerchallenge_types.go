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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// MaxDiscs caps the puzzle size. A puzzle with n discs needs 2^n - 1 moves,
// and the operator writes one ConfigMap per move: 10 discs means 1,023 objects.
const MaxDiscs = 10

// Condition types and reasons reported in status.conditions.
const (
	ConditionReady = "Ready"

	ReasonSolved      = "Solved"
	ReasonApplyFailed = "ApplyFailed"
)

// TowerChallengeSpec defines the desired state of TowerChallenge.
type TowerChallengeSpec struct {
	// discs is the number of discs to move from rod A to rod C.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +required
	Discs int32 `json:"discs"`
}

// TowerChallengeStatus defines the observed state of TowerChallenge.
type TowerChallengeStatus struct {
	// phase is a short summary of the state: Solving or Solved.
	// +optional
	Phase string `json:"phase,omitempty"`

	// totalMoves is the number of moves in the solution (2^discs - 1).
	// +optional
	TotalMoves int32 `json:"totalMoves,omitempty"`

	// observedGeneration is the spec generation the status refers to.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// solvedAt is when the operator last finished writing the solution.
	// +optional
	SolvedAt *metav1.Time `json:"solvedAt,omitempty"`

	// conditions represent the current state of the TowerChallenge.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tc
// +kubebuilder:printcolumn:name="Discs",type="integer",JSONPath=".spec.discs"
// +kubebuilder:printcolumn:name="Moves",type="integer",JSONPath=".status.totalMoves"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// TowerChallenge asks the operator to solve the Tower of Hanoi puzzle.
// The solution is written as one ConfigMap per move, owned by the challenge.
type TowerChallenge struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec TowerChallengeSpec `json:"spec"`

	// +optional
	Status TowerChallengeStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TowerChallengeList contains a list of TowerChallenge.
type TowerChallengeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []TowerChallenge `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &TowerChallenge{}, &TowerChallengeList{})
		return nil
	})
}
