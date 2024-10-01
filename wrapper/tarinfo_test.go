package wrapper

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/distribution/reference"
)

func TestPartialManifestErrorHandling(t *testing.T) {
	var tests = []struct {
		name    string
		input   string
		correct bool
	}{
		{"correct structure", `[{"Config":"a","RepoTags":["b"],"Layers":["c"]}]`, true},
		{"correct structure", `[{"Config":["a"],"RepoTags":"b","Layers":["c"]}]`, false},
		{"missing config", `[{"RepoTags":["b"],"Layers":["c"]}]`, false},
		{"missing tags", `[{"Config":"a","Layers":["c"]}]`, true},
		{"multiple elements", `[{"Config":"a","RepoTags":["b"],"Layers":["c"]},{"Config":"d","RepoTags":["e"],"Layers:["f"]}]`, false},
		{"unterminated json", `[{"Config":"a"`, false},
		{"garbage", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := partialManifestFromReader(strings.NewReader(tt.input))
			if err != nil && tt.correct {
				t.Errorf("error %s for correct input", err)
			}
			if err == nil && !tt.correct {
				t.Error("no error for incorrect input")
			}
		})
	}
}

func TestSeekedTarReader(t *testing.T) {
	buf := new(bytes.Buffer)
	tarContent := []byte("contents")
	tarWriter := tar.NewWriter(buf)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "a", Size: int64(len(tarContent))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(tarContent); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewReader(buf.Bytes())

	tr, err := seekedTarReader(reader, "a")
	if err != nil {
		t.Fatalf("error while looking for file in tar: %s", err)
	}
	testBuf := make([]byte, 100)
	rb, err := tr.Read(testBuf)
	if err != nil && err != io.EOF {
		t.Fatalf("error while reading tar: %s", err)
	}
	if rb != len(tarContent) {
		t.Fatalf("incorrect len: %d, content: %s", rb, testBuf)
	}
}

func TestBasicTarInfoWithHelloWorld(t *testing.T) {
	inTar, err := os.Open("../testdata/hello_world.tar")
	if err != nil {
		t.Fatal(err)
	}
	defer inTar.Close()

	bti, err := BasicTarInfoFromReader(inTar)
	if err != nil {
		t.Fatal(err)
	}

	if *bti.Tag != "docker.io/library/hello-world:linux" {
		t.Fatalf("tag mismatch: %s", *bti.Tag)
	}

	if bti.ImageId != "sha256:d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a" {
		t.Fatalf("digest mismatch: %s", bti.ImageId)
	}

	nameNoTag := reference.TrimNamed(bti.Ref).String()
	if nameNoTag != "docker.io/library/hello-world" {
		t.Fatalf("invalid name without tag: %s", nameNoTag)
	}

	withTag, err := bti.WithTag("cooltag")
	if err != nil {
		t.Fatal(err)
	}
	if withTag.String() != "docker.io/library/hello-world:cooltag" {
		t.Fatalf("invalid name with different tag: %s", withTag.String())
	}
}
