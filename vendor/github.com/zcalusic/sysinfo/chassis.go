// Copyright © 2016 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package sysinfo

import "strconv"

// Chassis information.
type Chassis struct {
	Type     uint   `json:"type,omitempty"`
	Vendor   string `json:"vendor,omitempty"`
	Version  string `json:"version,omitempty"`
	Serial   string `json:"serial,omitempty"`
	AssetTag string `json:"assettag,omitempty"`
}

func (si *SysInfo) getChassisInfo() {
	if chtype, err := strconv.ParseUint(slurpFile("/sys/class/dmi/id/chassis_type"), 10, 64); err == nil {
		si.Chassis.Type = uint(chtype)
	}
	si.Chassis.Vendor = slurpFile("/sys/class/dmi/id/chassis_vendor")
	si.Chassis.Version = slurpFile("/sys/class/dmi/id/chassis_version")
	si.Chassis.Serial = slurpFile("/sys/class/dmi/id/chassis_serial")
	si.Chassis.AssetTag = slurpFile("/sys/class/dmi/id/chassis_asset_tag")
}
