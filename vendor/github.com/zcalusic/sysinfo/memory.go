// Copyright © 2016 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package sysinfo

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// Memory information.
type Memory struct {
	Type  string `json:"type,omitempty"`
	Speed uint   `json:"speed,omitempty"` // RAM data rate in MT/s
	Size  uint   `json:"size,omitempty"`  // RAM size in MB
}

const epsSize = 0x1f

// ErrNotExist indicates that SMBIOS entry point could not be found.
var ErrNotExist = errors.New("SMBIOS entry point not found")

func word(data []byte, index int) uint16 {
	return binary.LittleEndian.Uint16(data[index : index+2])
}

func dword(data []byte, index int) uint32 {
	return binary.LittleEndian.Uint32(data[index : index+4])
}

func qword(data []byte, index int) uint64 {
	return binary.LittleEndian.Uint64(data[index : index+8])
}

func epsChecksum(sl []byte) (sum byte) {
	for _, v := range sl {
		sum += v
	}

	return
}

func epsValid(eps []byte) bool {
	if epsChecksum(eps) == 0 && bytes.Equal(eps[0x10:0x15], []byte("_DMI_")) && epsChecksum(eps[0x10:]) == 0 {
		return true
	}

	return false
}

func getStructureTableAddressEFI(f *os.File) (address int64, length int, err error) {
	systab, err := os.Open("/sys/firmware/efi/systab")
	if err != nil {
		return 0, 0, err
	}
	defer systab.Close()

	s := bufio.NewScanner(systab)
	for s.Scan() {
		sl := strings.Split(s.Text(), "=")
		if len(sl) != 2 || sl[0] != "SMBIOS" {
			continue
		}

		addr, err := strconv.ParseInt(sl[1], 0, 64)
		if err != nil {
			return 0, 0, err
		}

		eps, err := syscall.Mmap(int(f.Fd()), addr, epsSize, syscall.PROT_READ, syscall.MAP_SHARED)
		if err != nil {
			return 0, 0, err
		}
		defer syscall.Munmap(eps)

		if !epsValid(eps) {
			break
		}

		return int64(dword(eps, 0x18)), int(word(eps, 0x16)), nil
	}
	if err := s.Err(); err != nil {
		return 0, 0, err
	}

	return 0, 0, ErrNotExist
}

func getStructureTableAddress(f *os.File) (address int64, length int, err error) {
	// SMBIOS Reference Specification Version 3.0.0, page 21
	mem, err := syscall.Mmap(int(f.Fd()), 0xf0000, 0x10000, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return 0, 0, err
	}
	defer syscall.Munmap(mem)

	for i := range mem {
		if i > len(mem)-epsSize {
			break
		}

		// Search for the anchor string on paragraph (16 byte) boundaries.
		if i%16 != 0 || !bytes.Equal(mem[i:i+4], []byte("_SM_")) {
			continue
		}

		eps := mem[i : i+epsSize]
		if !epsValid(eps) {
			continue
		}

		return int64(dword(eps, 0x18)), int(word(eps, 0x16)), nil
	}

	return 0, 0, ErrNotExist
}

func getStructureTable() ([]byte, error) {
	f, err := os.Open("/dev/mem")
	if err != nil {
		dmi, err := ioutil.ReadFile("/sys/firmware/dmi/tables/DMI")
		if err != nil {
			return nil, err
		}
		return dmi, nil
	}
	defer f.Close()

	address, length, err := getStructureTableAddressEFI(f)
	if err != nil {
		if address, length, err = getStructureTableAddress(f); err != nil {
			return nil, err
		}
	}

	// Mandatory page aligning for mmap() system call, lest we get EINVAL
	align := address & (int64(os.Getpagesize()) - 1)
	mem, err := syscall.Mmap(int(f.Fd()), address-align, length+int(align), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	return mem[align:], nil
}

func (si *SysInfo) getMemoryInfo() {
	mem, err := getStructureTable()
	if err != nil {
		if targetKB := slurpFile("/sys/devices/system/xen_memory/xen_memory0/target_kb"); targetKB != "" {
			si.Memory.Type = "DRAM"
			size, _ := strconv.ParseUint(targetKB, 10, 64)
			si.Memory.Size = uint(size) / 1024
		}
		return
	}
	defer syscall.Munmap(mem)

	si.Memory.Size = 0
	var memSizeAlt uint
loop:
	for p := 0; p < len(mem)-1; {
		recType := mem[p]
		recLen := mem[p+1]

		switch recType {
		case 4:
			if si.CPU.Speed == 0 {
				si.CPU.Speed = uint(word(mem, p+0x16))
			}
		case 17:
			size := uint(word(mem, p+0x0c))
			if size == 0 || size == 0xffff || size&0x8000 == 0x8000 {
				break
			}
			if size == 0x7fff {
				if recLen >= 0x20 {
					size = uint(dword(mem, p+0x1c))
				} else {
					break
				}
			}

			si.Memory.Size += size

			if si.Memory.Type == "" {
				// SMBIOS Reference Specification Version 3.0.0, page 92
				memTypes := [...]string{
					"Other", "Unknown", "DRAM", "EDRAM", "VRAM", "SRAM", "RAM", "ROM", "FLASH",
					"EEPROM", "FEPROM", "EPROM", "CDRAM", "3DRAM", "SDRAM", "SGRAM", "RDRAM",
					"DDR", "DDR2", "DDR2 FB-DIMM", "Reserved", "Reserved", "Reserved", "DDR3",
					"FBD2", "DDR4", "LPDDR", "LPDDR2", "LPDDR3", "LPDDR4",
				}

				if index := int(mem[p+0x12]); index >= 1 && index <= len(memTypes) {
					si.Memory.Type = memTypes[index-1]
				}
			}

			if si.Memory.Speed == 0 && recLen >= 0x17 {
				if speed := uint(word(mem, p+0x15)); speed != 0 {
					si.Memory.Speed = speed
				}
			}
		case 19:
			start := uint(dword(mem, p+0x04))
			end := uint(dword(mem, p+0x08))
			if start == 0xffffffff && end == 0xffffffff {
				if recLen >= 0x1f {
					start64 := qword(mem, p+0x0f)
					end64 := qword(mem, p+0x17)
					memSizeAlt += uint((end64 - start64 + 1) / 1048576)
				}
			} else {
				memSizeAlt += (end - start + 1) / 1024
			}
		case 127:
			break loop
		}

		for p += int(recLen); p < len(mem)-1; {
			if bytes.Equal(mem[p:p+2], []byte{0, 0}) {
				p += 2
				break
			}
			p++
		}
	}

	// Sometimes DMI type 17 has no information, so we fall back to DMI type 19, to at least get the RAM size.
	if si.Memory.Size == 0 && memSizeAlt > 0 {
		si.Memory.Type = "DRAM"
		si.Memory.Size = memSizeAlt
	}
}
