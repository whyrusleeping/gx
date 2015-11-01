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
			delete(cfg.Repos, name)
		} else {
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
			tabPrintSortedMap(cfg.GetRepos())
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

		tabPrintSortedMap(repo)
	},
}

func tabPrintSortedMap(m map[string]string) {
	var names []string
	for k, _ := range m {
		names = append(names, k)
	}

	sort.Strings(names)

	w := tabwriter.NewWriter(os.Stdout, 12, 4, 1, ' ', 0)
	for _, n := range names {
		fmt.Fprintf(w, "%s\t%s\n", n, m[n])
	}
	w.Flush()
}

var RepoQueryCommand = cli.Command{
	Name: "query",
}
