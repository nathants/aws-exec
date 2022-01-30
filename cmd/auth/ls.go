package awsrce

import (
	"context"
	"fmt"
	"os"

	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/nathants/aws-rce/rce"
	"github.com/nathants/cli-aws/lib"
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
	_ = os.Getenv("PROJECT_DOMAIN")
	_ = os.Getenv("AUTH")

	var start map[string]*dynamodb.AttributeValue
	for {
		out, err := lib.DynamoDBClient().ScanWithContext(context.Background(), &dynamodb.ScanInput{
			TableName:         aws.String(table),
			ExclusiveStartKey: start,
		})
		if err != nil {
			panic(err)
		}
		for _, item := range out.Items {
			val := rce.Record{}
			err := dynamodbattribute.UnmarshalMap(item, &val)
			if err != nil {
				panic(err)
			}
			fmt.Println(val.ID, val.Value)
		}
		if out.LastEvaluatedKey == nil {
			break
		}
		start = out.LastEvaluatedKey
	}
}
