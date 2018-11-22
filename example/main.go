package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/dgodd/dockerdial"
)

func main() {
	tr := &http.Transport{Dial: dockerdial.Dial}
	client := &http.Client{Transport: tr}

	resp, err := client.Get("http://example.com")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Printf("HEADERS: %#v\n", resp.Header)

	// Second Request
	fmt.Println("\ngoogle")
	resp, err = client.Get("http://google.com")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	io.Copy(ioutil.Discard, resp.Body)
	fmt.Printf("HEADERS: %#v\n", resp.Header)
}
