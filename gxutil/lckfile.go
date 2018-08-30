package gxutil

import (
	"encoding/json"
	"fmt"
	"os"
)

const LockVersion = 1

type LockDep struct {
	// full ipfs path /ipfs/<hash>/foo
	Ref string `json:"ref"`

	// Mapping of dvcs import paths to LockData
	Deps map[string]LockDep `json:"deps"`
}

type LockFile struct {
	Language    string             `json:"language"`
	LockVersion int                `json:"lockVersion"`
	Deps        map[string]LockDep `json:"deps"`
}

func LoadLockFile(lck *LockFile, fname string) error {
	fi, err := os.Open(fname)
	if err != nil {
		return err
	}

	if err := json.NewDecoder(fi).Decode(lck); err != nil {
		return err
	}

	if lck.LockVersion != LockVersion {
		return fmt.Errorf("unsupported lockfile version: %d", lck.LockVersion)
	}

	return nil
}
