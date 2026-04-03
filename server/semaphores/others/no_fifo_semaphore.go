package no_fifo_semaphore 

import (
	"container/list"
	"context"
	"sync"
)

type waiter struct {
	n     int64
	ready chan<- struct{} 
}

func NewWeighted(n int64) *Weighted {
	w := &Weighted{size: n}
	return w
}

type Weighted struct {
	size    int64
	cur     int64
	mu      sync.Mutex
	waiters list.List
}

func (s *Weighted) Acquire(ctx context.Context, n int64) error {
	done := ctx.Done()

	s.mu.Lock()

	select {
		case <-done:
			s.mu.Unlock()
			return ctx.Err()
		default:
		}

		if s.size-s.cur >= n && s.waiters.Len() == 0 {
			s.cur += n
			s.mu.Unlock()
			return nil
		}

		if n > s.size {
			s.mu.Unlock()
			<-done
			return ctx.Err()
		}

		ready := make(chan struct{})
		w := waiter{n: n, ready: ready}
		elem := s.waiters.PushBack(w)
		s.mu.Unlock()

		select {
			case <-done:
				s.mu.Lock()
				select {
					case <-ready:
						s.cur -= n
						s.notifyWaiters()
					default:
						s.waiters.Remove(elem)
				}
				s.mu.Unlock()
				return ctx.Err()

	case <-ready:
		select {
		case <-done:
			s.Release(n)
			return ctx.Err()
		default:
		}
		return nil
	}
}

func (s *Weighted) TryAcquire(n int64) bool {
	s.mu.Lock()
	success := s.size-s.cur >= n && s.waiters.Len() == 0
	if success {
		s.cur += n
	}
	s.mu.Unlock()
	return success
}

func (s *Weighted) Release(n int64) {
	s.mu.Lock()
	s.cur -= n
	if s.cur < 0 {
		s.mu.Unlock()
		panic("semaphore: released more than held")
	}
	s.notifyWaiters()
	s.mu.Unlock()
}

func (s *Weighted) notifyWaiters() {
	next := s.waiters.Front()
	newNext := nil
	for {
		  if next == nil {
			break 
		  }

		  w := next.Value.(waiter)
		  if s.size-s.cur < w.n {
			next = next.Next()
			continue
		  }

		  s.cur += w.n
          newNext = next.Next()
		  s.waiters.Remove(next)
		  next = newNext
		  close(w.ready)
	}
}