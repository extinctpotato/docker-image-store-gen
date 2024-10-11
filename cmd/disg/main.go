package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containerd/log"
	"github.com/extinctpotato/docker-image-store-gen/idmap"
	"github.com/extinctpotato/docker-image-store-gen/process"
	"github.com/extinctpotato/docker-image-store-gen/wrapper"
)

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

func archivesToImport(fileOrDir string) ([]string, error) {
	s, err := os.Stat(fileOrDir)
	if err != nil {
		return nil, err
	}
	switch mode := s.Mode(); {
	case mode.IsRegular():
		return []string{fileOrDir}, err
	case mode.IsDir():
		return filepath.Glob(fileOrDir + "/*.tar")
	default:
		return nil, nil
	}
}

func main() {
	pathPtr := flag.String("path", "/tmp/docker-image-store", "path to the image store")
	tarPath := flag.String("tarpath", "/tmp/docker-image-store/test.tar", "path to the tar file to load")
	outTar := flag.String("out", "/tmp/docker-image-store.tar", "path to the output tar file")
	unshare := flag.Bool("unshare", false, "Run in a separate user and mount namespace")
	latestify := flag.Bool("taglatest", false, "Add the :latest tag to the imported images")
	tarSrcPathOverride := flag.String("tar-src-override", "", "Override source directory for output tar")
	flag.Parse()

	ctx := context.Background()
	isNs := idmap.RunningInUserNS()
	log.G(ctx).WithFields(log.Fields{
		"uid":  os.Getuid(),
		"euid": os.Geteuid(),
		"gid":  os.Getgid(),
		"egid": os.Getegid(),
		"pid":  os.Getpid(),
		"isNs": isNs,
	}).Info("initializing")

	if *unshare {
		if !isNs {
			cmd, writePipe, err := process.NewFirstLevelReExec()
			if err != nil {
				log.G(ctx).WithError(err).Error("first level fail")
				os.Exit(1)
			}
			if err := cmd.Start(); err != nil {
				log.G(ctx).WithField("args", cmd.Args).WithError(err).Error("unable to re-execute")
				os.Exit(1)
			}

			if err := newIdMap(cmd.Process.Pid); err != nil {
				log.G(ctx).WithError(err).Error("error while mapping id")
				os.Exit(1)
			}

			writePipe.Close()

			if err := cmd.Wait(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
			}
		} else {
			err := process.WaitForPipe()
			if err != nil {
				log.G(ctx).WithError(err).Error("waiting for pipe failed")
				os.Exit(1)
			}

			if err := process.NewSecondLevelReExec().Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
			}
		}
		os.Exit(0)
	}

	// Create imagestore location if it doesn't exist
	if err := os.MkdirAll(*pathPtr, os.ModePerm); err != nil {
		fmt.Printf("unable to create imageStore directory: %s\n", err)
	}

	minMoby, err := wrapper.NewMinimalMoby(*pathPtr)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to initialize the wrapper")
		os.Exit(1)
	}

	toImport, err := archivesToImport(*tarPath)
	if err != nil {
		log.G(ctx).WithError(err).Error("invalid path")
		os.Exit(1)
	}

	for _, resolvedTarPath := range toImport {
		log.G(ctx).WithField("path", resolvedTarPath).Info("importing tar")
		if err := minMoby.Load(resolvedTarPath, *latestify); err != nil {
			log.G(ctx).WithError(err).Error("failed to load the archive")
			os.Exit(1)
		}
	}

	if err := minMoby.DumpStore(*outTar, *tarSrcPathOverride); err != nil {
		log.G(ctx).WithError(err).Error("failed to pack the store")
		os.Exit(1)
	}
}
