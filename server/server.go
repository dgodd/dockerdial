package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/hashicorp/yamux"
)

func handleConnection(c io.ReadWriteCloser) {
	defer c.Close()

	r := bufio.NewReader(c)
	var head string
	var host string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "HANDLER PARSE ERR:", err)
			return
		}
		head += line + "\n"
		if strings.EqualFold(line[0:5], "HOST:") {
			host = strings.TrimSpace(line[5:])
			break
		}
	}
	conn, err := net.Dial("tcp", host+":80")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TCP CONN ERR:", err)
		return
	}
	go func() {
		conn.Write([]byte(head))
		io.Copy(conn, c)
	}()
	io.Copy(c, conn)
}

func main() {
	fmt.Println("STARTED")
	fmt.Fprintln(os.Stderr, "STARTED")

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
