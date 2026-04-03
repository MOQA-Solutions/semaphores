package semaphore_priority

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type waiter struct {
	n     int64
	ready chan<- struct{}
}

type priority int 

const (
	high priority = 3
	normal priority = 2
	low priority = 1
)

func NewWeighted(n int64) *Weighted {
	w := &Weighted{size: n}
	return w
}

type Weighted struct {
	size    int64
	cur     int64
	mu      sync.Mutex

	high list.List
	normal list.List
	low list.List
}

func (s *Weighted) Acquire(ctx context.Context, n int64, pr priority) error {
	waiters := getWaiters(s, pr)
	done := ctx.Done()

	s.mu.Lock()
	select {
	case <-done:
		s.mu.Unlock()
		return ctx.Err()
	default:
	}

	higherWaiters := getHigherWaiters(s, pr)
	if s.size-s.cur >= n && emptyHigherWaiters(higherWaiters) {
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
	
	elem := waiters.PushBack(w)
	s.mu.Unlock()

	timer := time.NewTimer(2 * time.Minute)
    defer timer.Stop()
    
	for {
	  select {
		case <-done:
		  s.mu.Lock()
		  select {
			case <-ready:
			  s.cur -= n
			  s.notifyWaiters()
			default:
			  isFront := waiters.Front() == elem
			  waiters.Remove(elem)
			  if isFront && s.size > s.cur {
			    s.notifyWaiters()
			  }
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

	    case <-timer.C: // upgrade priority after waiting for 2 minutes to prevent infinite waiting
	      select {
		    case <-ready:
			  return nil
		    case <-done:
			  s.Release(n)
			  return ctx.Err()
		    default:
			  newPr := newPriority(pr)
			  if newPr != high {
				s.mu.Lock() 
				waiters.Remove(elem) 
				waiters = getWaiters(s, newPr)
				elem = waiters.PushBack(w)
				pr = newPr
				timer.Reset(2 * time.Minute)
				s.mu.Unlock()
			  }  
	      }
      }
	}
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
    notifyWaiters(s, high)
	notifyWaiters(s, normal)
	notifyWaiters(s, low) 
   }

func notifyWaiters(s *Weighted, pr priority) {
    waiters := getWaiters(s, pr)
	for {
		next := waiters.Front()
		if next == nil {
			break 
		}

		w := next.Value.(waiter)
		if s.size-s.cur < w.n {
			break
		}

		s.cur += w.n
		waiters.Remove(next)
		close(w.ready)
	}
}

func getWaiters(s *Weighted, pr priority) *list.List {
	var waiters *list.List
	switch {
	case pr == high:
		waiters = &s.high
	case pr == normal:
		waiters = &s.normal
	default:
		waiters = &s.low
	}

	return waiters
}

func getHigherWaiters(s *Weighted, pr priority) []list.List {
	var result []list.List
	for {
		if pr == high {
			break
		}
		pr = newPriority(pr)
		result = append(result, *getWaiters(s, pr))
	} 
	return result 
}


func newPriority(pr priority) priority {
  var newPr priority 
  switch {
    case pr == high:
	  newPr = pr 
    case pr == normal:
	  newPr = high
    default:
	  newPr = normal
  }
  return newPr
}

func emptyHigherWaiters(higherWaiters []list.List) bool {
	for _, v := range higherWaiters {
      if v.Len() != 0 {return false} 
	} 
	return true 
}
