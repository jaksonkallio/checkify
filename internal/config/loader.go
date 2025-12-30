package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// FileReader interface allows mocking file I/O for testing
type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

// OSFileReader provides access to real OS file system
type OSFileReader struct{}

// ReadFile reads a file from the OS file system
func (o OSFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// LoadWithReader loads configuration from a file using the provided FileReader
func LoadWithReader(path string, reader FileReader) (Config, error) {
	var config Config
	fileBytes, err := reader.ReadFile(path)
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(fileBytes, &config)
	return config, err
}
