package semaphore 

import (
	"container/list"
	"context"
	"sync"
	"container/heap"
	"strings"
	"fmt"
)

type priorityElement struct {
    ID    string
    Quota int64
}

type quotaPriorities struct {
    elems []priorityElement
    index map[string]int
}

func NewQuotaPriorities() *quotaPriorities {
    return &quotaPriorities{index: make(map[string]int)}
}

func (h quotaPriorities) Len() int           { return len(h.elems) }
func (h quotaPriorities) Less(i, j int) bool { return h.elems[i].Quota < h.elems[j].Quota }
func (h quotaPriorities) Swap(i, j int) {
    h.elems[i], h.elems[j] = h.elems[j], h.elems[i]
    h.index[h.elems[i].ID] = i
    h.index[h.elems[j].ID] = j
}

func (h *quotaPriorities) Push(x any) {
    e := x.(priorityElement)
    h.index[e.ID] = len(h.elems)
    h.elems = append(h.elems, e)
}

func (h *quotaPriorities) Pop() any {
    old := h.elems
    e := old[len(old)-1]
    h.elems = old[:len(old)-1]
    delete(h.index, e.ID)
    return e
}

func (h *quotaPriorities) Upsert(e priorityElement) {
    if i, exists := h.index[e.ID]; exists {
        heap.Remove(h, i)
    }
    heap.Push(h, e)
}

func (h *quotaPriorities) Remove(ID string) {
    i, ok := h.index[ID]
    if !ok {
        return
    }
    delete(h.index, ID)
    heap.Remove(h, i)
}

/////////////////////////////////////////////////////////

type waiter struct {
	n     int64
	ready chan<- struct{}
}

func NewWeighted(n int64, tenants map[string]*Tenant) *Weighted {
	w := &Weighted{size: n, tenants: tenants,  priorities: NewQuotaPriorities()}
	return w
}

type Weighted struct {
	size    int64
	cur     int64
    tenants       map[string]*Tenant
	priorities *quotaPriorities
    mu            sync.Mutex        
}

type Tenant struct {
    MaxShare  int64     
    requested   int64      
    waiters   list.List  
}

////////////////////////////////////////////////////////////////////////////

func (s *Weighted) Acquire(ctx context.Context, ID string, n int64) error {
	done := ctx.Done()

	s.mu.Lock()
	select {
	case <-done:
		s.mu.Unlock()
		return ctx.Err()
	default:
	}

    tenant := s.tenants[ID]
	if s.size-s.cur >= n && s.priorities.Len() == 0 {
		s.cur += n
		tenant.requested += n
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
	tenant.requested += n
	elem := tenant.waiters.PushBack(w)
	quota := getQuota(*tenant)
	priorityElement := priorityElement{ID: ID, Quota: quota}
	s.priorities.Upsert(priorityElement)
	s.mu.Unlock()

	select {
	case <-done:
		s.mu.Lock()
		tenant.requested -= n
		s.updatePriorities(ID)
		select {
		case <-ready:
			s.cur -= n
			s.notifyTenants()
		default:
			isFront := s.priorities.elems[0].ID == ID && tenant.waiters.Front() == elem
			tenant.waiters.Remove(elem)
			if isFront && s.size > s.cur {
				tenant.notifyWaiters(s)
			}
		}
		s.mu.Unlock()
		return ctx.Err()

	case <-ready:
		select {
		case <-done:
			s.Release(ID, n)
			return ctx.Err()
		default:
		}
		return nil
	}
}

func (s *Weighted) Release(ID string, n int64) {
	s.mu.Lock()
	tenant := s.tenants[ID]
	s.cur -= n
	tenant.requested -= n
	if s.cur < 0 {
		s.mu.Unlock()
		panic("semaphore: released more than held")
	}
	s.updatePriorities(ID)
	s.notifyTenants()
	s.mu.Unlock()
}

func (s *Weighted) notifyTenants() {
	for len(s.priorities.elems) > 0 {
	  priorityElement := s.priorities.elems[0]
      tenant := s.tenants[priorityElement.ID] 
      tenant.notifyWaiters(s)
		if tenant.waiters.Len() != 0 {
			break
		}
		_ = heap.Pop(s.priorities)
    }
}

func (tenant *Tenant) notifyWaiters(s *Weighted) {
	for {
		next := tenant.waiters.Front()
		if next == nil {
			break 
		}

		w := next.Value.(waiter)
		if s.size-s.cur < w.n {
			break
		}

		s.cur += w.n
		tenant.waiters.Remove(next)
		close(w.ready)
	}
}

func getQuota(tenant Tenant) int64 {
	return (tenant.requested * 100) / tenant.MaxShare  
}

func (s *Weighted) updatePriorities(ID string) {
  tenant := s.tenants[ID]
  if tenant.waiters.Len() != 0 {
    quota := getQuota(*tenant)
	priorityElement := priorityElement{ID: ID, Quota: quota}
    s.priorities.Upsert(priorityElement)
  } else {
    s.priorities.Remove(ID)
  }
}

func (w *Weighted) String() string {
    var sb strings.Builder
    fmt.Fprintf(&sb, "size=%d cur=%d\n", w.size, w.cur)
    for name, t := range w.tenants {
        fmt.Fprintf(&sb, "  tenant=%s MaxShare=%d requested=%d waiters=%d\n",
            name, t.MaxShare, t.requested, t.waiters.Len())
    }
    for i, p := range w.priorities.elems {
        fmt.Fprintf(&sb, "  priority[%d] id=%s quota=%d\n", i, p.ID, p.Quota)
    }
    return sb.String()
}