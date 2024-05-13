package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/containerd/log"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/tarexport"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/plugin"
	refstore "github.com/docker/docker/reference"

	_ "github.com/docker/docker/daemon/graphdriver/overlay2"
)

type CustomLogger struct {
}

func (l *CustomLogger) LogImageEvent(imageID, refName string, action events.Action) {
	fmt.Printf("Event detected on imageID %s, refName %s with action %s", imageID, refName, action)
}

func main() {
	pathPtr := flag.String("path", "/tmp/docker-image-store", "path to the image store")
	tarPath := flag.String("tarpath", "/tmp/docker-image-store/test.tar", "path to the tar file to load")
	unshare := flag.Bool("unshare", false, "Run in a separate user and mount namespace")
	flag.Parse()

	fmt.Printf("My uid: %d\n", os.Getuid())

	if *unshare && os.Getuid() != 0 {
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Unshareflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
			UidMappings:  []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
			GidMappings:  []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
		}
		if err := cmd.Run(); err != nil {
			fmt.Printf("unable to re-execute: %s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Create imagestore location if it doesn't exist
	if err := os.MkdirAll(*pathPtr, os.ModePerm); err != nil {
		fmt.Printf("unable to create imageStore directory: %s\n", err)
	}

	log.SetLevel("warn")

	pluginStore := plugin.NewStore()

	layerStore, err := layer.NewStoreFromOptions(layer.StoreOptions{
		Root:                      *pathPtr,
		MetadataStorePathTemplate: filepath.Join(*pathPtr, "image", "%s", "layerdb"),
		GraphDriver:               "overlay2",
		GraphDriverOptions:        nil,
		IDMapping:                 idtools.IdentityMapping{},
		PluginGetter:              pluginStore,
		ExperimentalEnabled:       true,
	})
	if err != nil {
		fmt.Printf("unable to initialize layerStore: %s\n", err)
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
	tarToLoad, err := os.Open(*tarPath)
	if err != nil {
		fmt.Printf("unable to open %s: %s\n", *tarPath, err)
	}
	if err = tarExporter.Load(tarToLoad, os.Stderr, false); err != nil {
		fmt.Printf("unable to load tar: %s\n", err)
	}
}
