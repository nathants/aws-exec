package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/alexflint/go-arg"
	awsexec "github.com/nathants/aws-exec/exec"
	"github.com/nathants/libaws/lib"
)

func init() {
	// expose this cmd via the cli
	lib.Commands["rpc"] = rpc
	lib.Args["rpc"] = rpcArgs{}
}

type rpcArgs struct {
	RpcName     string `arg:"positional,required"`
	RpcArgsJson string `arg:"positional,required"`
}

func (rpcArgs) Description() string {
	return `
invoke command via rpc

usage: bash bin/cli.sh rpc listdir '{"path": "."}'
`
}

func rpc() {
	var args rpcArgs
	arg.MustParse(&args)
	rpcArgs := map[string]interface{}{}
	err := json.Unmarshal([]byte(args.RpcArgsJson), &rpcArgs)
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
	exitCode, err := awsexec.Exec(context.Background(), &awsexec.Args{
		Url:     fmt.Sprintf("https://%s", os.Getenv("PROJECT_DOMAIN")),
		Auth:    os.Getenv("AUTH"),
		RpcName: args.RpcName,
		RpcArgs: args.RpcArgsJson,
		LogDataCallback: func(logs string) {
			fmt.Print(logs)
		},
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
	os.Exit(exitCode)
}
