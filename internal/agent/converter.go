// Package agent provides the BlazeLog agent implementation.
package agent

import (
	"github.com/good-yellow-bee/blazelog/internal/models"
	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ToProtoLogEntry converts a models.LogEntry to a protobuf LogEntry.
func ToProtoLogEntry(entry *models.LogEntry) *blazelogv1.LogEntry {
	if entry == nil {
		return nil
	}

	protoEntry := &blazelogv1.LogEntry{
		Timestamp:  timestamppb.New(entry.Timestamp),
		Level:      ToProtoLogLevel(entry.Level),
		Message:    entry.Message,
		Source:     entry.Source,
		Type:       ToProtoLogType(entry.Type),
		Raw:        entry.Raw,
		Labels:     entry.Labels,
		LineNumber: entry.LineNumber,
		FilePath:   entry.FilePath,
	}

	// Convert Fields map to protobuf Struct
	if len(entry.Fields) > 0 {
		fields, err := structpb.NewStruct(entry.Fields)
		if err == nil {
			protoEntry.Fields = fields
		}
	}

	return protoEntry
}

// ToProtoLogLevel converts a models.LogLevel to a protobuf LogLevel.
func ToProtoLogLevel(level models.LogLevel) blazelogv1.LogLevel {
	switch level {
	case models.LevelDebug:
		return blazelogv1.LogLevel_LOG_LEVEL_DEBUG
	case models.LevelInfo:
		return blazelogv1.LogLevel_LOG_LEVEL_INFO
	case models.LevelWarning:
		return blazelogv1.LogLevel_LOG_LEVEL_WARNING
	case models.LevelError:
		return blazelogv1.LogLevel_LOG_LEVEL_ERROR
	case models.LevelFatal:
		return blazelogv1.LogLevel_LOG_LEVEL_FATAL
	default:
		return blazelogv1.LogLevel_LOG_LEVEL_UNSPECIFIED
	}
}

// ToProtoLogType converts a models.LogType to a protobuf LogType.
func ToProtoLogType(logType models.LogType) blazelogv1.LogType {
	switch logType {
	case models.LogTypeNginx:
		return blazelogv1.LogType_LOG_TYPE_NGINX
	case models.LogTypeApache:
		return blazelogv1.LogType_LOG_TYPE_APACHE
	case models.LogTypeMagento:
		return blazelogv1.LogType_LOG_TYPE_MAGENTO
	case models.LogTypePrestaShop:
		return blazelogv1.LogType_LOG_TYPE_PRESTASHOP
	case models.LogTypeWordPress:
		return blazelogv1.LogType_LOG_TYPE_WORDPRESS
	default:
		return blazelogv1.LogType_LOG_TYPE_UNSPECIFIED
	}
}

// ToProtoLogSource converts a SourceConfig to a protobuf LogSource.
// Note: LogSource.Type is a string field in the proto, not an enum.
func ToProtoLogSource(name, path, sourceType string, follow bool) *blazelogv1.LogSource {
	return &blazelogv1.LogSource{
		Name:   name,
		Path:   path,
		Type:   sourceType,
		Follow: follow,
	}
}
