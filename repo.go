package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	cli "github.com/codegangsta/cli"
	gx "github.com/whyrusleeping/gx/gxutil"
)

func cfgPath(global bool) (string, error) {
	if global {
		home := os.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("$HOME not set, cannot find global .gxrc")
		}
		return filepath.Join(home, gx.CfgFileName), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, gx.CfgFileName), nil
}

var RepoCommand = cli.Command{
	Name:  "repo",
	Usage: "manage set of tracked repositories",
	Subcommands: []cli.Command{
		RepoAddCommand,
		RepoRmCommand,
		RepoListCommand,
		RepoQueryCommand,
		RepoUpdateCommand,
	},
}

var RepoAddCommand = cli.Command{
	Name:  "add",
	Usage: "add a naming repository",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "global",
			Usage: "add repository to global set",
		},
	},
	Action: func(c *cli.Context) {
		global := c.Bool("global")
		cfp, err := cfgPath(global)
		if err != nil {
			Fatal(err)
		}

		cfg, err := gx.LoadConfigFrom(cfp)
		if err != nil {
			Fatal(err)
		}

		if len(c.Args()) != 2 {
			Fatal("Must specify name and repo-path")
		}
		name := c.Args()[0]
		rpath := c.Args()[1]

		// make sure we can fetch it
		_, err = pm.FetchRepo(rpath)
		if err != nil {
			Fatal("finding repo: ", err)
		}

		if global {
			cfg.Repos[name] = rpath
		} else {
			cfg.ExtraRepos[name] = rpath
		}

		err = gx.WriteConfig(cfg, cfp)
		if err != nil {
			Fatal(err)
		}
	},
}

var RepoRmCommand = cli.Command{
	Name:  "rm",
	Usage: "remove a repo from tracking",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "global",
			Usage: "remove repository from global set",
		},
	},
	Action: func(c *cli.Context) {
		global := c.Bool("global")
		cfp, err := cfgPath(global)
		if err != nil {
			Fatal(err)
		}

		cfg, err := gx.LoadConfigFrom(cfp)
		if err != nil {
			Fatal(err)
		}

		if !c.Args().Present() {
			Fatal("specify repo to remove")
		}
		name := c.Args().First()

		if global {
			if _, ok := cfg.Repos[name]; !ok {
				Fatal("no repo named %s", name)
			}
			delete(cfg.Repos, name)
		} else {
			if _, ok := cfg.ExtraRepos[name]; !ok {
				Fatal("no repo named %s", name)
			}
			delete(cfg.ExtraRepos, name)
		}

		err = gx.WriteConfig(cfg, cfp)
		if err != nil {
			Fatal(err)
		}
	},
}

var RepoListCommand = cli.Command{
	Name:  "list",
	Usage: "list tracked repos or packages in a repo",
	Action: func(c *cli.Context) {
		cfg, err := gx.LoadConfig()
		if err != nil {
			Fatal(err)
		}

		if !c.Args().Present() {
			tabPrintSortedMap(nil, cfg.GetRepos())
			return
		}

		rname := c.Args().First()
		r, ok := cfg.GetRepos()[rname]
		if !ok {
			Fatal("no such repo: ", rname)
		}

		repo, err := pm.FetchRepo(r)
		if err != nil {
			Fatal(err)
		}

		tabPrintSortedMap(nil, repo)
	},
}

func tabPrintSortedMap(headers []string, m map[string]string) {
	var names []string
	for k, _ := range m {
		names = append(names, k)
	}

	sort.Strings(names)

	w := tabwriter.NewWriter(os.Stdout, 12, 4, 1, ' ', 0)
	if headers != nil {
		fmt.Fprintf(w, "%s\t%s\n", headers[0], headers[1])
	}

	for _, n := range names {
		fmt.Fprintf(w, "%s\t%s\n", n, m[n])
	}
	w.Flush()
}

var RepoQueryCommand = cli.Command{
	Name:  "query",
	Usage: "search for a package in repos",
	Action: func(c *cli.Context) {
		if !c.Args().Present() {
			Fatal("must specify search criteria")
		}

		cfg, err := gx.LoadConfig()
		if err != nil {
			Fatal(err)
		}

		searcharg := c.Args().First()

		out := make(map[string]string)
		for name, rpath := range cfg.GetRepos() {
			repo, err := pm.FetchRepo(rpath)
			if err != nil {
				Fatal(err)
			}

			if val, ok := repo[searcharg]; ok {
				out[name] = val
			}
		}

		if len(out) > 0 {
			tabPrintSortedMap([]string{"repo", "ref"}, out)
		} else {
			Fatal("not found")
		}
	},
}

var RepoUpdateCommand = cli.Command{
	Name:  "update",
	Usage: "update cached versions of repos",
	Action: func(c *cli.Context) {
		Fatal("not yet implemented")
	},
}
