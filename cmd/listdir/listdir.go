package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/alexflint/go-arg"
	awsexec "github.com/nathants/aws-exec/exec"
	"github.com/nathants/libaws/lib"
)

func init() {
	// expose this cmd via the cli
	lib.Commands["listdir"] = listdir
	lib.Args["listdir"] = listdirArgs{}

	// expost the cmd via rpc
	awsexec.Rpc["listdir"] = func(ctx context.Context, println func(v ...interface{}), argsJson string) error {
		args := listdirArgs{}
		err := json.Unmarshal([]byte(argsJson), &args)
		if err != nil {
			return err
		}
		return Listdir(ctx, println, &args)
	}
}

type listdirArgs struct {
	Path string `arg:"positional,required" json:"path"`
}

func (listdirArgs) Description() string {
	return "\nan example implementation of a command exposed via cli and rpc\n"
}

func listdir() {
	var args listdirArgs
	arg.MustParse(&args)
	ctx := context.Background()
	println := func(v ...interface{}) {
		fmt.Println(v...)
	}
	err := Listdir(ctx, println, &args)
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
}

func Listdir(_ context.Context, println func(v ...interface{}), args *listdirArgs) error {
	err := filepath.Walk(args.Path, func(path string, info fs.FileInfo, err error) error {
		if err == nil && !info.IsDir() && !strings.HasPrefix(path, ".") && !strings.Contains(path, "/.") {
			println(path)
		}
		return nil
	})
	return err
}
