package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/Buerzhu/tiny_grpc/pool"
	log "github.com/golang/glog"
)

func main() {
	flag.Parse()
	defer log.Flush()

	poolTest()
}

func printNum(i int) error {
	fmt.Printf("num:%d\n", i)
	return nil
}
func poolTest() {
	for i := 0; i < 10; i = i + 1 {
		num := i
		pool.SubmitTask(func() error {
			fmt.Printf("num:%d\n", num)
			return nil
		})
	}
	time.Sleep(time.Second * 1)
}
