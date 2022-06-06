package awsexec

import (
	"context"
	"fmt"
	"os"

	"github.com/alexflint/go-arg"
	awsexec "github.com/nathants/aws-exec/exec"
	"github.com/nathants/libaws/lib"
)

func init() {
	lib.Commands["exec"] = exec
	lib.Args["exec"] = execArgs{}
}

type execArgs struct {
	Argv []string `arg:"positional,required"`
}

func (execArgs) Description() string {
	return "\nexec\n"
}

func exec() {
	var args execArgs
	arg.MustParse(&args)
	_ = os.Getenv("PROJECT_NAME")
	domain := os.Getenv("PROJECT_DOMAIN")
	auth := os.Getenv("AUTH")
	url := fmt.Sprintf("https://%s", domain)
	ctx := context.Background()
	callback := func(logs string) {
		fmt.Print(logs)
	}
	exitCode, err := awsexec.Exec(ctx, url, auth, args.Argv, callback, nil)
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
	os.Exit(exitCode)
}
