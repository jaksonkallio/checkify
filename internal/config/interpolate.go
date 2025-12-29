package config

import (
	"fmt"
	"strings"
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

type InterpolatorSet struct {
	interpolators []*Interpolator
}

func NewInterpolatorSet(interpolators ...*Interpolator) *InterpolatorSet {
	return &InterpolatorSet{
		interpolators: interpolators,
	}
}

// InterpolateMany applies all interpolators in the set to the input string.
func (is *InterpolatorSet) InterpolateMany(input InterpolatedString) string {
	result := string(input)
	for _, interpolator := range is.interpolators {
		result = interpolator.Interpolate(InterpolatedString(result))
	}
	return result
}

// ContainsPlaceholder returns true if the string contains a placeholder for any of the interpolators in the set.
func (is *InterpolatorSet) ContainsPlaceholder(str string) bool {
	for _, interpolator := range is.interpolators {
		if interpolator.ContainsPlaceholder(str) {
			return true
		}
	}
	return false
}
