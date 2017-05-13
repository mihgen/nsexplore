package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
)

type NsData struct {
	pids []string
	file string
}

type NsMap struct {
	Map map[string]*NsData
}

func processes() []os.FileInfo {
	var ps []os.FileInfo

	proc, err := os.Open("/proc")
	if err != nil {
		log.Fatal(err)
	}

	files, err := proc.Readdir(-1)
	if err != nil {
		log.Fatal(err)
	}

	r := regexp.MustCompile(`^\d+$`)

	for _, f := range files {
		if f.IsDir() && r.MatchString(f.Name()) {
			ps = append(ps, f)
		}
	}
	return ps
}

func AddFromPids(ps []os.FileInfo, netns NsMap) {
	r := regexp.MustCompile(`^net:\[([0-9]+)\]$`)

	for _, f := range ps {
		// if thread is created with CLONE_NEWNET option, it will live in new network namespace
		pdir, err := os.Open(filepath.Join("/proc", f.Name(), "task"))
		if err != nil {
			// this process may not exist by this time, skipping
			continue
		}
		tasks, err := pdir.Readdirnames(-1)
		if err != nil {
			// skip this process if we can't get its tasks
			continue
		}

		for _, t := range tasks {
			var fpath string
			if t == f.Name() {
				// use shorter path for main thread reference
				fpath = filepath.Join("/proc", f.Name(), "ns/net")
			} else {
				fpath = filepath.Join("/proc", f.Name(), "task", t, "ns/net")
			}
			nsName, err := os.Readlink(fpath)
			if err != nil {
				// Permission denied if you are not root: you can't see /proc/<pid>/
				// or there is no such process exist by now
				continue
			}

			// we are not converting string to a number for conveniency. nsNum is a string type.
			nsNum := r.FindStringSubmatch(nsName)[1]
			if _, ok := netns.Map[nsNum]; !ok {
				netns.Map[nsNum] = &NsData{file: fpath}
			}

			netns.Map[nsNum].pids = append(netns.Map[nsNum].pids, t)
		}
	}
}

func AddFromMount(netns NsMap) {
	// 55 54 0:3 net:[4026532152] /run/netns/myns rw shared:35 - nsfs nsfs rw
	//                            ^^ parsed[4] ^^
	r := regexp.MustCompile(` - nsfs`)
	var stat syscall.Stat_t

	f, err := os.Open("/proc/self/mountinfo")
	defer f.Close()
	if err != nil {
		log.Fatal(err)
	}
	s := bufio.NewScanner(f)
	for s.Scan() {
		if !r.MatchString(s.Text()) {
			continue // this is not nsfs, we don't need it
		}
		parsed := strings.Fields(s.Text())
		if len(parsed) < 5 {
			log.Printf("Err: Could not parse mountinfo line: '%s'\n", s.Text())
			continue
		}
		fpath := parsed[4]

		if err := syscall.Stat(fpath, &stat); err != nil {
			log.Printf("Err: Could not stat ns file: '%s'\n", fpath)
			continue
		}
		nsNum := strconv.FormatUint(stat.Ino, 10)
		if _, ok := netns.Map[nsNum]; !ok {
			netns.Map[nsNum] = &NsData{file: fpath}
		} else {
			// let's use this file instead of /proc/<pid>/...
			netns.Map[nsNum].file = fpath
		}
	}
}

func PrintNs(netns NsMap, all bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 0, ' ', tabwriter.AlignRight)

	// Sort by namespace
	nsList := []string{}
	for k, _ := range netns.Map {
		nsList = append(nsList, k)
	}
	sort.Strings(nsList)

	if all {
		fmt.Fprintln(w, "NS NUMBER \t FILE \t PID")
	} else {
		fmt.Fprintln(w, "NS NUMBER \t FILE \t PID \t CMD")
	}
	for _, ns := range nsList {
		if all {
			pidsJoined := strings.Join(netns.Map[ns].pids, ",")
			fmt.Fprintln(w, ns, "\t", netns.Map[ns].file, "\t", pidsJoined)
			continue
		}

		var pid, cmd string
		if len(netns.Map[ns].pids) > 0 {
			pid = netns.Map[ns].pids[0]
			data, _ := ioutil.ReadFile(filepath.Join("/proc", pid, "comm"))
			cmd = strings.TrimSuffix(string(data), "\n")
		}
		fmt.Fprintln(w, ns, "\t", netns.Map[ns].file, "\t", pid, "\t", cmd)
	}
	w.Flush()
}

func JoinNs(netns NsMap, target string) {
	// need to ensure that we don't run in another Linux thread:
	// if we change namespace by setns, all the other threads will continue to be in the old one
	// If Go scheduler changes this goroutine to run in a different thread, we will be surprised.
	runtime.LockOSThread()

	ns, ok := netns.Map[target]
	if !ok {
		log.Fatalf("Namespace %s not found\n", target)
	}

	fd, err := syscall.Open(ns.file, syscall.O_RDONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.Close(fd)
	exitCode, _, errno := syscall.RawSyscall(308, uintptr(fd), 0, 0) // 308 is setns
	if exitCode != 0 {
		log.Fatal(errno)
	}

	cmd := exec.Command(flag.Args()[0], flag.Args()[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	// If we run external command:
	join := flag.String("j", "", "Join namespace number specified, followed by command to run")
	all := flag.Bool("a", false, "Show all associated PIDs")
	flag.Parse()

	ps := processes()

	netns := NsMap{Map: make(map[string]*NsData)}
	AddFromPids(ps, netns)
	AddFromMount(netns)

	if *join == "" {
		PrintNs(netns, *all)
	} else {
		if len(flag.Args()) == 0 {
			flag.Usage()
			os.Exit(2)
		}
		JoinNs(netns, *join)
	}
}
