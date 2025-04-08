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
	lib.Commands["auth-new"] = authNew
	lib.Args["auth-new"] = authNewArgs{}
}

type authNewArgs struct {
	Name string `arg:"positional,required"`
}

func (authNewArgs) Description() string {
	return "\nnew auth\n"
}

func authNew() {
	var args authNewArgs
	arg.MustParse(&args)
	table := os.Getenv("PROJECT_NAME")
	key := exec.RandKey()
	item, err := attributevalue.MarshalMap(exec.Record{
		RecordKey: exec.RecordKey{
			ID: fmt.Sprintf("auth.%s", exec.Blake2b32(key)),
		},
		RecordData: exec.RecordData{
			Value: args.Name,
		},
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}

	err = lib.Retry(context.Background(), func() error {
		_, err := lib.DynamoDBClient().PutItem(context.Background(), &dynamodb.PutItemInput{
			Item:      item,
			TableName: aws.String(table),
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

	fmt.Println(key)
}
