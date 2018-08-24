package gxutil

import (
	"encoding/json"
	"io/ioutil"
)

type LockDep struct {
	// full ipfs path /ipfs/<hash>/foo
	Ref string

	// Mapping of dvcs import paths to LockData
	Deps map[string]LockDep
}

type LockFile struct {
	Language string
	Deps     map[string]LockDep
}

func LoadLockFile(lck *LockFile, fname string) error {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, lck)
}
