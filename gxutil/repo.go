package gxutil

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	. "github.com/whyrusleeping/stump"
)

func (pm *PM) FetchRepo(rpath string) (map[string]string, error) {
	if strings.HasPrefix(rpath, "/ipns/") {
		p, err := pm.cacheGet(rpath)
		if err != nil {
			return nil, err
		}

		rpath = p
	}
	links, err := pm.shell.List(rpath)
	if err != nil {
		return nil, err
	}

	out := make(map[string]string)
	for _, l := range links {
		out[l.Name] = l.Hash
	}

	return out, nil
}

var ErrNotFound = errors.New("cache miss")

// TODO: once on ipfs 0.4.0, use the files api
func (pm *PM) cacheGet(name string) (string, error) {
	home := os.Getenv("HOME")
	p := filepath.Join(home, ".gxcache")

	fi, err := os.Open(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
	} else {
		defer fi.Close()

		cache := make(map[string]string)
		err = json.NewDecoder(fi).Decode(&cache)
		if err != nil {
			return "", err
		}

		v, ok := cache[name]
		if ok {
			return v, nil
		}
	}

	out, err := pm.shell.ResolvePath(name)
	if err != nil {
		Error("error from resolve path", name)
		return "", err
	}

	err = pm.cacheResolution(name, out)
	if err != nil {
		return "", err
	}

	return out, nil
}

// TODO: think about moving gx global files into a .config/local type thing
func (pm *PM) cacheResolution(name, resolved string) error {
	home := os.Getenv("HOME")
	p := filepath.Join(home, ".gxcache")

	_, err := os.Stat(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	cache := make(map[string]string)
	if err == nil { // if the file already exists
		fi, err := os.Open(p)
		if err != nil {
			return err
		}

		err = json.NewDecoder(fi).Decode(&cache)
		if err != nil {
			return err
		}

		fi.Close()
	}

	cache[name] = resolved

	fi, err := os.Create(p)
	if err != nil {
		return err
	}

	err = json.NewEncoder(fi).Encode(cache)
	if err != nil {
		return err
	}

	return fi.Close()
}
