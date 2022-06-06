package awsexec

import "github.com/nathants/libaws/lib"

func init() {
	lib.Args = make(map[string]lib.ArgsStruct)
	lib.Commands = make(map[string]func())
}
