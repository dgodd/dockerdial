package grpcstdin

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockercli "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/yamux"
	"github.com/pkg/errors"
)

type grpcstdin struct {
	ctrID   string
	session *yamux.Session
}

func new(stderr io.Writer) (*grpcstdin, error) {
	fmt.Println("DG: NEW: 0")
	dockerCli, err := dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithVersion("1.38"))
	if err != nil {
		return nil, errors.Wrap(err, "grpcstdin: connect to docker:")
	}

	fmt.Println("DG: NEW: 1")
	s := &grpcstdin{}
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
	if err != nil {
		return nil, errors.Wrap(err, "grpcstdin: create container:")
	}
	s.ctrID = ctr.ID

	fmt.Println("DG: NEW: 2")
	res, err := dockerCli.ContainerAttach(ctx, ctr.ID, dockertypes.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		dockerCli.ContainerRemove(ctx, ctr.ID, dockertypes.ContainerRemoveOptions{})
		return nil, errors.Wrap(err, "grpcstdin: attach:")
	}

	err = dockerCli.ContainerStart(ctx, ctr.ID, dockertypes.ContainerStartOptions{})
	if err != nil {
		dockerCli.ContainerRemove(ctx, ctr.ID, dockertypes.ContainerRemoveOptions{})
		return nil, errors.Wrap(err, "grpcstdin: attach:")
	}

	fmt.Println("DG: NEW: 3")
	pr, pw := io.Pipe()
	go stdcopy.StdCopy(pw, stderr, res.Reader)

	fmt.Println("DG: NEW: 4")
	buf := make([]byte, 8)
	_, err = pr.Read(buf)
	if string(buf) != "STARTED\n" {
		res.Close()
		dockerCli.ContainerKill(ctx, ctr.ID, "SIGKILL")
		return nil, errors.New("grpcstdin: did not read started")
	}

	fmt.Println("DG: NEW: 5")
	s.session, err = yamux.Client(&StdinStdout{in: pr, out: res.Conn}, nil)
	if string(buf) != "STARTED\n" {
		res.Close()
		dockerCli.ContainerKill(ctx, ctr.ID, "SIGKILL")
		return nil, errors.New("grpcstdin: create session")
	}

	fmt.Println("DG: NEW: 6")
	return s, nil
}

var connOnce sync.Once
var connSingle *grpcstdin
var connErr error

func Dial() (io.ReadWriteCloser, error) {
	fmt.Println("DG: DIAL")
	connOnce.Do(func() {
		connSingle, connErr = new(os.Stderr)
	})
	if connErr != nil {
		return nil, errors.Wrap(connErr, "getting dial singleton")
	}
	fmt.Println("DG: DIAL SESSION:", connSingle.session)
	return connSingle.session.Open()
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
