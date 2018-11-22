package main

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/dgodd/grpcstdin"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	fmt.Println("MAIN")

	go func() {
		defer wg.Done()
		c, err := grpcstdin.Dial()
		if err != nil {
			fmt.Println("ERR:", err)
			return
		}

		_, err = c.Write([]byte("GET / HTTP/1.1\nHOST: example.com\n\n"))
		if err != nil {
			fmt.Println("ERR:", err)
			return
		}
		time.Sleep(2 * time.Millisecond)

		var buf bytes.Buffer
		io.Copy(&buf, c)
		c.Close()
		fmt.Println("EXAMPLE:", len(buf.String()), buf.String()[:200])
	}()

	go func() {
		defer wg.Done()
		c, err := grpcstdin.Dial()
		if err != nil {
			fmt.Println("ERR:", err)
			return
		}

		_, err = c.Write([]byte("GET / HTTP/1.1\nHOST: www.google.com\n\n"))
		if err != nil {
			fmt.Println("ERR:", err)
			return
		}
		time.Sleep(2 * time.Millisecond)

		var buf bytes.Buffer
		io.Copy(&buf, c)
		c.Close()
		fmt.Println("GOOGLE:", len(buf.String()), buf.String()[:200])
	}()

	wg.Wait()
}
