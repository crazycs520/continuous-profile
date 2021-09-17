// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logutil

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// DefaultLogMaxSize is the default size of log files.
	DefaultLogMaxSize = 300 // MB
)

// EmptyFileLogConfig is an empty FileLogConfig.
var EmptyFileLogConfig = log.FileLogConfig{}

// NewFileLogConfig creates a FileLogConfig.
func NewFileLogConfig(maxSize uint) log.FileLogConfig {
	return log.FileLogConfig{
		MaxSize: int(maxSize),
	}
}

// LogConfig serializes log related config in toml/json.
type LogConfig struct {
	log.Config
}

// NewLogConfig creates a LogConfig.
func NewLogConfig(level string, fileCfg log.FileLogConfig) *LogConfig {
	return &LogConfig{
		Config: log.Config{
			Level:            level,
			Format:           "text",
			DisableTimestamp: false,
			File:             fileCfg,
		},
	}
}

// InitLogger initializes a logger with cfg.
func InitLogger(cfg *LogConfig) error {
	gl, props, err := log.InitLogger(&cfg.Config, zap.AddStacktrace(zapcore.FatalLevel))
	if err != nil {
		return errors.Trace(err)
	}
	log.ReplaceGlobals(gl, props)

	return nil
}

// SetLevel sets the zap logger's level.
func SetLevel(level string) error {
	l := zap.NewAtomicLevel()
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return errors.Trace(err)
	}
	log.SetLevel(l.Level())
	return nil
}

// BgLogger is alias of `logutil.BgLogger()`
func BgLogger() *zap.Logger {
	return log.L()
}
