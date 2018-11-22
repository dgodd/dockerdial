package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockercli "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/yamux"
)

func main() {
	dockerCli, err := dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithVersion("1.38"))
	assertNil(err)

	// url := "http://example.com"
	ctx := context.Background()
	ctr, err := dockerCli.ContainerCreate(ctx, &container.Config{
		Image:        "dgodd/grpcstdinserver",
		OpenStdin:    true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}, &container.HostConfig{
		AutoRemove:  true,
		NetworkMode: "host",
	}, nil, "")
	assertNil(err)
	defer dockerCli.ContainerKill(ctx, ctr.ID, "SIGKILL")

	res, err := dockerCli.ContainerAttach(ctx, ctr.ID, dockertypes.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	assertNil(err)
	defer res.Close()

	bodyChan, errChan := dockerCli.ContainerWait(ctx, ctr.ID, container.WaitConditionNextExit)
	dockerCli.ContainerStart(ctx, ctr.ID, dockertypes.ContainerStartOptions{})

	pr, pw := io.Pipe()
	go stdcopy.StdCopy(pw, os.Stderr, res.Reader)

	buf := make([]byte, 8)
	_, err = pr.Read(buf)
	assertNil(err)
	if string(buf) != "STARTED\n" {
		fmt.Println("ERROR: STARTED not received")
		os.Exit(1)
	}
	fmt.Println("RECEIVED: STARTED")

	session, err := yamux.Client(&StdinStdout{in: pr, out: res.Conn}, nil)
	assertNil(err)
	go func() {
		c, err := session.Open()
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

		io.Copy(os.Stdout, c)
		c.Close()
	}()
	go func() {
		c, err := session.Open()
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

		io.Copy(os.Stdout, c)
		c.Close()
	}()

	select {
	case body := <-bodyChan:
		if body.StatusCode != 0 {
			fmt.Println("ERR: proxyDockerHostPort: failed with status code:", body.StatusCode)
		}
	case err := <-errChan:
		fmt.Println("ERR: proxyDockerHostPort:", err)
	}
	fmt.Println("DONE")
}

func assertNil(err error) {
	if err != nil {
		panic(err)
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
