package awsrce

import (
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/nathants/aws-rce/rce"
	"github.com/nathants/cli-aws/lib"
)

func init() {
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
	_ = os.Getenv("PROJECT_DOMAIN")
	_ = os.Getenv("AUTH")

	id := args.Auth
	if !strings.HasPrefix(id, "auth.") {
		id = fmt.Sprintf("auth.%s", id)
	}
	key, err := dynamodbattribute.MarshalMap(rce.RecordKey{
		ID: id,
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
	_, err = lib.DynamoDBClient().DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key:       key,
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
}
