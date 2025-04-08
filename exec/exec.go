package exec

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nathants/libaws/lib"
	"golang.org/x/crypto/blake2b"
)

type rpcFunc func(ctx context.Context, println func(v ...any), argsJson string) error

var Rpc = map[string]rpcFunc{}

const (
	EventExec       = "exec"
	MaxLogBytes     = 1024 * 1024 * 32 // reasonably upper bound to write to s3 from 128mb lambda
	LogShipInterval = 1 * time.Second
)

type GetRequest struct {
	Uid        string `json:"uid"`
	RangeStart int    `json:"range-start"`
}

type GetResponse struct {
	Exit *int   `json:"exit"`
	Url  string `json:"url"`
}

// s3 presigned put urls
type PushUrls struct {
	Log  string `json:"log"`
	Size string `json:"size"`
	Exit string `json:"exit"`
}

type PostRequest struct {
	PushUrls *PushUrls `json:"push-urls"`

	// to invoke subprocess, provide argv. this is slower.
	Argv []string `json:"argv"`

	// to invoke rpc, provide name and args. this is faster.
	RpcName string `json:"rpc-name"`
	RpcArgs string `json:"rpc-args"`
}

type PostResponse struct {
	Uid string `json:"uid"`
}

type AsyncEvent struct {
	EventType string    `json:"event-type"`
	AuthName  string    `json:"auth-name"`
	Uid       string    `json:"uid"`
	PushUrls  *PushUrls `json:"push-urls"`

	// to invoke subprocess, provide argv. this is slower.
	Argv []string `json:"argv"`

	// to invoke rpc, provide name and args. this is faster.
	RpcName string `json:"rpc-name"`
	RpcArgs string `json:"rpc-args" `
}

type RecordKey struct {
	ID string `json:"id" dynamodbav:"id"`
}

type RecordData struct {
	Value string `json:"value" dynamodbav:"value"`
}

type Record struct {
	RecordKey
	RecordData
}

type Args struct {
	Url             string
	Auth            string
	LogDataCallback func(logs string)
	PushUrls        *PushUrls

	// to invoke subprocess, provide argv. this is slower.
	Argv []string

	// to invoke rpc, provide name and args. this is faster.
	RpcName string
	RpcArgs string
}

// s3 keys to pull data from
type PullKeys struct {
	Log  string
	Size string
	Exit string
}

type TailArgs struct {
	PullBucket      string    // s3 bucket to pull data from
	PullKeys        *PullKeys // s3 keys to pull data from
	LogShipInterval time.Duration
	LogDataCallback func(logs string)
}

func Blake2b32(x string) string {
	val := blake2b.Sum256([]byte(x))
	return hex.EncodeToString(val[:])
}

func RandKey() string {
	val := make([]byte, 32)
	_, err := rand.Read(val)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(val)
}

func CaseInsensitiveGet(m map[string]string, k string) (string, bool) {
	for mk, mv := range m {
		if strings.EqualFold(mk, k) {
			return mv, true
		}
	}
	return "", false
}

// if pushUrls are not provided, data will be persisted by aws-exec and
// this function will poll until process completion, pulling log data
// as it is available and invoking logDataCallback, then returning the
// exit code.
//
// if pushUrls are provided, data will be persisted at those urls via
// http put with content-length set, and this function will return
// immediately. urls should remain valid for 20 minutes. log will be
// pushed repeatedly with the entire log contents. exit will be pushed
// once and will contain the exit code. size will be pushed once, will
// be pushed last, and will contain the size of the final log push.
func Exec(ctx context.Context, args *Args) (int, error) {
	postResponse := PostResponse{}
	var expectedErr error
	err := lib.RetryAttempts(ctx, 7, func() error {
		client := http.Client{}
		data, err := json.Marshal(PostRequest{
			Argv:     args.Argv,
			PushUrls: args.PushUrls,
			RpcName:  args.RpcName,
			RpcArgs:  args.RpcArgs,
		})
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, args.Url+"/api/exec", bytes.NewReader(data))
		req.Header.Set("auth", args.Auth)
		if err != nil {
			return err
		}
		out, err := client.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = out.Body.Close() }()
		data, err = io.ReadAll(out.Body)
		if err != nil {
			return err
		}
		if out.StatusCode == 200 {
			err = json.Unmarshal(data, &postResponse)
			if err != nil {
				return err
			}
			return nil
		}
		if fmt.Sprint(out.StatusCode)[:1] == "5" {
			return fmt.Errorf("%d %s", out.StatusCode, string(data))
		}
		expectedErr = fmt.Errorf("%d %s", out.StatusCode, string(data))
		return nil
	})
	if expectedErr != nil {
		lib.Logger.Println("error:", expectedErr)
		return -1, expectedErr
	}
	if err != nil {
		lib.Logger.Println("error:", err)
		return -1, err
	}
	if args.PushUrls != nil {
		return -1, nil
	}
	rangeStart := 0
	for {
		getResp := GetResponse{}
		err := lib.RetryAttempts(ctx, 7, func() error {
			client := http.Client{}
			req, err := http.NewRequest(http.MethodGet, args.Url+fmt.Sprintf("/api/exec?uid=%s&range-start=%d", postResponse.Uid, rangeStart), nil)
			if err != nil {
				return err
			}
			req.Header.Set("auth", args.Auth)
			out, err := client.Do(req)
			if err != nil {
				return err
			}
			defer func() { _ = out.Body.Close() }()
			data, err := io.ReadAll(out.Body)
			if err != nil {
				return err
			}
			if out.StatusCode != 200 {
				return fmt.Errorf("%d %s\n%s", out.StatusCode, out.Request.URL, string(data))
			}
			err = json.Unmarshal(data, &getResp)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			lib.Logger.Println("error:", err)
			return -1, err
		}
		if getResp.Exit != nil {
			return *getResp.Exit, nil
		}
		var data []byte
		err = lib.RetryAttempts(ctx, 7, func() error {
			req, err := http.NewRequest(http.MethodGet, getResp.Url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("range", fmt.Sprintf("bytes=%d-", rangeStart))
			out, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			data, err = io.ReadAll(out.Body)
			if err != nil {
				return err
			}
			err = out.Body.Close()
			if err != nil {
				return err
			}
			switch out.StatusCode {
			case 200, 206:
				return nil
			case 403, 416:
				time.Sleep(LogShipInterval)
				data = nil
				return nil
			default:
				data = nil
				err := fmt.Errorf("http %d", out.StatusCode)
				lib.Logger.Println("error:", err)
				return err
			}
		})
		if err != nil {
			lib.Logger.Println("error:", err)
			return -1, err
		}
		if len(data) > 0 {
			args.LogDataCallback(string(data))
			rangeStart += len(data)
		}
	}
}

// if pushUrls were provided to Exec(), you can use Tail() to follow
// the output and return the exit code.
func Tail(ctx context.Context, tailArgs *TailArgs) (int, error) {
	rangeStart := 0
	for {
		select {
		case <-ctx.Done():
			err := fmt.Errorf("context done")
			lib.Logger.Println("error:", err)
			return 0, err
		default:
		}
		// once size is known and client has read size bytes, return exit
		var sizeData []byte
		_ = lib.Retry(ctx, func() error {
			outSize, err := lib.S3Client().GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(tailArgs.PullBucket),
				Key:    aws.String(tailArgs.PullKeys.Size),
			})
			if err != nil {
				return nil // continue loop instead of retrying when size object not available
			}
			sizeData, err = io.ReadAll(outSize.Body)
			if err != nil {
				return err
			}
			err = outSize.Body.Close()
			if err != nil {
				return err
			}
			return nil
		})
		if len(sizeData) != 0 {
			size, err := strconv.Atoi(string(sizeData))
			if err != nil {
				lib.Logger.Println("error:", err)
				return 0, err
			}
			if rangeStart == size {
				exitStr := ""
				err := lib.Retry(ctx, func() error {
					outExit, err := lib.S3Client().GetObject(ctx, &s3.GetObjectInput{
						Bucket: aws.String(tailArgs.PullBucket),
						Key:    aws.String(tailArgs.PullKeys.Exit),
					})
					if err != nil {
						return err
					}
					exitData, err := io.ReadAll(outExit.Body)
					if err != nil {
						return err
					}
					err = outExit.Body.Close()
					if err != nil {
						return err
					}
					exitStr = string(exitData)
					return nil
				})
				if err != nil {
					lib.Logger.Println("error:", err)
					return 0, err
				}
				exit, err := strconv.Atoi(string(exitStr))
				if err != nil {
					lib.Logger.Println("error:", err)
					return 0, err
				}
				return exit, nil
			}
		}
		// otherwize process log data for range-start
		var data []byte
		err := lib.Retry(ctx, func() error {
			out, err := lib.S3Client().GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(tailArgs.PullBucket),
				Key:    aws.String(tailArgs.PullKeys.Log),
				Range:  aws.String(fmt.Sprintf("bytes=%d-", rangeStart)),
			})
			if err != nil {
				if strings.Contains(err.Error(), "InvalidRange") {
					time.Sleep(tailArgs.LogShipInterval)
					return nil
				}
				if strings.Contains(err.Error(), "NoSuchKey") {
					return nil
				}
				return err
			}
			data, err = io.ReadAll(out.Body)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			lib.Logger.Println("error:", err)
			return 0, err
		}
		tailArgs.LogDataCallback(string(data))
		rangeStart += len(data)
	}
}
