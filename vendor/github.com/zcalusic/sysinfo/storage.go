// Copyright © 2016 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package sysinfo

import (
	"bufio"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
)

// StorageDevice information.
type StorageDevice struct {
	Name   string `json:"name,omitempty"`
	Driver string `json:"driver,omitempty"`
	Vendor string `json:"vendor,omitempty"`
	Model  string `json:"model,omitempty"`
	Serial string `json:"serial,omitempty"`
	Size   uint   `json:"size,omitempty"` // device size in GB
}

func getSerial(name, fullpath string) (serial string) {
	var f *os.File
	var err error

	// Modern location/format of the udev database.
	if dev := slurpFile(path.Join(fullpath, "dev")); dev != "" {
		if f, err = os.Open(path.Join("/run/udev/data", "b"+dev)); err == nil {
			goto scan
		}
	}

	// Legacy location/format of the udev database.
	if f, err = os.Open(path.Join("/dev/.udev/db", "block:"+name)); err == nil {
		goto scan
	}

	// No serial :(
	return

scan:
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if sl := strings.Split(s.Text(), "="); len(sl) == 2 {
			if sl[0] == "E:ID_SERIAL_SHORT" {
				serial = sl[1]
				break
			}
		}
	}

	return
}

func (si *SysInfo) getStorageInfo() {
	sysBlock := "/sys/block"
	devices, err := ioutil.ReadDir(sysBlock)
	if err != nil {
		return
	}

	si.Storage = make([]StorageDevice, 0)
	for _, link := range devices {
		fullpath := path.Join(sysBlock, link.Name())
		dev, err := os.Readlink(fullpath)
		if err != nil {
			continue
		}

		if strings.HasPrefix(dev, "../devices/virtual/") {
			continue
		}

		// We could filter all removable devices here, but some systems boot from USB flash disks, and then we
		// would filter them, too. So, let's filter only floppies and CD/DVD devices, and see how it pans out.
		if strings.HasPrefix(dev, "../devices/platform/floppy") || slurpFile(path.Join(fullpath, "device", "type")) == "5" {
			continue
		}

		device := StorageDevice{
			Name:   link.Name(),
			Model:  slurpFile(path.Join(fullpath, "device", "model")),
			Serial: getSerial(link.Name(), fullpath),
		}

		if driver, err := os.Readlink(path.Join(fullpath, "device", "driver")); err == nil {
			device.Driver = path.Base(driver)
		}

		if vendor := slurpFile(path.Join(fullpath, "device", "vendor")); !strings.HasPrefix(vendor, "0x") {
			device.Vendor = vendor
		}

		size, _ := strconv.ParseUint(slurpFile(path.Join(fullpath, "size")), 10, 64)
		device.Size = uint(size) / 1953125 // GiB

		si.Storage = append(si.Storage, device)
	}
}
