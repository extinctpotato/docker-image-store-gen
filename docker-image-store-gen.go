package main

import (
	"fmt"

	"github.com/docker/docker/image"
)

func main() {
	fmt.Println("vim-go")

	image.NewFSStoreBackend("/tmp/cool")
}
