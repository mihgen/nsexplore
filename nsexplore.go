package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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

	r := regexp.MustCompile("^[0-9]+$")

	for _, f := range files {
		if f.IsDir() && r.MatchString(f.Name()) {
			ps = append(ps, f)
		}
	}
	return ps
}

func namespaces(ps []os.FileInfo) NsMap {
	netns := NsMap{Map: make(map[string]*NsData)}
	r := regexp.MustCompile("^net:\\[([0-9]+)\\]$")

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
	return netns
}

func print(netns NsMap) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 0, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "NS NUMBER \t PIDS")

	// Sort by namespace
	nsList := []string{}
	for k, _ := range netns.Map {
		nsList = append(nsList, k)
	}
	sort.Strings(nsList)

	for _, ns := range nsList {
		pidsJoined := strings.Join(netns.Map[ns].pids, ",")
		fmt.Fprintln(w, ns, "\t", pidsJoined)
	}
	w.Flush()
}

func joinNs(netns NsMap, target string) {
	pids := netns.Map[target].pids
	if len(pids) == 0 {
		log.Fatalf("Namespace %s not found.", target)
	}
	pid := pids[0]

	fd, err := syscall.Open(filepath.Join("/proc", pid, "ns/net"), syscall.O_RDONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
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

	netns := namespaces(ps)

	// If we run external command:
	joinPtr := flag.String("j", "", "Join namespace number")
	flag.Parse()

	if *joinPtr == "" {
		print(netns)
	} else {
		joinNs(netns, *joinPtr)
	}

}
