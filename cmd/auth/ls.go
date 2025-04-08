package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/nathants/aws-exec/exec"
	"github.com/nathants/libaws/lib"
)

func init() {
	// expose this cmd via the cli
	lib.Commands["auth-ls"] = authLs
	lib.Args["auth-ls"] = authLsArgs{}
}

type authLsArgs struct {
}

func (authLsArgs) Description() string {
	return "\nls auth\n"
}

func authLs() {
	var args authLsArgs
	arg.MustParse(&args)
	table := os.Getenv("PROJECT_NAME")
	var start map[string]types.AttributeValue
	for {
		var out *dynamodb.ScanOutput
		err := lib.Retry(context.Background(), func() error {
			var err error
			out, err = lib.DynamoDBClient().Scan(context.Background(), &dynamodb.ScanInput{
				TableName:         &table,
				ExclusiveStartKey: start,
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
		for _, item := range out.Items {
			val := exec.Record{}
			err := attributevalue.UnmarshalMap(item, &val)
			if err != nil {
				lib.Logger.Fatal("error: ", err)
			}
			if strings.HasPrefix(val.ID, "auth.") {
				fmt.Println(val.ID, val.Value)
			}
		}
		if out.LastEvaluatedKey == nil {
			break
		}
		start = out.LastEvaluatedKey
	}
}
