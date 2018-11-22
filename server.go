package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/yamux"
)

func handleConnection(c io.ReadWriteCloser) {
	defer c.Close()

	c.Write([]byte("Hi Mom"))
}

func main() {
	fmt.Println("STARTED")
	session, err := yamux.Server(&StdinStdout{in: os.Stdin, out: os.Stdout}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	for {
		stream, err := session.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "STREAM ERROR:", err)
			return
		}
		go handleConnection(stream)
	}
}

type StdinStdout struct {
	in  io.ReadCloser
	out io.WriteCloser
}

func (s *StdinStdout) Read(b []byte) (int, error) {
	return s.in.Read(b)
}
func (s *StdinStdout) Write(b []byte) (int, error) {
	return s.out.Write(b)
}
func (s *StdinStdout) Close() error {
	e1 := s.in.Close()
	e2 := s.out.Close()
	if e1 != nil {
		return e1
	}
	return e2
}
