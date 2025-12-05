// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobqueue

import (
	"container/heap"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/jobq"
)

func (oh *overlayHeap) verify(t *testing.T, i int) {
	t.Helper()
	n := oh.Len()
	j1 := 2*i + 1
	j2 := 2*i + 2
	if j1 < n {
		if oh.Less(j1, i) {
			t.Errorf("heap invariant invalidated [%d] = %d > [%d] = %d",
				i, oh.heapArray[oh.getIndex(i)].ID.Position, j1, oh.heapArray[oh.getIndex(j1)].ID.Position)
			return
		}
		oh.verify(t, j1)
	}
	if j2 < n {
		if oh.Less(j2, i) {
			t.Errorf("heap invariant invalidated [%d] = %d > [%d] = %d",
				i, oh.heapArray[oh.getIndex(i)].ID.Position, j1, oh.heapArray[oh.getIndex(j2)].ID.Position)
			return
		}
		oh.verify(t, j2)
	}
}

type simpleHeap []jobq.RepairJob

func (h *simpleHeap) Len() int           { return len(*h) }
func (h *simpleHeap) Less(i, j int) bool { return (*h)[i].Health < (*h)[j].Health }
func (h *simpleHeap) Swap(i, j int)      { (*h)[i], (*h)[j] = (*h)[j], (*h)[i] }
func (h *simpleHeap) Push(x any)         { *h = append(*h, x.(jobq.RepairJob)) }
func (h *simpleHeap) Pop() any {
	x := (*h)[len(*h)-1]
	*h = (*h)[:len(*h)-1]
	return x
}

// Tests adapted from Go stdlib container/heap/heap_test.go

func TestQueueOverlayInit0(t *testing.T) {
	sh := &simpleHeap{}
	for i := 20; i > 0; i-- {
		sh.Push(jobq.RepairJob{}) // all elements are the same
	}
	heap.Init(sh)
	oh := newOverlayHeap([]jobq.RepairJob(*sh), sh.Less)
	oh.verify(t, 0)

	for i := 1; oh.Len() > 0; i++ {
		x := heap.Pop(oh).(jobq.RepairJob)
		oh.verify(t, 0)
		require.Equal(t, jobq.RepairJob{}, x, i)
	}

	// check that sh is unchanged
	require.Len(t, *sh, 20)
	for i := 0; i < 20; i++ {
		require.Equal(t, jobq.RepairJob{}, (*sh)[i])
	}
}

func TestQueueOverlayInit1(t *testing.T) {
	sh := &simpleHeap{}
	for i := 20; i > 0; i-- {
		sh.Push(jobq.RepairJob{
			ID:     jobq.SegmentIdentifier{Position: uint64(i)},
			Health: float64(i),
		}) // all elements are different
	}
	heap.Init(sh)
	oh := newOverlayHeap([]jobq.RepairJob(*sh), sh.Less)
	oh.verify(t, 0)

	for i := 1; oh.Len() > 0; i++ {
		x := heap.Pop(oh).(jobq.RepairJob)
		oh.verify(t, 0)
		require.Equal(t, float64(i), x.Health, i)
		require.Equal(t, uint64(i), x.ID.Position)
	}

	// check that sh is unchanged
	require.Len(t, *sh, 20)
	for i := 0; i < 20; i++ {
		j := heap.Pop(sh).(jobq.RepairJob)
		require.Equal(t, float64(i+1), j.Health)
		require.Equal(t, uint64(i+1), j.ID.Position)
		require.Len(t, *sh, 19-i)
	}
}

func TestQueueOverlayRemove0(t *testing.T) {
	sh := &simpleHeap{}
	for i := 0; i < 10; i++ {
		sh.Push(jobq.RepairJob{
			ID:     jobq.SegmentIdentifier{Position: uint64(i)},
			Health: float64(i),
		})
	}
	heap.Init(sh)
	oh := newOverlayHeap([]jobq.RepairJob(*sh), sh.Less)
	oh.verify(t, 0)

	for oh.Len() > 0 {
		i := oh.Len() - 1
		x := heap.Remove(oh, i).(jobq.RepairJob)
		require.Equal(t, float64(i), x.Health)
		require.Equal(t, uint64(i), x.ID.Position)
		oh.verify(t, 0)
	}

	// check that sh is unchanged
	require.Len(t, *sh, 10)
	for i := 0; i < 10; i++ {
		j := heap.Pop(sh).(jobq.RepairJob)
		require.Equal(t, float64(i), j.Health)
		require.Equal(t, uint64(i), j.ID.Position)
		require.Len(t, *sh, 9-i)
	}
}

func TestQueueOverlayRemove1(t *testing.T) {
	sh := &simpleHeap{}
	for i := 0; i < 10; i++ {
		sh.Push(jobq.RepairJob{
			ID:     jobq.SegmentIdentifier{Position: uint64(i)},
			Health: float64(i),
		})
	}
	oh := newOverlayHeap([]jobq.RepairJob(*sh), sh.Less)
	oh.verify(t, 0)

	for i := 0; oh.Len() > 0; i++ {
		x := heap.Remove(oh, 0).(jobq.RepairJob)
		require.Equal(t, float64(i), x.Health)
		require.Equal(t, uint64(i), x.ID.Position)
		oh.verify(t, 0)
	}

	// check that sh is unchanged
	require.Len(t, *sh, 10)
	for i := 0; i < 10; i++ {
		j := heap.Pop(sh).(jobq.RepairJob)
		require.Equal(t, float64(i), j.Health)
		require.Equal(t, uint64(i), j.ID.Position)
		require.Len(t, *sh, 9-i)
	}
}

func TestQueueOverlayRemove2(t *testing.T) {
	N := 10

	sh := &simpleHeap{}
	for i := 0; i < N; i++ {
		sh.Push(jobq.RepairJob{
			ID:     jobq.SegmentIdentifier{Position: uint64(i)},
			Health: float64(i),
		})
	}
	oh := newOverlayHeap([]jobq.RepairJob(*sh), sh.Less)
	oh.verify(t, 0)

	m := make(map[uint64]bool)
	for oh.Len() > 0 {
		m[heap.Remove(oh, (oh.Len()-1)/2).(jobq.RepairJob).ID.Position] = true
		oh.verify(t, 0)
	}

	require.Len(t, m, N)
	for i := 0; i < len(m); i++ {
		if !m[uint64(i)] {
			t.Errorf("m[%d] doesn't exist", i)
		}
	}
}

func TestQueueOverlayFix(t *testing.T) {
	sh := &simpleHeap{}

	for i := 200; i > 0; i -= 10 {
		heap.Push(sh, jobq.RepairJob{
			ID:     jobq.SegmentIdentifier{Position: uint64(i)},
			Health: float64(i),
		})
	}
	oh := newOverlayHeap([]jobq.RepairJob(*sh), sh.Less)
	oh.verify(t, 0)

	require.Equal(t, uint64(10), (*sh)[0].ID.Position)

	oh.heapArray[oh.getIndex(0)].ID.Position = 210
	oh.heapArray[oh.getIndex(0)].Health = 210.0
	heap.Fix(oh, 0)
	oh.verify(t, 0)

	for i := 100; i > 0; i-- {
		elem := rand.Intn(oh.Len())
		ind := oh.getIndex(elem)
		if i&1 == 0 {
			oh.heapArray[ind].Health *= 2
			oh.heapArray[ind].ID.Position *= 2
		} else {
			oh.heapArray[ind].Health /= 2
			oh.heapArray[ind].ID.Position /= 2
		}
		heap.Fix(oh, elem)
		oh.verify(t, 0)
	}

	// don't check sh; we _do_ expect it to have changed this time.
}
