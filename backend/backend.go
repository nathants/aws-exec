//
// attr: name ${PROJECT_NAME}
// attr: concurrency 0
// attr: memory 128
// attr: timeout 900
//
// dynamodb: ${PROJECT_NAME} id:s:hash
// s3: ${PROJECT_BUCKET} cors=true acl=private ttldays=1
//
// trigger: api dns=${PROJECT_DOMAIN}
// trigger: cloudwatch rate(5 minutes)
//
// allow: dynamodb:* arn:aws:dynamodb:*:*:table/${PROJECT_NAME}
// allow: s3:* arn:aws:s3:::${PROJECT_BUCKET}/*
// allow: lambda:InvokeFunction arn:aws:lambda:*:*:function:${PROJECT_NAME}
//
// include: ../frontend/public/index.html.gzip
// include: ../frontend/public/favicon.png
//

package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	sdkLambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dustin/go-humanize"
	uuid "github.com/gofrs/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/nathants/aws-rce/rce"
	"github.com/nathants/cli-aws/lib"
)

func corsHeaders() map[string]string {
	return map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "POST, GET, OPTIONS, PUT, DELETE",
		"Access-Control-Allow-Headers": "auth, content-type",
	}
}

func index() events.APIGatewayProxyResponse {
	headers := map[string]string{
		"Content-Type": "text/html; charset=UTF-8",
	}
	indexBytes, err := ioutil.ReadFile("frontend/public/index.html.gzip")
	if err == nil {
		headers["Content-Encoding"] = "gzip"
	} else {
		indexBytes, err = ioutil.ReadFile("frontend/public/index.html")
		if err != nil {
			panic(err)
		}
	}
	return events.APIGatewayProxyResponse{
		Body:            base64.StdEncoding.EncodeToString(indexBytes),
		IsBase64Encoded: true,
		StatusCode:      200,
		Headers:         headers,
	}
}

func static(path string) events.APIGatewayProxyResponse {
	data, err := ioutil.ReadFile("frontend/public" + path)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
		}
	}
	headers := map[string]string{
		"Content-Type": mime.TypeByExtension("." + last(strings.Split(path, "."))),
	}
	var body string
	if len(data) > 4*1024*1024 {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		_, err = w.Write(data)
		if err != nil {
			panic(err)
		}
		err = w.Close()
		if err != nil {
			panic(err)
		}
		body = base64.StdEncoding.EncodeToString(buf.Bytes())
		headers["Content-Encoding"] = "gzip"
	} else {
		body = base64.StdEncoding.EncodeToString(data)
	}
	return events.APIGatewayProxyResponse{
		Body:            body,
		IsBase64Encoded: true,
		StatusCode:      200,
		Headers:         headers,
	}
}

func last(xs []string) string {
	return xs[len(xs)-1]
}

func notfound() events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		Body:       "404",
		StatusCode: 404,
	}
}

func checkAuth(ctx context.Context, auth string) (string, bool) {
	key, err := dynamodbattribute.MarshalMap(rce.RecordKey{
		ID: fmt.Sprintf("auth.%s", rce.Blake2b32(auth)),
	})
	if err != nil {
		return "", false
	}
	table := os.Getenv("PROJECT_NAME")
	out, err := lib.DynamoDBClient().GetItemWithContext(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(table),
		ConsistentRead: aws.Bool(true),
		Key:            key,
	})
	if err != nil {
		return "", false
	}
	val := rce.Record{}
	err = dynamodbattribute.UnmarshalMap(out.Item, &val)
	if err != nil {
		return "", false
	}
	if val.Value == "" {
		return "", false
	}
	return val.Value + ":" + val.ID[5:21], true
}

func httpExecGet(ctx context.Context, event *events.APIGatewayProxyRequest, res chan<- events.APIGatewayProxyResponse, authName string) {
	bucket := os.Getenv("PROJECT_BUCKET")
	getRequest := rce.ExecGetRequest{
		Uid:       event.QueryStringParameters["uid"],
		Increment: aws.Int(atoi(event.QueryStringParameters["increment"])),
	}
	headers := corsHeaders()
	headers["auth-name"] = authName
	headers["uid"] = getRequest.Uid
	// check for log N
	logKey := fmt.Sprintf("jobs/%s/%s/logs.%05d", authName, getRequest.Uid, *getRequest.Increment)
	_, err := lib.S3Client().HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(logKey),
	})
	if err == nil {
		req, _ := lib.S3Client().GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(logKey),
		})
		url, err := req.Presign(60 * time.Second)
		if err != nil {
			panic(err)
		}
		respData, err := json.Marshal(rce.ExecGetResponse{
			Increment: aws.Int(*getRequest.Increment + 1),
			LogUrl:    url,
		})
		if err != nil {
			panic(err)
		}
		res <- events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       string(respData),
			Headers:    headers,
		}
		return
	}
	// on log N miss, check for exit code
	exitKey := fmt.Sprintf("jobs/%s/%s/exit", authName, getRequest.Uid)
	_, err = lib.S3Client().HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(exitKey),
	})
	if err == nil {

		// on exit hit, check once more for logs since log N and exit could both be written after inital miss for log N
		_, err := lib.S3Client().HeadObjectWithContext(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(logKey),
		})
		if err == nil {
			req, _ := lib.S3Client().GetObjectRequest(&s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(logKey),
			})
			url, err := req.Presign(60 * time.Second)
			if err != nil {
				panic(err)
			}
			respData, err := json.Marshal(rce.ExecGetResponse{
				Increment: aws.Int(*getRequest.Increment + 1),
				LogUrl:    url,
			})
			if err != nil {
				panic(err)
			}
			res <- events.APIGatewayProxyResponse{
				StatusCode: 200,
				Body:       string(respData),
				Headers:    headers,
			}
			return
		}

		// on second log miss, return exit
		out, err := lib.S3Client().GetObjectWithContext(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(exitKey),
		})
		if err != nil {
			panic(err)
		}
		defer func() { _ = out.Body.Close() }()
		outData, err := ioutil.ReadAll(out.Body)
		if err != nil {
			panic(err)
		}
		respData, err := json.Marshal(rce.ExecGetResponse{
			ExitCode: aws.Int(atoi(string(outData))),
		})
		if err != nil {
			panic(err)
		}
		res <- events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       string(respData),
			Headers:    headers,
		}
		return
	}

	// no data, wait
	respData, err := json.Marshal(rce.ExecGetResponse{})
	if err != nil {
		panic(err)
	}
	res <- events.APIGatewayProxyResponse{
		StatusCode: 409,
		Body:       string(respData),
		Headers:    headers,
	}
}

func httpExecPost(ctx context.Context, event *events.APIGatewayProxyRequest, res chan<- events.APIGatewayProxyResponse, authName string) {
	postReqest := rce.ExecPostRequest{}
	if event.IsBase64Encoded {
		data, err := base64.StdEncoding.DecodeString(event.Body)
		if err != nil {
			panic(err)
		}
		event.Body = string(data)
	}
	err := json.Unmarshal([]byte(event.Body), &postReqest)
	if err != nil {
		panic(fmt.Sprint(event.Body, err))
	}
	uid := fmt.Sprintf("%d.%s", time.Now().Unix(), uuid.Must(uuid.NewV4()).String())
	data, err := json.Marshal(rce.ExecAsyncEvent{
		EventType: rce.EventExec,
		Uid:       uid,
		AuthName:  authName,
		Argv:      postReqest.Argv,
	})
	if err != nil {
		panic(err)
	}
	headers := corsHeaders()
	headers["auth-name"] = authName
	headers["uid"] = uid
	out, err := lib.LambdaClient().InvokeWithContext(ctx, &sdkLambda.InvokeInput{
		FunctionName:   aws.String(os.Getenv("AWS_LAMBDA_FUNCTION_NAME")),
		InvocationType: aws.String(sdkLambda.InvocationTypeEvent),
		LogType:        aws.String(sdkLambda.LogTypeNone),
		Payload:        data,
	})
	if err != nil {
		panic(err)
	}
	if *out.StatusCode != 202 {
		panic(out.StatusCode)
	}
	data, err = json.Marshal(rce.ExecPostResponse{
		Uid: uid,
	})
	if err != nil {
		panic(err)
	}
	res <- events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(data),
		Headers:    headers,
	}
}

func fileInfo(file string) map[string]string {
	info, err := os.Stat(file)
	if err != nil {
		panic(err)
	}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	hash := sha256.Sum256(data)
	return map[string]string{
		"sha256": hex.EncodeToString(hash[:]),
		"size":   humanize.Bytes(uint64(info.Size())),
	}
}

func handleApiEvent(ctx context.Context, event *events.APIGatewayProxyRequest, res chan<- events.APIGatewayProxyResponse) {
	if event.Path == "/" {
		res <- index()
		return
	}
	if event.Path == "/_version" {
		data, err := json.Marshal(map[string]map[string]string{
			"backend":  fileInfo("main"),
			"frontend": fileInfo("frontend/public/index.html.gzip"),
		})
		if err != nil {
			panic(err)
		}
		res <- events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       string(data),
		}
		return
	}
	if strings.HasPrefix(event.Path, "/js/main.js") ||
		strings.HasPrefix(event.Path, "/favicon.") {
		res <- static(event.Path)
		return
	}
	if strings.HasPrefix(event.Path, "/api/") {
		if event.HTTPMethod == http.MethodOptions {
			res <- events.APIGatewayProxyResponse{
				StatusCode: 200,
				Headers:    corsHeaders(),
			}
			return
		}
		authName, authOk := checkAuth(ctx, event.Headers["auth"])
		if !authOk {
			res <- unauthorized()
			return
		}
		switch event.Path {
		case "/api/exec":
			switch event.HTTPMethod {
			case http.MethodGet:
				httpExecGet(ctx, event, res, authName)
				return
			case http.MethodPost:
				httpExecPost(ctx, event, res, authName)
				return
			default:
			}
		default:
		}
		res <- notfound()
		return
	}
	res <- notfound()
}

func atoi(x string) int {
	n, err := strconv.Atoi(x)
	if err != nil {
		panic(err)
	}
	return n
}

func unauthorized() events.APIGatewayProxyResponse {
	time.Sleep(1 * time.Second)
	return events.APIGatewayProxyResponse{
		StatusCode: 401,
		Headers:    corsHeaders(),
	}
}

func logRecover(r interface{}, res chan<- events.APIGatewayProxyResponse) {
	stack := string(debug.Stack())
	lib.Logger.Println(r)
	lib.Logger.Println(stack)
	res <- events.APIGatewayProxyResponse{
		StatusCode: 500,
		Body:       fmt.Sprint(r) + "\n" + stack,
	}
}

func handleAsyncEvent(ctx context.Context, event *rce.ExecAsyncEvent, res chan<- events.APIGatewayProxyResponse) {
	bucket := os.Getenv("PROJECT_BUCKET")
	start := time.Now()
	cmd := exec.CommandContext(ctx, event.Argv[0], event.Argv[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	lines := make(chan *string, 128)
	go func() {
		for {
			if time.Since(start) > 14*time.Minute {
				lines <- aws.String("timeout after 14 minutes")
				_ = cmd.Process.Signal(syscall.SIGKILL)
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(1 * time.Second)
			}
		}
	}()
	for _, r := range []io.ReadCloser{stdout, stderr} {
		r := r
		go func() {
			readBuf := bufio.NewReader(r)
			for {
				line, err := readBuf.ReadString('\n')
				if err != nil {
					lines <- nil
					return
				}
				line = strings.TrimRight(line, "\n")
				lines <- &line
			}
		}()
	}
	logsDone := make(chan error)
	go func() {
		increment := 0
		doneCount := 0
		toShip := []string{}
		lastShipped := time.Now()
		shipLogs := func() {
			val := strings.Join(toShip, "\n")
			val = strings.Trim(val, " \n")
			if val == "" {
				return
			}
			key := fmt.Sprintf("jobs/%s/%s/logs.%05d", event.AuthName, event.Uid, increment)
			err = lib.Retry(ctx, func() error {
				_, err := lib.S3Client().PutObject(&s3.PutObjectInput{
					Bucket: aws.String(bucket),
					Key:    aws.String(key),
					Body:   bytes.NewReader([]byte(val)),
				})
				return err
			})
			if err != nil {
				lib.Logger.Println("error:", err)
				return
			}
			toShip = nil
			increment++
			lastShipped = time.Now()
		}
		for {
			select {
			case line := <-lines:
				if line == nil {
					doneCount++
					if doneCount == 2 {
						shipLogs()
						logsDone <- nil
						return
					}
				} else {
					toShip = append(toShip, *line)
				}
			case <-time.After(1 * time.Second):
				// check if logs need to be shipped even when no new output
			}
			if time.Since(lastShipped) > 1*time.Second {
				shipLogs()
			}
		}
	}()
	exitCode := 0
	err = cmd.Start()
	if err != nil {
		lib.Logger.Println("error:", err)
		exitCode = 1
	} else {
		<-logsDone
		err = cmd.Wait()
		if err != nil {
			exitCode = 1
		}
	}
	key := fmt.Sprintf("jobs/%s/%s/exit", event.AuthName, event.Uid)
	err = lib.Retry(ctx, func() error {
		_, err := lib.S3Client().PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader([]byte(fmt.Sprint(exitCode))),
		})
		return err
	})
	if err != nil {
		panic(err)
	}
	res <- events.APIGatewayProxyResponse{
		Body:       "ok",
		StatusCode: 200,
		Headers: map[string]string{
			"auth-name": event.AuthName,
			"uid":       event.Uid,
		},
	}
}

func handle(ctx context.Context, event map[string]interface{}, res chan<- events.APIGatewayProxyResponse) {
	defer func() {
		if r := recover(); r != nil {
			logRecover(r, res)
		}
	}()
	if event["EventType"] == rce.EventExec {
		asyncEvent := &rce.ExecAsyncEvent{}
		err := mapstructure.Decode(event, asyncEvent)
		if err != nil {
			panic(err)
		}
		handleAsyncEvent(ctx, asyncEvent, res)
		return
	}
	_, ok := event["path"]
	if !ok {
		res <- notfound()
		return
	}
	apiEvent := &events.APIGatewayProxyRequest{}
	err := mapstructure.Decode(event, apiEvent)
	if err != nil {
		panic(err)
	}
	handleApiEvent(ctx, apiEvent, res)
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func handleRequest(ctx context.Context, event map[string]interface{}) (events.APIGatewayProxyResponse, error) {
	setupLogging(ctx)
	defer lib.Logger.Flush()
	start := time.Now()
	res := make(chan events.APIGatewayProxyResponse)
	go handle(ctx, event, res)
	r := <-res
	path, ok := event["path"]
	if ok {
		uid := r.Headers["uid"]
		if uid == "" {
			uid = "-"
		}
		authName := r.Headers["auth-name"]
		if authName == "" {
			authName = "-"
		}
		ip := event["requestContext"].(map[string]interface{})["identity"].(map[string]interface{})["sourceIp"].(string)
		lib.Logger.Println("http", r.StatusCode, path, authName, uid, time.Since(start), ip, timestamp())
	} else {
		uid, ok := event["Uid"].(string)
		if !ok {
			uid = "-"
		}
		authName, ok := event["AuthName"].(string)
		if !ok {
			authName = "-"
		}
		eventType, ok := event["EventType"].(string) // our event
		if !ok {
			_, ok = event["detail-type"].(string) // aws scheduled event
			if ok {
				eventType = "scheduled-event"
			} else {
				eventType = "-"
			}
		}
		lib.Logger.Println("async-event", eventType, authName, uid, time.Since(start), timestamp())
	}
	return r, nil
}

func setupLogging(ctx context.Context) {
	lock := sync.RWMutex{}
	var lines []string
	uid := uuid.Must(uuid.NewV4()).String()
	count := 0
	lib.Logger = &lib.LoggerStruct{
		Print: func(args ...interface{}) {
			lock.Lock()
			defer lock.Unlock()
			lines = append(lines, fmt.Sprint(args...))
		},
		Flush: func() {
			lock.Lock()
			defer lock.Unlock()
			if len(lines) == 0 {
				return
			}
			text := strings.Join(lines, "")
			lines = nil
			unix := time.Now().Unix()
			key := fmt.Sprintf("logs/%d.%s.%03d", unix, uid, count)
			count++
			err := lib.Retry(context.Background(), func() error {
				_, err := lib.S3Client().PutObject(&s3.PutObjectInput{
					Bucket: aws.String(os.Getenv("PROJECT_BUCKET")),
					Key:    aws.String(key),
					Body:   bytes.NewReader([]byte(text)),
				})
				return err
			})
			if err != nil {
				lib.Logger.Println("error:", err)
				return
			}
		},
	}
	go func() {
		for {
			lib.Logger.Flush()
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

func main() {
	lambda.Start(handleRequest)
}
