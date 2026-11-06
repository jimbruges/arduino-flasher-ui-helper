package provision

import (
	"testing"
)

func TestWindowsDescriptorDiscovery(t *testing.T) {
	devices, err := parseWindowsDevices([]byte(`[{"Product":"QUSB_BULK_CID:0404_SN:12345678","Path":"USB-device"}]`))
	if err != nil || len(devices) != 1 || devices[0].Serial != "0x12345678" {
		t.Fatalf("%+v %v", devices, err)
	}
	if _, err := parseWindowsDevices([]byte(`[{"Product":"Qualcomm COM3","Path":"device"}]`)); err == nil {
		t.Fatal("Accepted non-serial friendly name")
	}
}
