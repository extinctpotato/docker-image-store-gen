package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/containerd/log"
	"github.com/extinctpotato/docker-image-store-gen/idmap"
	"github.com/extinctpotato/docker-image-store-gen/loggers"

	"github.com/docker/docker/image"
	"github.com/docker/docker/image/tarexport"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/plugin"
	refstore "github.com/docker/docker/reference"

	_ "github.com/docker/docker/daemon/graphdriver/overlay2"
)

func removeString(s []string, unwanted string) []string {
	for i, v := range s {
		if v == unwanted {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func newIdMap(pid int) error {
	uidInfo, err := idmap.NewUidInfo()
	if err != nil {
		return fmt.Errorf("uid info error: %s", err)
	}
	gidInfo, err := idmap.NewGidInfo()
	if err != nil {
		return fmt.Errorf("gid info error: %s", err)
	}

	newUidMap, err := uidInfo.SubrangeCmd(pid)
	if err != nil {
		return fmt.Errorf("failed to initialize uidmap: %s", err)
	}
	newGidMap, err := gidInfo.SubrangeCmd(pid)
	if err != nil {
		return fmt.Errorf("failed to initialize gidmap: %s", err)
	}

	newUidMap.Run()
	newGidMap.Run()
	return nil
}

func main() {
	pathPtr := flag.String("path", "/tmp/docker-image-store", "path to the image store")
	tarPath := flag.String("tarpath", "/tmp/docker-image-store/test.tar", "path to the tar file to load")
	unshare := flag.Bool("unshare", false, "Run in a separate user and mount namespace")
	flag.Parse()

	isNs := idmap.RunningInUserNS()
	log.G(context.TODO()).WithFields(log.Fields{
		"uid":  os.Getuid(),
		"euid": os.Geteuid(),
		"gid":  os.Getgid(),
		"egid": os.Getegid(),
		"pid":  os.Getpid(),
		"isNs": isNs,
	}).Info("initializing")

	if *unshare {
		if !isNs {
			readPipe, writePipe, _ := os.Pipe()
			cmd := exec.Command(os.Args[0], os.Args[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Unshareflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
			}
			cmd.ExtraFiles = []*os.File{readPipe}
			if err := cmd.Start(); err != nil {
				log.G(context.TODO()).WithField("args", cmd.Args).WithError(err).Error("unable to re-execute")
				os.Exit(1)
			}

			if err := newIdMap(cmd.Process.Pid); err != nil {
				log.G(context.Background()).WithError(err).Error("error while mapping id")
				os.Exit(1)
			}

			writePipe.Close()

			if err := cmd.Wait(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
			}
			os.Exit(0)
		}
		readPipe := os.NewFile(uintptr(3), "pipe")
		log.G(context.TODO()).Info("reading from pipe")
		buf := make([]byte, 1)
		_, err := readPipe.Read(buf)
		if err != nil {
			log.G(context.TODO()).Info("pipe fell through")
		}

		cmd := exec.Command(os.Args[0], removeString(os.Args[1:], "-unshare")...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
		}
		os.Exit(0)
	}

	// Create imagestore location if it doesn't exist
	if err := os.MkdirAll(*pathPtr, os.ModePerm); err != nil {
		fmt.Printf("unable to create imageStore directory: %s\n", err)
	}

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

	tarExporter := tarexport.NewTarExporter(imageStore, layerStore, rs, new(loggers.TarExporterLogger))
	tarToLoad, err := os.Open(*tarPath)
	if err != nil {
		fmt.Printf("unable to open %s: %s\n", *tarPath, err)
	}
	if err = tarExporter.Load(tarToLoad, new(loggers.TarExporterLoadLogger), false); err != nil {
		fmt.Printf("unable to load tar: %s\n", err)
	}
}
