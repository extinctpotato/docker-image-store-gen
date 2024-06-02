package process

import (
	"io"
	"os"
)

const internalPipeName = "eof_pipe"

func WaitForPipe() error {
	p := os.NewFile(uintptr(3), internalPipeName) // first extra FD passed by parent
	defer p.Close()
	buf := make([]byte, 1) // throwaway, don't care what's in there
	if _, err := p.Read(buf); err != io.EOF {
		return err
	}
	return nil
}
