package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/alexflint/go-arg"
	awsexec "github.com/nathants/aws-exec/exec"
	"github.com/nathants/libaws/lib"
)

func init() {
	// expose this cmd via the cli
	lib.Commands["exec"] = exec
	lib.Args["exec"] = execArgs{}
}

type execArgs struct {
	Argv []string `arg:"positional,required"`
}

func (execArgs) Description() string {
	return `
invoke command via subprocess

usage: bash bin/cli.sh exec ./cli listdir .
`
}

func exec() {
	var args execArgs
	arg.MustParse(&args)
	exitCode, err := awsexec.Exec(context.Background(), &awsexec.Args{
		Url:  fmt.Sprintf("https://%s", os.Getenv("PROJECT_DOMAIN")),
		Auth: os.Getenv("AUTH"),
		Argv: args.Argv,
		LogDataCallback: func(logs string) {
			fmt.Print(logs)
		},
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
	os.Exit(exitCode)
}
