package main

import (
	"bufio"
	"flag"
	"fmt"
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
		fpath := filepath.Join("/proc", f.Name(), "ns/net")
		nsName, err := os.Readlink(fpath)
		if err != nil {
			// Permission denied if you are not root: you can't see /proc/<pid>/
			continue
		}

		// we are not converting string to a number for conveniency. nsNum is a string type.
		nsNum := r.FindStringSubmatch(nsName)[1]
		if _, ok := netns.Map[nsNum]; !ok {
			netns.Map[nsNum] = &NsData{file: fpath}
		}

		netns.Map[nsNum].pids = append(netns.Map[nsNum].pids, f.Name())
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
		} // otherwise we already have it under one of pids
	}
}

func PrintNs(netns NsMap) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 0, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "NS NUMBER \t FILE \t PIDS")

	// Sort by namespace
	nsList := []string{}
	for k, _ := range netns.Map {
		nsList = append(nsList, k)
	}
	sort.Strings(nsList)

	for _, ns := range nsList {
		pidsJoined := strings.Join(netns.Map[ns].pids, ",")
		fmt.Fprintln(w, ns, "\t", netns.Map[ns].file, "\t", pidsJoined)
	}
	w.Flush()
}

func JoinNs(netns NsMap, target string) {
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
	runtime.LockOSThread()
	ps := processes()

	netns := NsMap{Map: make(map[string]*NsData)}
	AddFromPids(ps, netns)
	AddFromMount(netns)

	// If we run external command:
	joinPtr := flag.String("j", "", "Join namespace number")
	flag.Parse()

	if *joinPtr == "" {
		PrintNs(netns)
	} else {
		JoinNs(netns, *joinPtr)
	}
}
