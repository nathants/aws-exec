package rce

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nathants/cli-aws/lib"
	"golang.org/x/crypto/blake2b"
)

const (
	EventExec       = "exec"
	MaxLogBytes     = 1024 * 1024 * 32 // 30MB takes ~3s to write to s3 from 128mb lambda
	LogShipInterval = 3 * time.Second
)

type ExecGetRequest struct {
	Uid        string `json:"uid"`
	RangeStart int    `json:"range-start"`
}

type ExecGetResponse struct {
	Exit *int   `json:"exit"`
	Url  string `json:"url"`
}

type ExecPostRequest struct {
	Argv []string `json:"argv"`
}

type ExecPostResponse struct {
	Uid string `json:"uid"`
}

type ExecAsyncEvent struct {
	EventType string   `json:"event-type"`
	AuthName  string   `json:"auth-name"`
	Uid       string   `json:"uid"`
	Argv      []string `json:"argv"`
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

func Exec(ctx context.Context, url, auth string, argv []string, logDataCallback func(logs string)) (int, error) {
	postResponse := ExecPostResponse{}
	err := lib.RetryAttempts(ctx, 7, func() error {
		client := http.Client{}
		data, err := json.Marshal(ExecPostRequest{
			Argv: argv,
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
	lib.Logger.Println("uid:", postResponse.Uid)
	rangeStart := 0
	for {
		getResp := ExecGetResponse{}
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
