package main

import (
    "bufio"
    "fmt"
    "net/http"
    "os"
    "strings"
	"log"
)

func main () {
    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        parts := strings.Split(scanner.Text(), " ")
        switch parts[0] {
        case "acquire":
            acquire(parts[1], parts[2])
        case "release":
            release(parts[1], parts[2])
		    case "log":
            logg()
        }
	}
}

func acquire(id string, n string) {
  resp, err := http.Get(fmt.Sprintf("http://localhost:9090/acquire?id=%s&share=%s", id, n))
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println(resp.StatusCode)
}

func release(id string, n string) {
  resp, err := http.Get(fmt.Sprintf("http://localhost:9090/release?id=%s&share=%s", id, n))
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println(resp.StatusCode)
}

func logg() {
  resp, err := http.Get("http://localhost:9090/log")
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println(resp.StatusCode)
}