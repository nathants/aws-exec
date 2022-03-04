package rce

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/nathants/cli-aws/lib"
	"golang.org/x/crypto/blake2b"
)

const EventExec = "exec"

type ExecGetRequest struct {
	Uid       string `json:"uid"`
	Increment *int   `json:"increment"`
}

type ExecGetResponse struct {
	HttpCode  int
	ExitCode  *int   `json:"exit_code"`
	Increment *int   `json:"increment"`
	LogUrl    string `json:"log"`
}

type ExecPostRequest struct {
	Argv []string
}

type ExecPostResponse struct {
	Uid string `json:"uid"`
}

// no tags because of mapstructure
type ExecAsyncEvent struct {
	EventType string
	AuthName  string
	Uid       string
	Argv      []string
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

func Blake2b32(password string) string {
	val := blake2b.Sum256([]byte(password))
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
		req.Header.Set("auth", Blake2b32(auth))
		if err != nil {
			return err
		}
		out, err := client.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = out.Body.Close() }()
		data, err = ioutil.ReadAll(out.Body)
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
		if out.StatusCode == 409 {
			return fmt.Errorf("%d %s", out.StatusCode, string(data))
		}
		panic(fmt.Sprintf("%d %s", out.StatusCode, string(data)))
	})
	if err != nil {
		lib.Logger.Println("error:", err)
		return -1, err
	}

	lib.Logger.Println("uid:", postResponse.Uid)

	increment := 0
	for {
		getResp := ExecGetResponse{}
		err := lib.RetryAttempts(ctx, 7, func() error {
			client := http.Client{}
			req, err := http.NewRequest(http.MethodGet, url+fmt.Sprintf("/api/exec?uid=%s&increment=%d", postResponse.Uid, increment), nil)
			if err != nil {
				return err
			}
			req.Header.Set("auth", Blake2b32(auth))
			out, err := client.Do(req)
			if err != nil {
				return err
			}
			defer func() { _ = out.Body.Close() }()
			data, err := ioutil.ReadAll(out.Body)
			if err != nil {
				return err
			}
			if out.StatusCode != 200 && out.StatusCode != 409 {
				return fmt.Errorf("%d %s\n%s", out.StatusCode, out.Request.URL, string(data))
			}
			err = json.Unmarshal(data, &getResp)
			if err != nil {
				return err
			}
			getResp.HttpCode = out.StatusCode
			return nil
		})
		if err != nil {
			lib.Logger.Println("error:", err)
			return -1, err
		}
		if getResp.HttpCode == 409 {
			lib.Logger.Println("waiting", postResponse.Uid, time.Now())
			time.Sleep(1 * time.Second)
			continue
		}
		if getResp.ExitCode != nil {
			return *getResp.ExitCode, nil
		}
		if getResp.Increment != nil {
			var data []byte
			err := lib.RetryAttempts(ctx, 7, func() error {
				out, err := http.Get(getResp.LogUrl)
				if err != nil {
					return err
				}
				data, err = ioutil.ReadAll(out.Body)
				if err != nil {
					return err
				}
				_ = out.Body.Close()
				return nil
			})
			if err != nil {
				lib.Logger.Println("error:", err)
				return -1, err
			}
			logDataCallback(string(data))
			increment = *getResp.Increment
			continue
		}
		panic("unreachable\n" + lib.Pformat(getResp))
	}

}
