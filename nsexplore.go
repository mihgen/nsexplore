package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
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

func main() {
	ps := processes()
	// key is namespace number, value is list of process pids. All stored as string type.
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 0, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "NS NUMBER \t PIDS")

	for ns, pids := range netns {
		pidsJoined := strings.Join(pids, ",")
		fmt.Fprintln(w, ns, "\t", pidsJoined)
	}
	w.Flush()
}
