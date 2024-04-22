package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/docker/docker/image"
)

func main() {
	pathPtr := flag.String("path", "/tmp/docker-image-store/imagedb", "path to the image store")
	flag.Parse()

	imageDb, err := image.NewFSStoreBackend(*pathPtr)
	if err != nil {
		fmt.Printf("unable to initialize fs backend at %s: %s\n", *pathPtr, imageDb)
		os.Exit(1)
	}

	// TODO: get the image store by creating a LayerGetRelease like so:
	// layer.NewStoreFromOptions
	// More info in https://github.com/moby/moby/blob/master/daemon/daemon.go#L1167
}
