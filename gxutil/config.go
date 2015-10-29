package gxutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
)

const CfgFileName = ".gxrc"

type Config struct {
	Repos      []string `json:"repos,omitempty"`
	ExtraRepos []string `json:"extra_repos,omitempty"`
	User       User     `json:"user,omitempty"`
}

func (c *Config) getUsername() string {
	return c.User.Name
}

type User struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

func LoadConfig() (*Config, error) {
	// first check $HOME/.gxrc
	cfg, err := loadFile(path.Join(os.Getenv("HOME"), CfgFileName))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	local, err := loadFile(path.Join(cwd, CfgFileName))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if local == nil {
		return mapToCfg(cfg)
	}

	if cfg == nil {
		return mapToCfg(local)
	}

	merged := mergeConfigs(cfg, local)

	return mapToCfg(merged)
}

func mergeConfigs(base, extra map[string]interface{}) map[string]interface{} {
	if base == nil {
		return extra
	}

	for k, v := range extra {
		bk, ok := base[k]
		if !ok {
			base[k] = v
			continue
		}

		bmp, bmpok := bk.(map[string]interface{})
		emp, empok := v.(map[string]interface{})
		if !bmpok || !empok {
			// if the field is not an object, overwrite
			base[k] = v
			continue
		}

		base[k] = mergeConfigs(bmp, emp)
	}

	return base
}

func LoadConfigFrom(paths ...string) (*Config, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no path specified!")
	}

	base := paths[0]
	paths = paths[1:]

	cfg, err := loadFile(base)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	for _, np := range paths {
		next, err := loadFile(np)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
		}

		cfg = mergeConfigs(cfg, next)
	}

	return mapToCfg(cfg)
}

func loadFile(fname string) (map[string]interface{}, error) {
	var cfg map[string]interface{}
	fi, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(fi).Decode(&cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func mapToCfg(cfg map[string]interface{}) (*Config, error) {
	if cfg == nil {
		return new(Config), nil
	}

	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(cfg)
	if err != nil {
		return nil, err
	}

	out := new(Config)
	err = json.NewDecoder(buf).Decode(out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func WriteConfig(cfg *Config, file string) error {
	fi, err := os.Create(file)
	if err != nil {
		return err
	}
	defer fi.Close()
	return json.NewEncoder(fi).Encode(cfg)
}
