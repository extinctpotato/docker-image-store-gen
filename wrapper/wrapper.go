package wrapper

import (
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/image"
	"github.com/docker/docker/image/tarexport"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/plugin"
	"github.com/docker/docker/reference"
	"github.com/extinctpotato/docker-image-store-gen/loggers"

	_ "github.com/docker/docker/daemon/graphdriver/overlay2"
)

type MinimalMoby struct {
	Root        string
	LayerStore  layer.Store
	ImageStore  image.Store
	RefStore    reference.Store
	TarExporter image.Exporter
}

func (m *MinimalMoby) Load(tarPath string, tagAsLatest bool) error {
	inTar, err := os.Open(tarPath)
	if err != nil {
		return err
	}

	info, err := BasicTarInfoFromReader(inTar)
	if err != nil {
		return err
	}
	if _, err := inTar.Seek(0, io.SeekStart); err != nil {
		return err
	}

	err = m.TarExporter.Load(inTar, new(loggers.TarExporterLoadLogger), false)
	if err != nil {
		return err
	}

	if tagAsLatest {
		newTag, err := info.WithTag("latest")
		if err != nil {
			return err
		}
		return m.RefStore.AddTag(newTag, info.ImageId.Digest(), false)
	}

	return nil
}

func (m *MinimalMoby) DumpStore(tarPath string, srcPath string) error {
	outputTar, err := os.Create(tarPath)
	if err != nil {
		return err
	}

	if srcPath == "" {
		srcPath = m.Root
	}

	arch, err := chrootarchive.Tar(srcPath, &archive.TarOptions{
		Compression: archive.Uncompressed,
	}, srcPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(outputTar, arch); err != nil {
		return err
	}

	return nil
}

func NewMinimalMoby(root string) (*MinimalMoby, error) {
	layerStore, err := layer.NewStoreFromOptions(layer.StoreOptions{
		Root:                      root,
		MetadataStorePathTemplate: filepath.Join(root, "image", "%s", "layerdb"),
		GraphDriver:               "overlay2",
		GraphDriverOptions:        nil,
		IDMapping:                 idtools.IdentityMapping{},
		PluginGetter:              plugin.NewStore(),
		ExperimentalEnabled:       true,
	})
	if err != nil {
		return nil, err
	}

	imageRoot := filepath.Join(root, "image", layerStore.DriverName())
	ifs, err := image.NewFSStoreBackend(filepath.Join(imageRoot, "imagedb"))
	if err != nil {
		return nil, err
	}

	refStoreLocation := filepath.Join(imageRoot, `repositories.json`)
	rs, err := reference.NewReferenceStore(refStoreLocation)
	if err != nil {
		return nil, err
	}

	imageStore, err := image.NewImageStore(ifs, layerStore)
	if err != nil {
		return nil, err
	}

	tarExporter := tarexport.NewTarExporter(imageStore, layerStore, rs, new(loggers.TarExporterLogger))

	return &MinimalMoby{
		Root:        root,
		LayerStore:  layerStore,
		ImageStore:  imageStore,
		RefStore:    rs,
		TarExporter: tarExporter,
	}, nil
}
