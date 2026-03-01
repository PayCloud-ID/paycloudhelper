package paycloudhelper

import (
	"errors"
	"testing"
)

func TestLoggerErrorHub_NoPanic(t *testing.T) {
	LoggerErrorHub("plain string")
	LoggerErrorHub(errors.New("error value"))
	LoggerErrorHub("format %s", "arg")
}
