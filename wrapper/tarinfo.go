package wrapper

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"io"
	"io/fs"

	"github.com/distribution/reference"
	"github.com/docker/docker/image"
	"github.com/opencontainers/go-digest"
)

type BasicTarInfo struct {
	ImageId image.ID
	Tag     *string
	Ref     reference.Named
}

type partialManifest struct {
	Config   string
	RepoTags []string
}

func seekedTarReader(r io.ReadSeeker, name string) (*tar.Reader, error) {
	// Seek to beginning since reader might have been used
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	t := tar.NewReader(r)
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Name == name {
			return t, nil
		}
	}

	return nil, fs.ErrNotExist
}

func partialManifestFromReader(r io.Reader) (*partialManifest, error) {
	var pm partialManifest
	dec := json.NewDecoder(r)
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if err := dec.Decode(&pm); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("ambiguous manifest")
	}
	if pm.Config == "" {
		return nil, errors.New("missing config value")
	}
	return &pm, nil
}

func BasicTarInfoFromReader(r io.ReadSeeker) (*BasicTarInfo, error) {
	t, err := seekedTarReader(r, "manifest.json")
	if err != nil {
		return nil, err
	}

	partialManifest, err := partialManifestFromReader(t)
	if err != nil {
		return nil, err
	}

	t, err = seekedTarReader(r, partialManifest.Config)
	if err != nil {
		return nil, err
	}

	dgst, err := digest.FromReader(t)
	if err != nil {
		return nil, err
	}

	var tag *string
	if len(partialManifest.RepoTags) > 0 {
		tag = &partialManifest.RepoTags[0]
	}

	ref, err := reference.ParseDockerRef(*tag)
	if err != nil {
		return nil, err
	}

	return &BasicTarInfo{
		ImageId: image.ID(dgst),
		Tag:     tag,
		Ref:     ref,
	}, nil
}
