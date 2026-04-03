package main

import "github.com/abdelghani/test-project/server/semaphores"
import (
	    "context"
		"net/http"
		"fmt"
		"strconv"
		"strings"
		"hash/fnv"
    )

var (
    tenants map[string]*semaphore.Tenant
    workers = make(map[int]chan string)
    ctx     = context.Background()
    s       *semaphore.Weighted
)

func init() {
    tenants = map[string]*semaphore.Tenant{
        "User1": {MaxShare: 200},
        "User2": {MaxShare: 500},
        "User3": {MaxShare: 300},
        "User4": {MaxShare: 400},
    }
    s = semaphore.NewWeighted(1000, tenants)
}

func main() {
  
  for i:=1; i<4; i++ {
	ch := make(chan string, 100)
	workers[i] = ch
	go loop(ch)
  }

  mux := http.NewServeMux()

  mux.HandleFunc("/acquire", acquireHandler)
  mux.HandleFunc("/release", releaseHandler)
  mux.HandleFunc("/log", logHandler)

  err := http.ListenAndServe(":9090", mux)
  fmt.Println("ListenAndServe error:", err)
}

func acquireHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	n := r.URL.Query().Get("share")  
	ch := getChannel(id)
	ch <- fmt.Sprintf("acquire-%s-%s", id, n)
	<- ch 
	w.WriteHeader(http.StatusOK)
}

func releaseHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	n := r.URL.Query().Get("share")  
	ch := getChannel(id)
	ch <- fmt.Sprintf("release-%s-%s", id, n) 
	<- ch 
	w.WriteHeader(http.StatusOK)
}

func logHandler(rw http.ResponseWriter, r *http.Request) {
    result := s.String()
    fmt.Println("log result:", result) // print to terminal
    fmt.Fprint(rw, result)
}

func loop(ch chan string) {
	for msg := range ch {
      parts := strings.Split(msg, "-")
	  op := parts[0]
	  id := parts[1]
	  n, _ := strconv.ParseInt(parts[2] , 10, 64)
	  switch op {
        case "acquire":
			s.Acquire(ctx, id, n) 
			ch <- "done" 
		case "release":
			fmt.Printf("release received")
			s.Release(id, n)
			ch <- "done" 
		default:
            continue
	  }
	}
}

func getChannel(id string) chan string {
	index := phash(id, 3) + 1
	return workers[index]
}

func phash(s string, n int) int {
    h := fnv.New32a()
    h.Write([]byte(s))
    return int(h.Sum32()) % n
}




