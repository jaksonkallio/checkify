// Package config provides configuration loading and parsing, with basic config validation.
package config

import (
	"time"
)

type Config struct {
	Prometheus   Prometheus    `yaml:"prometheus"`
	Environments []Environment `yaml:"environments"`
	Checks       []Check       `yaml:"checks"`
}

type Prometheus struct {
	Port int    `yaml:"port"`
	Path string `yaml:"path"`
}

func (p Prometheus) Enabled() bool {
	return p.Port > 0
}

type Environment struct {
	Name string                        `yaml:"name"`
	Vars map[string]InterpolatedString `yaml:"vars"`
}

type Check struct {
	Name           string                        `yaml:"name"`
	Timeout        time.Duration                 `yaml:"timeout"`
	Interval       time.Duration                 `yaml:"interval"`
	Environments   []string                      `yaml:"environments"`
	Endpoint       InterpolatedString            `yaml:"endpoint"`
	Method         string                        `yaml:"method"`
	Headers        map[string]InterpolatedString `yaml:"headers"`
	Tags           []string                      `yaml:"tags"`
	ResponseFormat ResponseFormat                `yaml:"response_format"`
	Assertion      string                        `yaml:"assertion"`
}

type ResponseFormat string

const (
	FormatJSON   ResponseFormat = "json"
	FormatString ResponseFormat = "string"
	FormatBinary ResponseFormat = "binary"
	FormatNone   ResponseFormat = "none"
)

func Load(path string) (Config, error) {
	return LoadWithReader(path, OSFileReader{})
}
