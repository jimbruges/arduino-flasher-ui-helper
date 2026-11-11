package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/jimbruges/arduino-flasher-ui-helper/internal/provision"
	"github.com/jimbruges/arduino-flasher-ui-helper/internal/updates"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend
var assets embed.FS

//go:embed packaging/icons/app-icon.png
var appIcon []byte

type App struct {
	smokeReport string
	engine      *provision.Engine
	ctx         context.Context
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.engine.SetEmitter(func(e provision.Event) { runtime.EventsEmit(ctx, "provision:event", e) })
}
func (a *App) Status() provision.Status {
	s := a.engine.Status()
	s.Smoke = a.smokeReport != ""
	return s
}

// ReportUISmoke records a demo-only native bridge test, never hardware work.
func (a *App) ReportUISmoke(report string) error {
	if a.smokeReport == "" || !a.engine.Status().Demo {
		return nil
	}
	return os.WriteFile(a.smokeReport, []byte(report), 0600)
}
func (a *App) AppVersion() string { return updates.BuildVersion() }
func (a *App) CheckUpdates() (updates.Info, error) {
	if a.smokeReport != "" {
		return updates.Info{Current: updates.BuildVersion(), Latest: updates.BuildVersion(), Comparable: true}, nil
	}
	return updates.Check(a.ctx)
}
func (a *App) OpenRelease(tag string) error {
	link, err := updates.ReleaseURL(tag)
	if err != nil {
		return err
	}
	runtime.BrowserOpenURL(a.ctx, link)
	return nil
}
func (a *App) USBPaths() ([]string, error) {
	if a.engine.Status().Demo {
		devices, err := a.engine.Devices(a.ctx)
		paths := []string{}
		for _, device := range devices {
			paths = append(paths, device.Path)
		}
		return paths, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	return provision.USBPaths(ctx)
}
func (a *App) Devices() ([]provision.Device, error)             { return a.engine.Devices(a.ctx) }
func (a *App) Image(path string) (provision.ImageInfo, error)   { return provision.InspectImage(path) }
func (a *App) Plan(r provision.Request) (provision.Plan, error) { return a.engine.Plan(r) }
func (a *App) FixtureDevices(fixture string) ([]provision.Device, error) {
	return a.engine.FixtureDevices(a.ctx, fixture)
}
func (a *App) Start(r provision.Request) (string, error) { return a.engine.Start(r) }
func (a *App) Cancel()                                   { a.engine.Cancel() }
func (a *App) Browse(kind string) (string, error) {
	switch kind {
	case "folder":
		return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a golden image or extracted factory image"})
	case "save":
		return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "Save golden image in a new folder", DefaultFilename: "golden-image"})
	case "fixture":
		return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose calibrated USB fixture", Filters: []runtime.FileFilter{{DisplayName: "USB fixture", Pattern: "*.json"}}})
	case "programmer":
		return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose Firehose programmer", Filters: []runtime.FileFilter{{DisplayName: "Firehose programmer", Pattern: "*.elf"}}})
	default:
		return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose factory image archive", Filters: []runtime.FileFilter{{DisplayName: "Factory image archive", Pattern: "*.tar.zst;*.tar.gz;*.tgz;*.tar;*.zip"}}})
	}
}
func (a *App) OpenGuide(board string) {
	url := "https://docs.arduino.cc/tutorials/uno-q/update-image/"
	if board == "ventunoq" {
		url = "https://docs.arduino.cc/tutorials/ventuno-q/flash-image/"
	}
	runtime.BrowserOpenURL(a.ctx, url)
}
func (a *App) RevealLogs() {
	path := a.engine.Status().Logs
	if path != "" {
		runtime.BrowserOpenURL(a.ctx, "file://"+path)
	}
}
func main() {
	demo := flag.Bool("demo", false, "Preview the wizard with simulated boards; hardware commands are disabled")
	tools := flag.String("tools", defaultTools(), "Directory containing the external flasher CLI, QDL and optional digest helper")
	smoke := flag.String("smoke-report", "", "Write a demo-only native UI smoke-test report")
	flag.Parse()
	if *smoke != "" && !*demo {
		fmt.Fprintln(os.Stderr, "--smoke-report requires --demo")
		os.Exit(2)
	}
	engine, err := provision.NewEngine(*tools, *demo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	app := &App{engine: engine, smokeReport: *smoke}
	frontend, _ := fs.Sub(assets, "frontend")
	err = wails.Run(&options.App{Title: "Arduino Flasher UI Helper", Width: 1180, Height: 820, MinWidth: 920, MinHeight: 680, Linux: &linux.Options{Icon: appIcon, ProgramName: "arduino-flasher-ui-helper", WebviewGpuPolicy: linux.WebviewGpuPolicyNever}, BackgroundColour: options.NewRGB(246, 247, 244), AssetServer: &assetserver.Options{Assets: frontend}, Bind: []interface{}{app}, OnStartup: app.Startup, OnBeforeClose: func(ctx context.Context) bool {
		if !engine.Status().Running {
			return false
		}
		choice, _ := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{Type: runtime.QuestionDialog, Title: "Flashing is still running", Message: "Closing stops the current transfers. An interrupted board may need to be flashed again.", Buttons: []string{"Keep flashing", "Stop and close"}, DefaultButton: "Keep flashing", CancelButton: "Keep flashing"})
		if choice != "Stop and close" {
			return true
		}
		if !engine.Stop() {
			return true
		}
		return false
	}})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func defaultTools() string {
	if path := os.Getenv("ARDUINO_FLASHER_UI_TOOLS_DIR"); path != "" {
		return path
	}
	executable, _ := os.Executable()
	base := filepath.Dir(executable)
	for _, path := range []string{filepath.Join(base, "..", "Resources", "tools"), filepath.Join(base, "tools"), "tools"} {
		if info, err := os.Stat(filepath.Join(path, provision.ToolName("arduino-flasher-cli"))); err == nil && !info.IsDir() {
			abs, _ := filepath.Abs(path)
			return abs
		}
	}
	return "tools"
}
