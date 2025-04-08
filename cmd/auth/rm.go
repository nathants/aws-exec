package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/nathants/aws-exec/exec"
	"github.com/nathants/libaws/lib"
)

func init() {
	// expose this cmd via the cli
	lib.Commands["auth-rm"] = authRm
	lib.Args["auth-rm"] = authRmArgs{}
}

type authRmArgs struct {
	Auth string `arg:"positional,required"`
}

func (authRmArgs) Description() string {
	return "\nrm auth\n"
}

func authRm() {
	var args authRmArgs
	arg.MustParse(&args)
	table := os.Getenv("PROJECT_NAME")
	id := args.Auth
	if !strings.HasPrefix(id, "auth.") {
		id = fmt.Sprintf("auth.%s", id)
	}
	key, err := attributevalue.MarshalMap(exec.RecordKey{
		ID: id,
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
	err = lib.Retry(context.Background(), func() error {
		_, err := lib.DynamoDBClient().DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
			TableName: aws.String(table),
			Key:       key,
		})
		if err != nil {
			if strings.Contains(err.Error(), "AccessDeniedException") {
				panic(err)
			}
		}
		return err
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
}
