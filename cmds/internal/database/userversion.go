package database

import (
	"fmt"
)

const (
	DatabaseGooglePlay uint8 = 1
	DatabaseAppStore   uint8 = 2
)

// https://www.sqlite.org/pragma.html#pragma_application_id
func DecodeUserVersion(userVersion int32) (store uint8, version uint8, err error) {
	if userVersion < 0 {
		err = fmt.Errorf("invalid userVersion: must be non-negative")
		return
	}

	userVersionUnsigned := uint16(userVersion)
	store, version = uint8(userVersionUnsigned>>8), uint8(userVersionUnsigned&0xFF)
	return
}

func EncodeUserVersion(store uint8, version uint8) int32 {
	return int32((uint16(store) << 8) | (uint16(version) & 0xFF))
}
