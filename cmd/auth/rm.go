package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
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
	key, err := dynamodbattribute.MarshalMap(exec.RecordKey{
		ID: id,
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
	err = lib.Retry(context.Background(), func() error {
		_, err := lib.DynamoDBClient().DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(table),
			Key:       key,
		})
		aerr, ok := err.(awserr.Error)
		if ok && aerr.Code() == "AccessDeniedException" {
			panic(err)
		}
		return err
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
}
