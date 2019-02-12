package main

import (
	"context"
	"fmt"
	"sync"
)

type routes []route
type route struct {
	//addr    storj.NodeID
	addr string
	//neighbors storj.NodeIDList
	neighbors []string
}

//type nodeIDSet map[storj.NodeID]struct{}
type addrSet map[string]struct{}
type Queue struct {
	ctx     context.Context
	workers int
	//routesC chan route

	mu      sync.Mutex
	working int

	cond      sync.Cond
	pending   []string
	inspected addrSet
	routes    routes
}

func NewQueue(ctx context.Context, workers int, start string, work func(context.Context, string, *Queue) error) *Queue {
	ctx, cancel := context.WithCancel(ctx)
	q := &Queue{
		ctx:     ctx,
		workers: workers,
		//routesC: make(chan route, workers),

		mu: sync.Mutex{},

		cond: sync.Cond{
			L: &sync.Mutex{},
		},
		pending:   []string{start},
		inspected: make(addrSet),
	}

	for i := 0; i < q.workers; i++ {
		go func() {
			for {
				q.mu.Lock()
				select {
				case <-ctx.Done():
					return
				default:
					q.working++
					q.mu.Unlock()

					q.mu.Lock()
					// Check if all done
					if q.working == 0 && len(q.pending) == 0 {
						cancel()
						q.cond.L.Unlock()
						// All done, wake all workers up to return
						q.cond.Broadcast()
						break
					}

					next := q.Pop()
					if err := work(ctx, next, q); err != nil {
						fmt.Printf("error: %s\n", err)
						continue
					}

					q.working--

					q.mu.Unlock()

					q.cond.Signal()
				}
			}
		}()
	}
	return q
}

func (q *Queue) Push(r route) {
	//q.routesC <- r
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	// Add neighbors to pending
	for _, neighbor := range r.neighbors {
		if _, seen := q.inspected[neighbor]; seen {
			// Skip if seen before
			continue
		}

		q.routes = append(q.routes, r)
		//q.pendingC <- neighbor
	}
	q.cond.Signal()
}

func (q *Queue) Pop() string {
	//return <-q.pendingC
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	return q.pending[0]
}
