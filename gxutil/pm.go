package gxutil

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	sh "github.com/ipfs/go-ipfs-api"
	mh "github.com/multiformats/go-multihash"
	prog "github.com/whyrusleeping/progmeter"
	. "github.com/whyrusleeping/stump"
)

const GxVersion = "0.14.2"

const PkgFileName = "package.json"
const LckFileName = "gx-lock.json"

var installPathsCache map[string]string
var binarySuffix string

func init() {
	installPathsCache = make(map[string]string)

	if runtime.GOOS == "windows" {
		binarySuffix = ".exe"
	}
}

type PM struct {
	ipfssh *sh.Shell

	cfg *Config

	ProgMeter *prog.ProgMeter

	global bool

	// hash of the 'empty' ipfs dir to avoid extra calls to object new
	blankDir string
}

func NewPM(cfg *Config) (*PM, error) {
	sh := NewShell()
	sh.SetTimeout(time.Minute * 8)
	return &PM{
		ipfssh: sh,
		cfg:    cfg,
	}, nil
}

func GetPackageRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for cwd != "/" {
		_, err := os.Stat(filepath.Join(cwd, PkgFileName))
		switch {
		case err == nil:
			return cwd, nil
		case os.IsNotExist(err):
			cwd = filepath.Join(cwd, "..")
		default:
			return "", err
		}
	}

	return "", fmt.Errorf("no package found in this directory or any above")
}

func (pm *PM) Shell() *sh.Shell {
	if pm.ipfssh == nil {
		pm.ipfssh = NewShell()
		pm.ipfssh.SetTimeout(time.Minute * 8)
	}

	return pm.ipfssh
}

func (pm *PM) ShellOnline() bool {
	_, err := pm.Shell().ID()
	return err == nil
}

func (pm *PM) SetGlobal(g bool) {
	pm.global = g
}

func maybeRunPostInstall(pkg *Package, pkgdir string, global bool) error {
	dir := filepath.Join(pkgdir, pkg.Name)
	if !pkgRanHook(dir, "post-install") {
		before := time.Now()
		VLog("  - running post install for %s:", pkg.Name, pkgdir)
		args := []string{pkgdir}
		if global {
			args = append(args, "--global")
		}
		err := TryRunHook("post-install", pkg.Language, pkg.SubtoolRequired, args...)
		if err != nil {
			return err
		}
		VLog("  - post install finished in ", time.Since(before))
		err = writePkgHook(dir, "post-install")
		if err != nil {
			return fmt.Errorf("error writing hook log: %s", err)
		}
	}

	return nil
}

func (pm *PM) InstallPackage(hash, ipath string) (*Package, error) {
	// if its already local, skip it
	pkgdir := filepath.Join(ipath, "gx", "ipfs", hash)
	cpkg := new(Package)
	err := FindPackageInDir(cpkg, pkgdir)
	if err != nil {
		VLog("  - %s not found locally, fetching into %s", hash, pkgdir)
		deppkg, err := pm.GetPackageTo(hash, pkgdir)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch package: %s: %s", hash, err)
		}
		VLog("  - fetch complete!")
		cpkg = deppkg
	}

	VLog("  - now processing dep %s-%s", cpkg.Name, hash)
	err = pm.InstallDeps(cpkg, ipath)
	if err != nil {
		return nil, err
	}

	if err := maybeRunPostInstall(cpkg, pkgdir, pm.global); err != nil {
		return nil, err
	}

	return cpkg, nil
}

func isTempError(err error) bool {
	return strings.Contains(err.Error(), "too many open files")
}

type DepWork struct {
	CacheDir string
	LinkDir  string
	Dep      string
	Ref      string
}

// InstallLock recursively installs all dependencies for the given lockfile
func (pm *PM) InstallLock(lck Lock, cwd string) error {
	lockList := []Lock{lck}

	maxWorkers := 20
	workers := make(chan DepWork, maxWorkers)

	var wg sync.WaitGroup
	var lk sync.Mutex
	var firstError error

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			for work := range workers {
				pm.ProgMeter.AddEntry(work.Ref, work.Dep, "[fetch]   <ELAPSED>"+work.Ref)

				cacheloc := filepath.Join(work.CacheDir, work.Ref)
				linkloc := filepath.Join(work.LinkDir, work.Dep)

				if err := pm.CacheAndLinkPackage(work.Ref, cacheloc, linkloc); err != nil {
					pm.ProgMeter.Error(work.Ref, err.Error())

					lk.Lock()
					if firstError == nil {
						firstError = err
					}
					lk.Unlock()

					continue
				}

				pm.ProgMeter.Finish(work.Ref)
			}

			wg.Done()
		}()
	}

	for {
		if len(lockList) == 0 {
			break
		}

		curr := lockList[0]
		lockList = lockList[1:]

		newLocks, err := pm.installLock(curr, cwd, workers)
		if err != nil {
			return err
		}

		if firstError == nil {
			lockList = append(lockList, newLocks...)
		}
	}

	close(workers)
	wg.Wait()

	return firstError
}

func (pm *PM) installLock(lck Lock, cwd string, workers chan<- DepWork) ([]Lock, error) {
	// Install all the direct dependencies for this lock

	// Each lock contains a mapping of languages to their own dependencies
	returnList := []Lock{}

	for lang, langdeps := range lck.Deps {
		ipath, err := InstallPath(lang, cwd, false)
		if err != nil {
			return []Lock{}, err
		}

		pm.ProgMeter.AddTodos(len(langdeps))

		for dep, deplock := range langdeps {
			if deplock.Deps != nil {
				returnList = append(returnList, deplock)
			}

			workers <- DepWork{
				CacheDir: filepath.Join(cwd, ".gx", "cache"),
				LinkDir:  ipath,
				Dep:      dep,
				Ref:      deplock.Ref,
			}

		}
	}

	return returnList, nil
}

func (pm *PM) SetProgMeter(meter *prog.ProgMeter) {
	pm.ProgMeter = meter
}

func padRight(s string, w int) string {
	if len(s) < w {
		return s + strings.Repeat(" ", len(s)-w)
	}
	return s
}

func pkgRanHook(dir, hook string) bool {
	p := filepath.Join(dir, ".gx", hook)
	_, err := os.Stat(p)
	return err == nil
}

func writePkgHook(dir, hook string) error {
	gxdir := filepath.Join(dir, ".gx")
	err := os.MkdirAll(gxdir, 0755)
	if err != nil {
		return err
	}

	fipath := filepath.Join(gxdir, hook)
	fi, err := os.Create(fipath)
	if err != nil {
		return err
	}

	return fi.Close()
}

func (pm *PM) InitPkg(dir, name, lang string, setup func(*Package)) error {
	// check for existing packagefile
	p := filepath.Join(dir, PkgFileName)
	_, err := os.Stat(p)
	if err == nil {
		return errors.New("package file already exists in working dir")
	}

	username := pm.cfg.User.Name
	if username == "" {
		u, err := user.Current()
		if err != nil {
			return fmt.Errorf("error looking up current user: %s", err)
		}
		username = u.Username
	}

	pkg := &Package{
		PackageBase: PackageBase{
			Name:      name,
			Author:    username,
			Language:  lang,
			Version:   "0.0.0",
			GxVersion: GxVersion,
			ReleaseCmd: "MESSAGE=\"gx publish $GX_VERSION\n\nReleased as $GX_HASH\"" +
				"&& git commit -a -m \"$MESSAGE\"" +
				"&& git tag -as -m \"$MESSAGE\" \"v$GX_VERSION\"" +
				"&& echo 'Please remember to push tags with --tags'",
		},
	}

	if setup != nil {
		setup(pkg)
	}

	// check if the user has a tool installed for the selected language
	CheckForHelperTools(lang)

	err = SavePackageFile(pkg, p)
	if err != nil {
		return err
	}

	err = TryRunHook("post-init", lang, pkg.SubtoolRequired, dir)
	return err
}

func CheckForHelperTools(lang string) {
	p, err := getSubtoolPath(lang)
	if err == nil && p != "" {
		return
	}

	if p == "" || strings.Contains(err.Error(), "file not found") {
		Log("notice: no helper tool found for", lang)
		return
	}

	Error("checking for helper tool:", err)
}

// ImportPackage downloads the package specified by dephash into the package
// in the directory 'dir'
func (pm *PM) ImportPackage(dir, dephash string) (*Dependency, error) {
	pkgpath := filepath.Join(dir, "gx", "ipfs", dephash)
	// check if its already imported
	_, err := os.Stat(pkgpath)
	if err == nil {
		var pkg Package
		err := FindPackageInDir(&pkg, pkgpath)
		if err != nil {
			return nil, fmt.Errorf("dir for package already exists, but no package found:%v", err)
		}

		return &Dependency{
			Name:    pkg.Name,
			Hash:    dephash,
			Version: pkg.Version,
		}, nil
	}

	ndep, err := pm.GetPackageTo(dephash, pkgpath)
	if err != nil {
		return nil, err
	}

	err = maybeRunPostInstall(ndep, pkgpath, pm.global)
	if err != nil {
		return nil, err
	}

	for _, child := range ndep.Dependencies {
		_, err := pm.ImportPackage(dir, child.Hash)
		if err != nil {
			return nil, err
		}
	}

	err = TryRunHook("post-import", ndep.Language, ndep.SubtoolRequired, dephash)
	if err != nil {
		return nil, err
	}

	return &Dependency{
		Name:    ndep.Name,
		Hash:    dephash,
		Version: ndep.Version,
	}, nil
}

// ResolveDepName resolves a given package name to a hash
// using configured repos as a mapping.
func (pm *PM) ResolveDepName(name string) (string, error) {
	_, err := mh.FromB58String(name)
	if err == nil {
		return name, nil
	}

	if strings.HasPrefix(name, "github.com/") {
		return pm.resolveGithubDep(name)
	}

	return pm.resolveNameInRepos(name)
}

func githubRawPath(repo string) string {
	base := strings.Replace(repo, "github.com", "raw.githubusercontent.com", 1)
	return base + "/master"
}

func (pm *PM) resolveGithubDep(name string) (string, error) {
	resp, err := http.Get("https://" + githubRawPath(name) + "/.gx/lastpubver")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		out, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		parts := strings.Split(string(out), ": ")
		if len(parts) < 2 {
			return "", fmt.Errorf("unrecognized format on .gx/lastpubver")
		}
		VLog("  - resolved %q to %s, version %s", name, parts[1], parts[0])
		return strings.TrimSpace(parts[1]), nil
	case 404:
		return "", fmt.Errorf("no gx package found at %s", name)
	default:
		return "", fmt.Errorf("unrecognized http response from github: %d: %s", resp.StatusCode, resp.Status)
	}
}

func (pm *PM) resolveNameInRepos(name string) (string, error) {
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		rpath, ok := pm.cfg.GetRepos()[parts[0]]
		if !ok {
			return "", fmt.Errorf("unknown repo: '%s'", parts[0])
		}

		pkgs, err := pm.FetchRepo(rpath, true)
		if err != nil {
			return "", err
		}

		val, ok := pkgs[parts[1]]
		if !ok {
			return "", fmt.Errorf("package %s not found in repo %s", parts[1], parts[0])
		}

		return val, nil
	}

	out, err := pm.QueryRepos(name)
	if err != nil {
		return "", err
	}

	if len(out) == 0 {
		return "", fmt.Errorf("could not find package by name: %s", name)
	}

	if len(out) == 1 {
		for _, v := range out {
			return v, nil
		}
	}

	return "", fmt.Errorf("ambiguous ref, appears in multiple repos")
}

func (pm *PM) EnumerateDependencies(pkg *Package) (map[string]string, error) {
	deps := make(map[string]string)
	err := pm.enumerateDepsRec(pkg, deps)
	if err != nil {
		return nil, err
	}

	return deps, nil
}

func (pm *PM) enumerateDepsRec(pkg *Package, set map[string]string) error {
	for _, d := range pkg.Dependencies {
		if _, ok := set[d.Hash]; ok {
			continue
		}

		set[d.Hash] = d.Name

		var depkg Package
		err := LoadPackage(&depkg, pkg.Language, d.Hash)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("package %s (%s) not found", d.Name, d.Hash)
			}
			return err
		}

		err = pm.enumerateDepsRec(&depkg, set)
		if err != nil {
			return err
		}
	}
	return nil
}

type PkgStats struct {
	totalDepth   int
	AverageDepth float64

	TotalImports int
}

type DepStats struct {
	TotalCount  int
	TotalUnique int

	AverageDepth float64
	totalDepth   int

	Packages map[string]*PkgStats
}

func (ds *DepStats) Finalize() {
	ds.AverageDepth = float64(ds.totalDepth) / float64(ds.TotalCount)

	for _, pkg := range ds.Packages {
		pkg.AverageDepth = float64(pkg.totalDepth) / float64(pkg.TotalImports)
	}
}

func newDepStats() *DepStats {
	return &DepStats{
		Packages: make(map[string]*PkgStats),
	}
}

func GetDepStats(pkg *Package) (*DepStats, error) {
	ds := newDepStats()
	err := getDepStatsRec(pkg, ds, 1)
	if err != nil {
		return nil, err
	}

	ds.Finalize()

	return ds, nil
}

func getDepStatsRec(pkg *Package, stats *DepStats, depth int) error {
	return pkg.ForEachDep(func(dep *Dependency, dpkg *Package) error {
		stats.TotalCount++
		stats.totalDepth += depth

		ps, ok := stats.Packages[dep.Hash]
		if !ok {
			stats.TotalUnique++
			ps = new(PkgStats)
			stats.Packages[dep.Hash] = ps
		}

		ps.totalDepth += depth
		ps.TotalImports++

		return getDepStatsRec(dpkg, stats, depth+1)
	})
}

func LocalPackageByName(dir, name string, out interface{}) error {
	if IsHash(name) {
		return FindPackageInDir(out, filepath.Join(dir, name))
	}

	var local Package
	err := LoadPackageFile(&local, PkgFileName)
	if err != nil {
		return err
	}

	return resolveDepName(&local, out, dir, name, make(map[string]struct{}))
}

func LoadPackage(out interface{}, env, hash string) error {
	VLog("  - load package:", hash)
	ipath, err := InstallPath(env, "", true)
	if err != nil {
		return err
	}

	p := filepath.Join(ipath, "gx", "ipfs", hash)
	err = FindPackageInDir(out, p)
	if err == nil {
		return nil
	}

	ipath, err = InstallPath(env, "", false)
	if err != nil {
		return err
	}

	p = filepath.Join(ipath, "gx", "ipfs", hash)
	return FindPackageInDir(out, p)
}

var ErrUnrecognizedName = fmt.Errorf("unrecognized package name")

func resolveDepName(pkg *Package, out interface{}, dir, name string, checked map[string]struct{}) error {
	// first check if its a direct dependency of this package
	for _, d := range pkg.Dependencies {
		if d.Name == name {
			return LoadPackageFile(out, filepath.Join(dir, d.Hash, d.Name, PkgFileName))
		}
	}

	// recurse!
	var dpkg Package
	for _, d := range pkg.Dependencies {
		if _, ok := checked[d.Hash]; ok {
			continue
		}

		err := LoadPackageFile(&dpkg, filepath.Join(dir, d.Hash, d.Name, PkgFileName))
		if err != nil {
			return err
		}

		err = resolveDepName(&dpkg, out, dir, name, checked)
		switch err {
		case nil:
			return nil // success!
		case ErrUnrecognizedName:
			checked[d.Hash] = struct{}{}
		default:
			return err
		}
	}

	return ErrUnrecognizedName
}
func IsSubtoolInstalled(env string) (bool, error) {
	p, err := getSubtoolPath(env)
	if err != nil {
		return false, err
	}

	return p != "", nil
}

func getSubtoolPath(env string) (string, error) {
	if env == "" {
		return "", nil
	}

	binname := "gx-" + env + binarySuffix
	_, err := exec.LookPath(binname)
	if err != nil {
		if eErr, ok := err.(*exec.Error); ok {
			if eErr.Err != exec.ErrNotFound {
				return "", err
			}
		} else {
			return "", err
		}

		if dir, file := filepath.Split(os.Args[0]); dir != "" {
			fileNoExe := strings.TrimSuffix(file, binarySuffix)
			nearBin := filepath.Join(dir, fileNoExe+"-"+env+binarySuffix)

			if _, err := os.Stat(nearBin); err != nil {
				VLog("subtool_exec: No gx helper tool found for", env)
				return "", nil
			}
			binname = nearBin
		} else {
			return "", nil
		}
	}

	return binname, nil
}

func TryRunHook(hook, env string, req bool, args ...string) error {
	binname, err := getSubtoolPath(env)
	if err != nil {
		return err
	}

	if binname == "" {
		if req {
			return fmt.Errorf("no binary named gx-%s was found.", env)
		}
		return nil
	}

	args = append([]string{"hook", hook}, args...)
	cmd := exec.Command(binname, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("%s hook failed: %s", hook, err)
	}

	return nil
}

const defaultLocalPath = "vendor"

func InstallPath(env, relpath string, global bool) (string, error) {
	if env == "" {
		VLog("no env, returning empty install path")
		return defaultLocalPath, nil
	}

	if cached, ok := checkInstallPathCache(env, global); ok {
		return cached, nil
	}

	binname, err := getSubtoolPath(env)
	if err != nil {
		return "", err
	}
	if binname == "" {
		return defaultLocalPath, nil
	}

	args := []string{"hook", "install-path"}
	if global {
		args = append(args, "--global")
	}
	cmd := exec.Command(binname, args...)

	cmd.Stderr = os.Stderr
	cmd.Dir = relpath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("install-path hook failed: %s", err)
	}

	val := strings.Trim(string(out), " \t\n")
	setInstallPathCache(env, global, val)
	return val, nil
}

func checkInstallPathCache(env string, global bool) (string, bool) {
	if global {
		env += " --global"
	}
	v, ok := installPathsCache[env]
	return v, ok
}

func setInstallPathCache(env string, global bool, val string) {
	if global {
		env += " --global"
	}

	installPathsCache[env] = val
}

func IsHash(s string) bool {
	return strings.HasPrefix(s, "Qm") && len(s) == 46
}

// InstallDeps fetches all dependencies for the given package (in parallel)
// and then calls the `post-install` hook on each one. Those two processes
// are not combined because the rewrite process in the `post-install` hook
// needs all of the dependencies (directs and transitives) of a package to
// compute the rewrite map which enforces a particular order in the traversal
// of the dependency graph and that constraint invalidates the parallel fetch
// in `fetchDependencies` (where the dependencies are processes in the random
// order they are fetched, without consideration for their order in the
// dependency graph).
func (pm *PM) InstallDeps(pkg *Package, location string) error {
	err := pm.fetchDependencies(pkg, location)
	if err != nil {
		return err
	}

	return pm.dependenciesPostInstall(pkg, location)
}

// Queue of dependency packages to install. Supported by a slice,
// it's not very performant but the main bottleneck here is the
// fetch operation (`GetPackageTo`).
type DependencyQueue struct {
	// Slice that supports the queue.
	queue []*Dependency
	// Map that keeps track of the dependencies already added to
	// the queue (at some point, may not be in the queue at the
	// moment), accessed by the `Dependency.Hash` (set to `true`
	// if the dependency has already been added).
	added map[string]bool
}

// NewDependencyQueue creates a new `DependencyQueue` with
// the specified `initialCapacity` for the slice.
func NewDependencyQueue(initialCapacity int) *DependencyQueue {
	return &DependencyQueue{
		queue: make([]*Dependency, 0, initialCapacity),
		added: make(map[string]bool),
	}
}

// AddPackageDependencies adds all of the dependencies of `pkg`
// to the queue that had not been already added. Return the
// actual number of dependencies added to the queue.
func (dq *DependencyQueue) AddPackageDependencies(pkg *Package) int {
	addedDepCount := 0
	for _, dep := range pkg.Dependencies {
		if dq.added[dep.Hash] == false {
			dq.queue = append(dq.queue, dep)
			addedDepCount++
			dq.added[dep.Hash] = true
		}
	}
	return addedDepCount
}

// Len returns the number of dependencies currently stored in the queue.
func (dq *DependencyQueue) Len() int {
	return len(dq.queue)
}

// Pop the first dependency in the queue and return it
// (or `nil` if the queue is empty).
func (dq *DependencyQueue) Pop() *Dependency {
	if dq.Len() == 0 {
		return nil
	}

	dep := dq.queue[0]
	dq.queue = dq.queue[1:]
	return dep
}

// Fetch all of the dependencies of this package (direct and transitive
// ones). Use (if possible) `maxGoroutines` goroutines working in parallel
// (coordinated by this function). Each new dependency fetched is another
// package with more (potentially new) dependencies that may also be fetched.
//
// TODO: Depending on the perspective sometimes we use the *package*
// term and others *dependency* (of another package), that should
// be unified and clarified as much as possible (not just in this function).
func (pm *PM) fetchDependencies(pkg *Package, location string) error {

	// Maximum number of goroutines allowed to run in parallel fetching
	// packages.
	const maxGoroutines = 20
	// TODO: Consider making this value a parameter of the function
	// (or an attribute of the `PM` structure).

	// Central queue of dependencies that need to be fetched. Handled only
	// by this function. Created with an initial a capacity on the rough
	// estimate of twice the maximum goroutines running.
	depQueue := NewDependencyQueue(maxGoroutines * 2)

	// List of channels for each spawned goroutine to store either the
	// fetched package or an error. To ensure they are non blocking the
	// maximum number of goroutines it's assigned for their capacity
	// (worst case scenario).
	fetchedPackages := make(chan *Package, maxGoroutines)
	fetchErrs := make(chan error, maxGoroutines)

	// Save the first fetch error as the function return value,
	// if more errors come after that they will be logged but not
	// returned.
	var firstFetchErr error

	// To start the process add the dependencies of the root package.
	addedDepCount := depQueue.AddPackageDependencies(pkg)
	pm.ProgMeter.AddTodos(addedDepCount)

	// Counter to keep track of spawned goroutines, it's not locked as it's
	// only handled by this function. Decremented any time a message is received
	// on the above channels, which indicates that the goroutine has finished.
	activeGoroutines := 0

	// Main loop of the function.
	for {
		// If there are no more dependencies to install and there aren't any
		// goroutines running (which could potentially add new dependencies
		// to the queue) we're finished.
		if depQueue.Len() == 0 && activeGoroutines == 0 {
			return nil
		}

		// Keep spawning goroutines until the allowed maximum or until
		// there are no new dependencies to fetch at the moment.
		for activeGoroutines < maxGoroutines && depQueue.Len() > 0 {
			// TODO: Use the worker pattern here (https://gobyexample.com/worker-pools),
			// instead of counting active goroutines we should be counting active jobs.

			// Goroutine that only calls `GetPackageTo` to fetch the dependency,
			// it either returns it or returns an error.
			go func(dep *Dependency) {

				pkgDir := filepath.Join(location, "gx", "ipfs", dep.Hash)
				// TODO: Encapsulate in a function. Used in too many places
				// and is part of the standard.

				pm.ProgMeter.AddEntry(dep.Hash, dep.Name, "[fetch]   <ELAPSED>"+dep.Hash)
				pkg, err := pm.GetPackageTo(dep.Hash, pkgDir)

				// Either with error or with the package the goroutine ends here.
				if err != nil {
					fetchErrs <- fmt.Errorf("failed to fetch package: %s: %s", dep.Hash, err)
					pm.ProgMeter.Error(dep.Hash, err.Error())
				} else {
					fetchedPackages <- pkg
					pm.ProgMeter.Finish(dep.Hash)
				}
			}(depQueue.Pop())

			activeGoroutines++
		}

		// Once all the possible goroutines have been spawned wait
		// for anyone to finish and analyze (restart loop) if more
		// goroutines can be called.
		select {
		case fetchedPkg := <-fetchedPackages:
			VLog("fetched dep: %s", fetchedPkg.Name)
			addedDepCount := depQueue.AddPackageDependencies(fetchedPkg)
			pm.ProgMeter.AddTodos(addedDepCount)
		case firstFetchErr = <-fetchErrs:
			Error("parallel fetch: %s", firstFetchErr)
		}
		activeGoroutines--

		if firstFetchErr != nil {
			break
			// An error happened inside a fetch goroutine, stop the main `for`,
			// do not order more fetches.
			// TODO: If `GetPackageTo` or the `shell.Get()` function had a `Context`
			// it would be useful to issue a `cancel()` here before returning.
		}
	}

	// We broke out of the `for`, at least one error was detected in the
	// fetch operations, wait for the rest of the goroutines to finish.
	for activeGoroutines > 0 {
		select {
		case err := <-fetchErrs:
			Error("parallel fetch: %s", err)
		case _ = <-fetchedPackages:
		}
		activeGoroutines--
	}

	return firstFetchErr
}

// Call the `post-install` hook on each of the dependencies of this package
// (direct or transitive).
//
// TODO: This function could also use the same parallel goroutine processing
// structure of `fetchDependencies` but right now the `post-install` hook
// of the only sub-tool (`gx-go rewrite`) already does a parallel processing
// of its own, so there's little to gain here.
func (pm *PM) dependenciesPostInstall(pkg *Package, location string) error {
	depQueue := NewDependencyQueue(len(pkg.Dependencies) * 2)

	addedDepCount := depQueue.AddPackageDependencies(pkg)
	pm.ProgMeter.AddTodos(addedDepCount)

	for {
		dep := depQueue.Pop()
		if dep == nil {
			return nil
			// No more dependencies to process
		}

		hash := dep.Hash
		pkgdir := filepath.Join(location, "gx", "ipfs", hash)
		// TODO: Encapsulate in a function.

		pkg := new(Package)
		err := FindPackageInDir(pkg, pkgdir)
		if err != nil {
			return err
		}

		pm.ProgMeter.AddEntry(dep.Hash, dep.Name, "[install] <ELAPSED>"+dep.Hash)
		pm.ProgMeter.Working(dep.Hash, "work")
		if err := maybeRunPostInstall(pkg, pkgdir, pm.global); err != nil {
			pm.ProgMeter.Error(dep.Hash, err.Error())
			return err
		}
		pm.ProgMeter.Finish(dep.Hash)

		// Add the dependencies of this package to the queue.
		addedDepCount := depQueue.AddPackageDependencies(pkg)
		pm.ProgMeter.AddTodos(addedDepCount)
	}
}
