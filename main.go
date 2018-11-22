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
)

func main() {
	dockerCli, err := dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithVersion("1.38"))
	assertNil(err)

	// url := "http://example.com"
	ctx := context.Background()
	ctr, err := dockerCli.ContainerCreate(ctx, &container.Config{
		Image:        "node",
		Cmd:          []string{"node", "/server.js"},
		OpenStdin:    true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}, &container.HostConfig{
		NetworkMode: "host",
	}, nil, "")
	assertNil(err)
	defer dockerCli.ContainerRemove(ctx, ctr.ID, dockertypes.ContainerRemoveOptions{})

	serverJS := `#!/usr/bin/env node
const EventEmitter = require('events');
const http = require('http');

function stdinLineByLine() {
  const stdin = new EventEmitter();
  let buff = "";

  process.stdin
    .on('data', data => {
      buff += data;
      lines = buff.split(/[\r\n|\n]/);
      buff = lines.pop();
      lines.forEach(line => stdin.emit('line', line));
    })
    .on('end', () => {
      if (buff.length > 0) stdin.emit('line', buff);
    });

  return stdin;
}

const stdin = stdinLineByLine();
stdin.on('line', function(line) {
	const arr = line.split(/\s+/);
    if (arr.length != 2) {
        console.log(0, "PARSE", line);
    }
	const idx = arr[0];
	const url = arr[1];
	http.get(url, (resp) => {
	  resp.on('data', (chunk) => {
        console.log(idx, chunk.length);
        console.log(chunk.toString());
	  });
	  resp.on('end', () => {
		console.log(idx, 'END')
	  });
	}).on("error", (err) => {
	  console.log(idx, "ERROR", err.message);
	});
});
	`
	ioutil.WriteFile("/tmp/server.js", []byte(serverJS), 0755)
	r, err := CreateSingleFileTar("/server.js", serverJS)
	err = dockerCli.CopyToContainer(ctx, ctr.ID, "/", r, dockertypes.CopyToContainerOptions{})
	assertNil(err)

	res, err := dockerCli.ContainerAttach(ctx, ctr.ID, dockertypes.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: false,
		Logs:   false,
	})
	assertNil(err)
	defer res.Close()

	bodyChan, errChan := dockerCli.ContainerWait(ctx, ctr.ID, container.WaitConditionNextExit)
	dockerCli.ContainerStart(ctx, ctr.ID, dockertypes.ContainerStartOptions{})
	if err != nil {
		fmt.Println("ERR: proxyDockerHostPort: START:", err)
		return
	}

	cpErr := make(chan error)
	go func() {
		_, err := stdcopy.StdCopy(os.Stdout, os.Stderr, res.Reader)
		// _, err := io.Copy(os.Stdout, res.Reader)
		cpErr <- err
	}()

	// go func() { io.Copy(res.Conn, conn) }()
	go func() {
		fmt.Fprintln(res.Conn, "1 http://www.example.com")
		fmt.Fprintln(res.Conn, "2 http://www.sun.com")
		fmt.Fprintln(res.Conn, "3 http://www.google.com")
	}()

	select {
	case body := <-bodyChan:
		if body.StatusCode != 0 {
			fmt.Println("ERR: proxyDockerHostPort: failed with status code:", body.StatusCode)
		}
	case err := <-errChan:
		fmt.Println("ERR: proxyDockerHostPort:", err)
	}
	fmt.Println("DONE:", <-cpErr)
}

func assertNil(err error) {
	if err != nil {
		panic(err)
	}
}

func CreateSingleFileTar(path, txt string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: path, Size: int64(len(txt)), Mode: 0666}); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(txt)); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}
