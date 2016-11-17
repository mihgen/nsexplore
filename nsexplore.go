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
	"strings"
	"syscall"
	"text/tabwriter"
)

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

func namespaces(ps []os.FileInfo) map[string][]string {
	netns := make(map[string][]string)
	r := regexp.MustCompile("^net:\\[([0-9]+)\\]$")

	for _, f := range ps {
		nsName, err := os.Readlink("/proc/" + f.Name() + "/ns/net")
		if err != nil {
			// Permission denied if you are not root: you can't see /proc/<pid>/
			continue
		}

		// we are not converting string to a number for conveniency. nsNum is a string type.
		nsNum := r.FindStringSubmatch(nsName)[1]

		netns[nsNum] = append(netns[nsNum], f.Name())
	}
	return netns
}

func main() {
	runtime.LockOSThread()
	ps := processes()
	// key is namespace number, value is list of process pids. All stored as string type.
	netns := namespaces(ps)

	// If we run external command:
	joinPtr := flag.String("j", "", "Join namespace number")
	flag.Parse()

	if *joinPtr == "" {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 0, ' ', tabwriter.AlignRight)
		fmt.Fprintln(w, "NS NUMBER \t PIDS")

		//TODO: sort by namespace number, otherwise it's random from hashmap
		for ns, pids := range netns {
			pidsJoined := strings.Join(pids, ",")
			fmt.Fprintln(w, ns, "\t", pidsJoined)
		}
		w.Flush()
		os.Exit(0)
	}

	//TODO: is there a way to get fd without going to proc?
	//TODO: check that PID exists
	pid := netns[*joinPtr][0]

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
