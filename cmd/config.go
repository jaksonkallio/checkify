package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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

type InterpolatedString string

type Interpolator struct {
	// Prefix is the dot prefix for this interpolator.
	Prefix string

	// Env is a map of actual environment variables.
	Values map[string]string
}

func NewInterpolator(prefix string) *Interpolator {
	return &Interpolator{
		Prefix: prefix,
		Values: make(map[string]string),
	}
}

func (ie *Interpolator) Set(key, value string) {
	ie.Values[key] = value
}

func (ie *Interpolator) Clear() {
	ie.Values = make(map[string]string)
}

func (ie *Interpolator) Interpolate(inputIStr InterpolatedString) string {
	input := string(inputIStr)

	for key, value := range ie.Values {
		placeholder := fmt.Sprintf("${%s.%s}", ie.Prefix, key)
		input = strings.ReplaceAll(string(input), placeholder, value)
	}

	return input
}

func (ie *Interpolator) ContainsPlaceholder(str string) bool {
	return strings.Contains(str, fmt.Sprintf("${%s.", ie.Prefix))
}

func InterpolateMany(input InterpolatedString, interpolators ...*Interpolator) string {
	result := string(input)
	for _, interpolator := range interpolators {
		result = interpolator.Interpolate(InterpolatedString(result))
	}
	return result
}

func ContainsInterpolatorPlaceholder(str string, interpolators ...*Interpolator) bool {
	for _, interpolator := range interpolators {
		if interpolator.ContainsPlaceholder(str) {
			return true
		}
	}
	return false
}

func loadConfig(path string) (Config, error) {
	var config Config
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(fileBytes, &config)
	return config, err
}
