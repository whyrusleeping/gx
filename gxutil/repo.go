package gxutil

func (pm *PM) FetchRepo(rpath string) (map[string]string, error) {
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
