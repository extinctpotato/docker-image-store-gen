package loggers

import (
	"context"
	"encoding/json"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/pkg/jsonmessage"
)

// TarExporterLogger represents a custom logging adapter for image.Exporter.
// It implements tarexport.LogImageEvent from github.com/docker/docker/image/tarexport.
type TarExporterLogger struct {
}

func (l *TarExporterLogger) LogImageEvent(imageID, refName string, action events.Action) {
	log.G(context.TODO()).WithFields(log.Fields{"image": imageID, "ref": refName, "action": action}).Info("Event detected")
}

// TarExporterLoadLogger is used to log the real-time events related to importing containers from tar archives.
type TarExporterLoadLogger struct {
}

func (l TarExporterLoadLogger) Write(p []byte) (int, error) {
	var jsonMsg jsonmessage.JSONMessage
	baseLogger := log.G(context.TODO()).WithField("module", "tar-loader")
	if err := json.Unmarshal(p, &jsonMsg); err != nil {
		baseLogger.WithError(err).Warnln(string(p))
		return 0, err
	}
	if jsonMsg.Progress != nil {
		baseLogger.WithFields(log.Fields{
			"current": jsonMsg.Progress.Current,
			"total":   jsonMsg.Progress.Total,
		}).Infoln(jsonMsg.Status)
	} else {
		baseLogger.Infoln(jsonMsg.Stream)
	}
	return len(p), nil
}
