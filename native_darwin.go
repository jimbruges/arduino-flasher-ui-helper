//go:build darwin

package main

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
*/
import "C"

// Wails native file dialogs require UniformTypeIdentifiers on current SDKs.
