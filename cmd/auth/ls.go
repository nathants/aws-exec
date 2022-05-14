package awsrce

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/nathants/aws-rce/rce"
	"github.com/nathants/libaws/lib"
)

func init() {
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
	var start map[string]*dynamodb.AttributeValue
	for {
		var out *dynamodb.ScanOutput
		err := lib.Retry(context.Background(), func() error {
			var err error
			out, err = lib.DynamoDBClient().ScanWithContext(context.Background(), &dynamodb.ScanInput{
				TableName:         aws.String(table),
				ExclusiveStartKey: start,
			})
			return err
		})
		if err != nil {
			lib.Logger.Fatal("error: ", err)
		}
		for _, item := range out.Items {
			val := rce.Record{}
			err := dynamodbattribute.UnmarshalMap(item, &val)
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
