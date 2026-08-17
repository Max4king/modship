// Package logging provides a global structured logger for modship.
package logging

import "go.uber.org/zap"

// L is the global logger. Initialize with Init() before use.
var L *zap.Logger = zap.NewNop() // default no-op so it's safe before Init

// Init creates and sets the global logger. development=true uses a human-readable
// console encoder; false uses JSON.
func Init(development bool) error {
	var logger *zap.Logger
	var err error
	if development {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		return err
	}
	L = logger
	return nil
}

// Sync flushes buffered logs. Call before exit.
func Sync() { _ = L.Sync() }
