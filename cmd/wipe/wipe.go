package awsrce

import "github.com/nathants/cli-aws/lib"

func init() {
	lib.Args = make(map[string]lib.ArgsStruct)
	lib.Commands = make(map[string]func())
}
