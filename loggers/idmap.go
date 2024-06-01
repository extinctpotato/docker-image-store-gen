package loggers

import (
	"bufio"
	"bytes"
	"context"

	"github.com/containerd/containerd/log"
)

type idMapLogger struct {
	command    string
	loggerFunc func(...interface{})
}

func (l idMapLogger) Write(p []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewBuffer(p))
	for scanner.Scan() {
		l.loggerFunc(scanner.Text())
	}
	return len(p), nil
}

func idMapBaseLogger(command string) *log.Entry {
	return log.G(context.TODO()).WithFields(log.Fields{"idmap": command})
}

func NewStderrIdmapLogger(command string) *idMapLogger {
	return &idMapLogger{
		command:    command,
		loggerFunc: idMapBaseLogger(command).Error,
	}
}

func NewStdoutIdmapLogger(command string) *idMapLogger {
	return &idMapLogger{
		command:    command,
		loggerFunc: idMapBaseLogger(command).Info,
	}
}
