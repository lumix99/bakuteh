package main

import (
	"fmt"
)

type mychan chan error

// func main() {
// 	lock := sync.RWMutex{}
// 	lock.Lock()
// 	fmt.Println("lock!")
// 	lock.Unlock()
// 	fmt.Println("unlock!")
// 	lock.Unlock()
// 	fmt.Println("unlock again!")

// }

func sub(name string, ch <-chan bool) {
	count := 5
	for {
		b, ok := <-ch
		fmt.Println(name, b, ok)
		if !ok && count <= 0 {
			return
		} else {
			count--
		}
	}
}
