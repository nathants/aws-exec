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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nathants/libaws/lib"
	"golang.org/x/crypto/blake2b"
)

const (
	EventExec       = "exec"
	MaxLogBytes     = 1024 * 1024 * 32 // 30MB takes ~3s to write to s3 from 128mb lambda
	LogShipInterval = 3 * time.Second
)

type GetRequest struct {
	Uid        string `json:"uid"`
	RangeStart int    `json:"range-start"`
}

type GetResponse struct {
	Exit *int   `json:"exit"`
	Url  string `json:"url"`
}

type PushUrls struct {
	Log  string `json:"log"`
	Size string `json:"size"`
	Exit string `json:"exit"`
}

type PostRequest struct {
	Argv     []string  `json:"argv"`
	PushUrls *PushUrls `json:"push-urls"`
}

type PostResponse struct {
	Uid string `json:"uid"`
}

type AsyncEvent struct {
	EventType string    `json:"event-type"`
	AuthName  string    `json:"auth-name"`
	Uid       string    `json:"uid"`
	Argv      []string  `json:"argv"`
	PushUrls  *PushUrls `json:"push-urls"`
}

type RecordKey struct {
	ID string `json:"id"`
}

type RecordData struct {
	Value string `json:"value"`
}

type Record struct {
	RecordKey
	RecordData
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
// http put with content-length set, and this function will exit
// immediately with exit code -1. urls should remain valid for 20
// minutes. log will be pushed repeatedly with the entire log
// contents. exit will be pushed once and will contain the exit
// code. size will be pushed once, will be pushed last, and will
// contain the size of the final log push.
//
func Exec(ctx context.Context, url, auth string, argv []string, logDataCallback func(logs string), pushUrls *PushUrls) (int, error) {
	postResponse := PostResponse{}
	err := lib.RetryAttempts(ctx, 7, func() error {
		client := http.Client{}
		data, err := json.Marshal(PostRequest{
			Argv:     argv,
			PushUrls: pushUrls,
		})
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, url+"/api/exec", bytes.NewReader(data))
		req.Header.Set("auth", auth)
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
		panic(fmt.Sprintf("%d %s", out.StatusCode, string(data)))
	})
	if err != nil {
		lib.Logger.Println("error:", err)
		return -1, err
	}
	if pushUrls != nil {
		return -1, nil
	}
	rangeStart := 0
	for {
		getResp := GetResponse{}
		err := lib.RetryAttempts(ctx, 7, func() error {
			client := http.Client{}
			req, err := http.NewRequest(http.MethodGet, url+fmt.Sprintf("/api/exec?uid=%s&range-start=%d", postResponse.Uid, rangeStart), nil)
			if err != nil {
				return err
			}
			req.Header.Set("auth", auth)
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
			logDataCallback(string(data))
			rangeStart += len(data)
		}
	}
}

// if pushUrls were provided to Exec(), you can use this function to tail the output and return the exit code.
//
// the values in pushUrls should now be s3 keys, not presigned urls.
//
func Tail(ctx context.Context, logDataCallback func(logs string), bucket string, pushUrls *PushUrls, logShipInterval time.Duration) (int, error) {
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
			outSize, err := lib.S3Client().GetObjectWithContext(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(pushUrls.Size),
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
					outExit, err := lib.S3Client().GetObjectWithContext(ctx, &s3.GetObjectInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(pushUrls.Exit),
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
			out, err := lib.S3Client().GetObjectWithContext(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(pushUrls.Log),
				Range:  aws.String(fmt.Sprintf("bytes=%d-", rangeStart)),
			})
			if err != nil {
				aerr, ok := err.(awserr.Error)
				if ok && aerr.Code() == "InvalidRange" {
					time.Sleep(logShipInterval)
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
		logDataCallback(string(data))
		rangeStart += len(data)
	}
}
