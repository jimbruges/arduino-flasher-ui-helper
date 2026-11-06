package provision

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type plist struct {
	XMLName xml.Name
	Text    string  `xml:",chardata"`
	Nodes   []plist `xml:",any"`
}

func systemDevices(ctx context.Context) ([]Device, error) {
	var found []Device
	switch runtime.GOOS {
	case "linux":
		paths, err := filepath.Glob("/sys/bus/usb/devices/*")
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			v, _ := os.ReadFile(filepath.Join(path, "idVendor"))
			p, _ := os.ReadFile(filepath.Join(path, "idProduct"))
			if strings.TrimSpace(string(v)) != "05c6" || strings.TrimSpace(string(p)) != "9008" {
				continue
			}
			product, _ := os.ReadFile(filepath.Join(path, "product"))
			match := sourceSerial.FindStringSubmatch(string(product))
			if len(match) != 2 {
				return nil, fmt.Errorf("EDL device at %s has no selectable serial.", path)
			}
			found = append(found, Device{Serial: "0x" + match[1], Product: strings.TrimSpace(string(product)), Path: path})
		}
	case "windows":
		const script = `$ErrorActionPreference='Stop'; $items=@(Get-PnpDevice -PresentOnly | Where-Object { $_.InstanceId -match '^USB\\VID_05C6&PID_9008' -and $_.InstanceId -notmatch '&MI_[0-9A-F]{2}' } | ForEach-Object { $d=Get-PnpDeviceProperty -InstanceId $_.InstanceId -KeyName 'DEVPKEY_Device_BusReportedDeviceDesc'; [pscustomobject]@{ Product=[string]$d.Data; Path=$_.InstanceId } }); ConvertTo-Json -InputObject $items -Compress`
		out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
		if err != nil {
			return nil, err
		}
		return parseWindowsDevices(out)
	case "darwin":
		out, err := exec.CommandContext(ctx, "/usr/sbin/ioreg", "-a", "-r", "-c", "IOUSBHostDevice").Output()
		if err != nil {
			return nil, err
		}
		if len(strings.TrimSpace(string(out))) == 0 {
			return []Device{}, nil
		}
		var tree plist
		if err := xml.Unmarshal(out, &tree); err != nil {
			return nil, err
		}
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
				vendor, _ := strconv.ParseUint(values["idVendor"], 0, 64)
				product, _ := strconv.ParseUint(values["idProduct"], 0, 64)
				if vendor == 0x05c6 && product == 0x9008 {
					label := values["USB Product Name"]
					if label == "" {
						label = values["IORegistryEntryName"]
					}
					match := sourceSerial.FindStringSubmatch(label)
					if len(match) == 2 {
						found = append(found, Device{Serial: "0x" + match[1], Product: label, Path: "ioreg:" + values["locationID"]})
					}
				}
			}
			for _, child := range node.Nodes {
				visit(child)
			}
		}
		visit(tree)
	default:
		return nil, fmt.Errorf("Device discovery is unsupported on %s.", runtime.GOOS)
	}
	return normalizeDevices(found)
}

func parseWindowsDevices(data []byte) ([]Device, error) {
	var items []Device
	data = []byte(strings.TrimPrefix(string(data), "\ufeff"))
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	for i := range items {
		match := sourceSerial.FindStringSubmatch(items[i].Product)
		if len(match) != 2 {
			return nil, fmt.Errorf("EDL device at %s has no selectable serial", items[i].Path)
		}
		items[i].Serial = "0x" + match[1]
	}
	return normalizeDevices(items)
}
