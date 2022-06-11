package main

import (
	_ "github.com/nathants/aws-exec/cmd/wipe" // important that this is first, we re-use cli mechanism from libaws, and this clears the function list before we re-populate it

	"fmt"
	"os"
	"sort"
	"strings"

	_ "github.com/nathants/aws-exec/cmd/auth"
	_ "github.com/nathants/aws-exec/cmd/exec"
	_ "github.com/nathants/aws-exec/cmd/listdir"
	_ "github.com/nathants/aws-exec/cmd/rpc"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/nathants/aws-exec/backend"
	"github.com/nathants/libaws/lib"
)

func usage() {
	var fns []string
	maxLen := 0
	for fn := range lib.Commands {
		fns = append(fns, fn)
		maxLen = lib.Max(maxLen, len(fn))
	}
	sort.Strings(fns)
	fmtStr := "%-" + fmt.Sprint(maxLen) + "s - %s\n"
	for _, fn := range fns {
		fmt.Printf(fmtStr, fn, strings.Split(strings.Trim(lib.Args[fn].Description(), "\n"), "\n")[0])
	}
}

// we use a single binary for both lambda and the cli
func main() {
	// start lambda
	if len(os.Args) == 1 {
		lambda.Start(backend.HandleRequest)
		return
	}
	// start cli
	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	fn, ok := lib.Commands[cmd]
	if !ok {
		usage()
		os.Exit(1)
	}
	var args []string
	for _, a := range os.Args[1:] {
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			for _, k := range a[1:] {
				args = append(args, fmt.Sprintf("-%s", string(k)))
			}
		} else {
			args = append(args, a)
		}
	}
	os.Args = args
	fn()
}
