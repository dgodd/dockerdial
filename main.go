package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockercli "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/yamux"
)

func main() {
	fmt.Println("DG: MAIN: 0")
	dockerCli, err := dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithVersion("1.38"))
	assertNil(err)
	fmt.Println("DG: MAIN: 1")

	// url := "http://example.com"
	ctx := context.Background()
	ctr, err := dockerCli.ContainerCreate(ctx, &container.Config{
		Image:        "golang",
		Cmd:          []string{"go", "run", "server.go"},
		WorkingDir:   "/app",
		Env:          []string{"GO111MODULE=on"},
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
	fmt.Println("DG: MAIN: 2")

	r, err := CreateServerTar()
	assertNil(err)
	err = dockerCli.CopyToContainer(ctx, ctr.ID, "/", r, dockertypes.CopyToContainerOptions{})
	assertNil(err)
	fmt.Println("DG: MAIN: 3")

	_, err = dockerCli.ContainerCommit(ctx, ctr.ID, dockertypes.ContainerCommitOptions{Reference: "dg"})
	assertNil(err)

	res, err := dockerCli.ContainerAttach(ctx, ctr.ID, dockertypes.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	assertNil(err)
	defer res.Close()
	fmt.Println("DG: MAIN: 4")

	bodyChan, errChan := dockerCli.ContainerWait(ctx, ctr.ID, container.WaitConditionNextExit)
	dockerCli.ContainerStart(ctx, ctr.ID, dockertypes.ContainerStartOptions{})
	fmt.Println("DG: MAIN: 5")

	pr, pw := io.Pipe()
	go stdcopy.StdCopy(pw, os.Stderr, res.Reader)

	buf := make([]byte, 8)
	_, err = pr.Read(buf)
	assertNil(err)
	fmt.Printf("RECEIVED: |%s|", buf)

	session, err := yamux.Client(&StdinStdout{in: pr, out: res.Conn}, nil)
	assertNil(err)
	go func() {
		fmt.Println("DG: FUNC: 0")
		c, err := session.Open()
		if err != nil {
			fmt.Println("ERR:", err)
			return
		}
		fmt.Println("DG: FUNC: 1")

		_, err = c.Write([]byte("GET / HTTP/1.1\nConnection: close\nHost: example.com\n\n"))
		if err != nil {
			fmt.Println("ERR:", err)
			return
		}
		fmt.Println("DG: FUNC: 2")

		io.Copy(os.Stdout, c)
		fmt.Println("DG: FUNC: 3")
		c.Close()
		fmt.Println("DG: FUNC: 4")
	}()
	fmt.Println("DG: MAIN: 6")

	select {
	case body := <-bodyChan:
		if body.StatusCode != 0 {
			fmt.Println("ERR: proxyDockerHostPort: failed with status code:", body.StatusCode)
		}
	case err := <-errChan:
		fmt.Println("ERR: proxyDockerHostPort:", err)
	}
	fmt.Println("DG: MAIN: 7")
}

func assertNil(err error) {
	if err != nil {
		panic(err)
	}
}

func CreateServerTar() (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	b, err := ioutil.ReadFile("server.go")
	if err != nil {
		return nil, err
	}
	for path, txt := range map[string]string{
		"/app/go.mod":    "module myapp\n\nrequire (\ngithub.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d\n)\n",
		"/app/server.go": string(b),
	} {
		if err := tw.WriteHeader(&tar.Header{Name: path, Size: int64(len(txt)), Mode: 0666}); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(txt)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
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
