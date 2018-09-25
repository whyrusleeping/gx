package gxutil

import (
	"encoding/json"
	"fmt"
	"os"
)

const LockVersion = 1

type LockFile struct {
	Lock
	LockVersion int `json:"lockVersion"`
}

type Lock struct {
	Language string `json:"language,omitempty"`

	Ref  string                     `json:"ref,omitempty"`
	Deps map[string]map[string]Lock `json:"deps,omitempty"`
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
