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

package hanoi

import (
	"fmt"
	"testing"
)

func TestSolveInvalid(t *testing.T) {
	for _, n := range []int{-1, 0} {
		if got := Solve(n); got != nil {
			t.Errorf("Solve(%d) = %v, want nil", n, got)
		}
	}
}

func TestSolveThreeDiscs(t *testing.T) {
	want := []string{
		"Move disc 1 from A to C",
		"Move disc 2 from A to B",
		"Move disc 1 from C to B",
		"Move disc 3 from A to C",
		"Move disc 1 from B to A",
		"Move disc 2 from B to C",
		"Move disc 1 from A to C",
	}
	got := Solve(3)
	if len(got) != len(want) {
		t.Fatalf("Solve(3) returned %d moves, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("move %d = %q, want %q", i+1, got[i], want[i])
		}
	}
}

// TestSolveIsValidAndOptimal replays every solution on three rods and checks
// the rules: only the top disc moves, a disc never lands on a smaller one,
// and all discs end on rod C in 2^n - 1 moves.
func TestSolveIsValidAndOptimal(t *testing.T) {
	for n := 1; n <= 12; n++ {
		t.Run(fmt.Sprintf("%d discs", n), func(t *testing.T) {
			rods := map[string][]int{RodA: {}, RodB: {}, RodC: {}}
			for d := n; d >= 1; d-- {
				rods[RodA] = append(rods[RodA], d)
			}

			moves := Solve(n)
			if want := (1 << n) - 1; len(moves) != want {
				t.Fatalf("got %d moves, want %d", len(moves), want)
			}

			for i, m := range moves {
				from := rods[m.From]
				if len(from) == 0 || from[len(from)-1] != m.Disc {
					t.Fatalf("move %d (%v): disc %d is not on top of rod %s", i+1, m, m.Disc, m.From)
				}
				to := rods[m.To]
				if len(to) > 0 && to[len(to)-1] < m.Disc {
					t.Fatalf("move %d (%v): disc %d placed on smaller disc %d", i+1, m, m.Disc, to[len(to)-1])
				}
				rods[m.From] = from[:len(from)-1]
				rods[m.To] = append(to, m.Disc)
			}

			if len(rods[RodC]) != n || len(rods[RodA]) != 0 || len(rods[RodB]) != 0 {
				t.Fatalf("final state A=%v B=%v C=%v, want all discs on C", rods[RodA], rods[RodB], rods[RodC])
			}
		})
	}
}
