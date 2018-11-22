package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/hashicorp/yamux"
)

func handleConnection(c io.ReadWriteCloser) {
	defer c.Close()

	var addrLen uint32
	if err := binary.Read(c, binary.LittleEndian, &addrLen); err != nil {
		fmt.Fprintf(os.Stderr, "READ ADDR LEN ERR: %s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "ADDRLEN: %d\n", addrLen)

	addr := make([]byte, addrLen)
	if _, err := c.Read(addr); err != nil {
		fmt.Fprintf(os.Stderr, "READ ADDR ERR: %s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "ADDR: |%s|\n", addr)

	conn, err := net.Dial("tcp", string(addr))
	if err != nil {
		fmt.Fprintf(os.Stderr, "TCP CONN ERR: %s: %s\n", addr, err)
		return
	}
	go io.Copy(conn, c)
	io.Copy(c, conn)
}

func main() {
	fmt.Println("STARTED")
	fmt.Fprintln(os.Stderr, "STARTED")

	session, err := yamux.Server(&StdinStdout{in: os.Stdin, out: os.Stdout}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "SESSION CREATE ERROR:", err)
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
