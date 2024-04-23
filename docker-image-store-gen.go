package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/tarexport"
	"github.com/docker/docker/layer"
	refstore "github.com/docker/docker/reference"
)

type CustomLogger struct {
}

func (l *CustomLogger) LogImageEvent(imageID, refName string, action events.Action) {
	fmt.Printf("Event detected on imageID %s, refName %s with action %s", imageID, refName, action)
}

func main() {
	pathPtr := flag.String("path", "/tmp/docker-image-store", "path to the image store")
	tarPath := flag.String("tarpath", "/tmp/docker-image-store/test.tar", "path to the tar file to load")
	flag.Parse()

	layerStore, err := layer.NewStoreFromOptions(layer.StoreOptions{
		Root:                      *pathPtr,
		MetadataStorePathTemplate: filepath.Join(*pathPtr, "image", "%s", "layerdb"),
		GraphDriver:               "overlay2",
		ExperimentalEnabled:       false,
	})
	if err != nil {
		fmt.Printf("unable to initialize layerStore\n")
		os.Exit(1)
	}

	imageRoot := filepath.Join(*pathPtr, "imagedb")
	imageDb, err := image.NewFSStoreBackend(imageRoot)
	if err != nil {
		fmt.Printf("unable to initialize fs backend at %s: %s\n", *pathPtr, imageDb)
		os.Exit(1)
	}

	refStoreLocation := filepath.Join(imageRoot, `repositories.json`)
	rs, err := refstore.NewReferenceStore(refStoreLocation)
	if err != nil {
		fmt.Printf("couldn't create reference store repository: %s", err)
		os.Exit(1)
	}

	imageStore, err := image.NewImageStore(imageDb, layerStore)
	if err != nil {
		fmt.Printf("couldn't create image store: %s\n", err)
		os.Exit(1)
	}

	tarExporter := tarexport.NewTarExporter(imageStore, layerStore, rs, new(CustomLogger))
	_ = tarExporter
	_ = tarPath
}
