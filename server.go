package main

import (
	"fmt"
	"github.com/alecthomas/multiplex"
)

func handleConnection(c io.ReadWriteCloser) {
	// defer c.Close()
	c.Close()

	c.Write([]byte("Hi Mom"))
}

func main() {
	mx := multiplex.MultiplexedServer(&StdinStdout{in: os.Stdin, out: os.Stdout})
	for {
		c, err := mx.Accept()
		go handleConnection(c)
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
