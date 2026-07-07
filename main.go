package main

import (
    "fmt"
    "sync"
)

func main() {
    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        fmt.Println("hello 线程1")
    }()

    go func() {
        defer wg.Done()
        fmt.Println("hello 线程2")
    }()

    wg.Wait()
}
