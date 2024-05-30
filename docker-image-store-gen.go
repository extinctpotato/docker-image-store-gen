package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
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
	log.G(context.TODO()).WithFields(log.Fields{"image": imageID, "ref": refName, "action": action}).Info("Event detected")
}

type NoValidIdMappingError struct {
	filename *string
	user     *user.User
}

func (e *NoValidIdMappingError) Error() string {
	return "no valid id map found in " + *e.filename + " for " + e.user.Uid
}

func removeString(s []string, unwanted string) []string {
	for i, v := range s {
		if v == unwanted {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func idMapping(filename string) (string, string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", "", fmt.Errorf("unable to obtain current user: %s", err)
	}

	file, err := os.Open("/etc/" + filename)
	if err != nil {
		return "", "", fmt.Errorf("failed to open id file %s: %s", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if mapping := strings.Split(scanner.Text(), ":"); len(mapping) == 3 {
			if mapping[0] == currentUser.Uid || mapping[0] == currentUser.Username {
				return mapping[1], mapping[2], nil
			}
		}
	}

	return "", "", &NoValidIdMappingError{&filename, currentUser}
}

func idMapCommand(idType string, pid int, currentId int) (*exec.Cmd, error) {
	idStart, count, err := idMapping("sub" + idType)
	if err != nil {
		return nil, err
	}
	return exec.Command("new"+idType+"map", strconv.Itoa(pid), "0", strconv.Itoa(currentId), "1", "1", idStart, count), nil
}

func main() {
	pathPtr := flag.String("path", "/tmp/docker-image-store", "path to the image store")
	tarPath := flag.String("tarpath", "/tmp/docker-image-store/test.tar", "path to the tar file to load")
	unshare := flag.Bool("unshare", false, "Run in a separate user and mount namespace")
	flag.Parse()

	fmt.Printf("ids: %d,%d,%d,%d\n", os.Getuid(), os.Geteuid(), os.Getgid(), os.Getegid())

	if *unshare {
		if os.Getuid() != 0 {
			cmd := exec.Command(os.Args[0], os.Args[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Unshareflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
			}
			if err := cmd.Start(); err != nil {
				fmt.Printf("unable to re-execute: %s\n", err)
				os.Exit(1)
			}

			newUidMap, err := idMapCommand("uid", cmd.Process.Pid, os.Getuid())
			if err != nil {
				fmt.Printf("uid mapping error: %s\n", err)
			}
			newGidMap, err := idMapCommand("gid", cmd.Process.Pid, os.Getgid())
			if err != nil {
				fmt.Printf("gid mapping error: %s\n", err)
			}
			newUidMap.Stdout = os.Stdout
			newUidMap.Stderr = os.Stderr
			newGidMap.Stderr = os.Stderr
			newGidMap.Stdout = os.Stdout
			newUidMap.Run()
			newGidMap.Run()

			if err := cmd.Wait(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
			}
			os.Exit(0)
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

	log.SetLevel("info")

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
