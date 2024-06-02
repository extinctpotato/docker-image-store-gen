package idmap

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/extinctpotato/docker-image-store-gen/loggers"
)

type IdKind string

const (
	UidIdKind IdKind = "uid"
	GidIdKind IdKind = "gid"
)

type IdInfo struct {
	Id    string
	Name  string
	IdMap IdKind
}

func (i IdInfo) subrange() (string, string, error) {
	filename := "/etc/sub" + string(i.IdMap)
	file, err := os.Open(filename)
	if err != nil {
		return "", "", fmt.Errorf("failed to open id file %s: %s", filename, err)
	}
	defer file.Close()
	return idSubrange(file, i)
}

func (i IdInfo) SubrangeCmd(pid int) (*exec.Cmd, error) {
	idStart, count, err := i.subrange()
	if err != nil {
		return nil, err
	}
	commandName := "new" + string(i.IdMap) + "map"
	cmd := exec.Command(commandName, strconv.Itoa(pid), "0", i.Id, "1", "1", idStart, count)
	cmd.Stderr = loggers.NewStderrIdmapLogger(commandName)
	cmd.Stdout = loggers.NewStdoutIdmapLogger(commandName)
	return cmd, nil
}

func NewUidInfo() (*IdInfo, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	return &IdInfo{
		Id:    currentUser.Uid,
		Name:  currentUser.Username,
		IdMap: UidIdKind,
	}, nil
}

func NewGidInfo() (*IdInfo, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	primaryGroup, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		return nil, err
	}
	return &IdInfo{
		Id:    primaryGroup.Gid,
		Name:  primaryGroup.Name,
		IdMap: GidIdKind,
	}, nil
}

func idSubrange(r io.Reader, i IdInfo) (string, string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if mapping := strings.Split(scanner.Text(), ":"); len(mapping) == 3 {
			if mapping[0] == i.Id || mapping[0] == i.Name {
				return mapping[1], mapping[2], nil
			}
		}
	}

	return "", "", &NoValidIdMappingError{i}
}

type NoValidIdMappingError struct {
	IdInfo IdInfo
}

func (e *NoValidIdMappingError) Error() string {
	return fmt.Sprintf("no valid %s map found for <%s:%s>", e.IdInfo.IdMap, e.IdInfo.Id, e.IdInfo.Name)
}
