package database

import (
	"math"
	"testing"
)

func TestEncodeDecodeUserVersion(t *testing.T) {
	for store := 0; store <= math.MaxUint8; store++ {
		store := uint8(store)
		for version := 0; version < math.MaxUint8; version++ {
			version := uint8(version)
			userVersion := EncodeUserVersion(store, version)

			outStore, outVersion, err := DecodeUserVersion(userVersion)
			if err != nil {
				t.Error(err)
				continue
			}

			if store != outStore {
				t.Error("store != outStore")
			}

			if version != outVersion {
				t.Error("version != outVersion")
			}
		}
	}
}
