// Package config provides configuration loading and parsing, with basic config validation.
package config

import (
	"time"
)

type Checks struct {
	HTTP []HTTPCheck `yaml:"http"`
	Ping []PingCheck `yaml:"ping"`
	TCP  []TCPCheck  `yaml:"tcp"`
	DNS  []DNSCheck  `yaml:"dns"`
}

type Config struct {
	Prometheus   Prometheus    `yaml:"prometheus"`
	Environments []Environment `yaml:"environments"`
	Checks       Checks        `yaml:"checks"`
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

type HTTPCheck struct {
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

type PingCheck struct {
	Name         string             `yaml:"name"`
	Timeout      time.Duration      `yaml:"timeout"`
	Count        int                `yaml:"count"`
	Interval     time.Duration      `yaml:"interval"`
	Environments []string           `yaml:"environments"`
	Host         InterpolatedString `yaml:"host"`
	Tags         []string           `yaml:"tags"`
	IPStack      string             `yaml:"ip_stack"`
	Assertion    string             `yaml:"assertion"`
}

type TCPCheck struct {
	Name         string             `yaml:"name"`
	Timeout      time.Duration      `yaml:"timeout"`
	Interval     time.Duration      `yaml:"interval"`
	Environments []string           `yaml:"environments"`
	Host         InterpolatedString `yaml:"host"`
	Port         int                `yaml:"port"`
	IPStack      string             `yaml:"ip_stack"`
	Tags         []string           `yaml:"tags"`
}

type DNSCheck struct {
	Name         string             `yaml:"name"`
	Timeout      time.Duration      `yaml:"timeout"`
	Interval     time.Duration      `yaml:"interval"`
	Environments []string           `yaml:"environments"`
	Query        InterpolatedString `yaml:"query"`
	RecordType   string             `yaml:"record_type"`
	Host         InterpolatedString `yaml:"host"`
	Port         int                `yaml:"port"`
	IPStack      string             `yaml:"ip_stack"`
	Tags         []string           `yaml:"tags"`
	Assertion    string             `yaml:"assertion"`
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
