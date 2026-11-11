package provision

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// USBPaths checks presence without claiming interfaces, including boards that
// have left EDL after flashing. Paths use the same keys as CLI discovery.
func USBPaths(ctx context.Context) ([]string, error) {
	paths := []string{}
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.CommandContext(ctx, "/usr/sbin/ioreg", "-a", "-r", "-c", "IOUSBHostDevice").Output()
		if err != nil {
			return nil, err
		}
		if len(strings.TrimSpace(string(out))) == 0 {
			return paths, nil
		}
		var tree plist
		if err := xml.Unmarshal(out, &tree); err != nil {
			return nil, err
		}
		return plistUSBPaths(tree), nil
	case "linux":
		entries, err := os.ReadDir("/sys/bus/usb/devices")
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			path := filepath.Join("/sys/bus/usb/devices", entry.Name())
			if _, err := os.Stat(filepath.Join(path, "idVendor")); err == nil {
				paths = append(paths, path)
			}
		}
	case "windows":
		out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; ConvertTo-Json -InputObject @(Get-PnpDevice -PresentOnly | Where-Object { $_.InstanceId -like 'USB\*' } | ForEach-Object { $_.InstanceId }) -Compress`).Output()
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(string(out), "\ufeff")), &paths); err != nil {
			return nil, err
		}
	}
	return paths, ctx.Err()
}

func plistUSBPaths(tree plist) []string {
	paths := []string{}
	var visit func(plist)
	visit = func(node plist) {
		if node.XMLName.Local == "dict" {
			values := map[string]string{}
			for i := 0; i+1 < len(node.Nodes); i++ {
				if node.Nodes[i].XMLName.Local == "key" {
					values[node.Nodes[i].Text] = node.Nodes[i+1].Text
					i++
				}
			}
			if values["idVendor"] != "" {
				key := values["locationID"]
				if key == "" {
					key = values["IORegistryEntryID"]
				}
				if key != "" {
					paths = append(paths, "ioreg:"+key)
				}
			}
		}
		for _, child := range node.Nodes {
			visit(child)
		}
	}
	visit(tree)
	return paths
}
