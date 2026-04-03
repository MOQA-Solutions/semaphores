# Semaphores

This repository contains various types of **Semaphores** implemented in Go, some of which have been **Tested in Production** and proven **Stable** under Concurrent workloads.

## Use Cases

- Rate limiting concurrent goroutines
- Controlling worker pool concurrency
- Protecting shared resources
- Backpressure in distributed systems

## Multi-tenant Semaphore Example

```go
tenants := ... // Subscribers backed by a map for simplicity
n := 1000 
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

sem := NewWeighted(n, tenants)

sem.Acquire(ctx, "tenantID_1", 150) 
go sem.Acquire(ctx, "tenantID_2", 350)
sem.Release("tenantID_1", 100)
...
```
