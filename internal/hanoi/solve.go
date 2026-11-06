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

// Package hanoi solves the Tower of Hanoi puzzle.
package hanoi

import "fmt"

// Rods of the puzzle. Discs start on A and must end on C.
const (
	RodA = "A"
	RodB = "B"
	RodC = "C"
)

// Move is one step of the solution: disc Disc goes from rod From to rod To.
// Discs are numbered from 1 (smallest) to n (largest).
type Move struct {
	Disc int
	From string
	To   string
}

// String renders the move as a sentence, e.g. "Move disc 1 from A to C".
func (m Move) String() string {
	return fmt.Sprintf("Move disc %d from %s to %s", m.Disc, m.From, m.To)
}

// Solve returns the optimal solution for n discs: 2^n - 1 moves.
// It returns nil for n < 1.
func Solve(n int) []Move {
	if n < 1 {
		return nil
	}
	moves := make([]Move, 0, (1<<n)-1)
	var solve func(n int, from, to, via string)
	solve = func(n int, from, to, via string) {
		if n == 0 {
			return
		}
		solve(n-1, from, via, to)
		moves = append(moves, Move{Disc: n, From: from, To: to})
		solve(n-1, via, to, from)
	}
	solve(n, RodA, RodC, RodB)
	return moves
}
