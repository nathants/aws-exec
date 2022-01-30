package awsrce

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/nathants/aws-rce/rce"
	"github.com/nathants/cli-aws/lib"
)

func init() {
	lib.Commands["exec"] = exec
	lib.Args["exec"] = execArgs{}
}

type execArgs struct {
	Argv []string `arg:"positional,required"`
}

func (execArgs) Description() string {
	return "\nexec\n"
}

func exec() {
	var args execArgs
	arg.MustParse(&args)

	_ = os.Getenv("PROJECT_NAME")
	domain := os.Getenv("PROJECT_DOMAIN")
	auth := os.Getenv("AUTH")
	url := fmt.Sprintf("https://%s", domain)
	ctx := context.Background()

	postResponse := rce.ExecPostResponse{}
	err := lib.RetryAttempts(ctx, 7, func() error {
		client := http.Client{}
		data, err := json.Marshal(rce.ExecPostRequest{
			Argv: args.Argv,
		})
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, url+"/api/exec", bytes.NewReader(data))
		req.Header.Set("auth", rce.Blake2b32(auth))
		if err != nil {
			return err
		}
		out, err := client.Do(req)
		if err != nil {
			lib.Logger.Println("error:", err)
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
		lib.Logger.Fatal("error: ", err)
	}

	increment := 0
	for {
		getResp := rce.ExecGetResponse{}
		err := lib.RetryAttempts(ctx, 7, func() error {
			client := http.Client{}
			data, err := json.Marshal(rce.ExecGetRequest{
				Uid:       postResponse.Uid,
				Increment: aws.Int(increment),
			})
			if err != nil {
				return err
			}
			req, err := http.NewRequest(http.MethodGet, url+"/api/exec", bytes.NewReader(data))
			if err != nil {
				return err
			}
			req.Header.Set("auth", rce.Blake2b32(auth))
			out, err := client.Do(req)
			if err != nil {
				return err
			}
			defer func() { _ = out.Body.Close() }()
			data, err = ioutil.ReadAll(out.Body)
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
			lib.Logger.Fatal("error: ", err)
		}
		if getResp.HttpCode == 409 {
			lib.Logger.Println("waiting", postResponse.Uid, time.Now())
			time.Sleep(1 * time.Second)
			continue
		}
		if getResp.ExitCode != nil {
			os.Exit(*getResp.ExitCode)
		}
		if getResp.Increment != nil {
			out, err := http.Get(getResp.LogUrl)
			if err != nil {
				panic(err)
			}
			data, err := ioutil.ReadAll(out.Body)
			if err != nil {
				panic(err)
			}
			_ = out.Body.Close()
			fmt.Println(string(data))
			increment = *getResp.Increment
			continue
		}
		panic("unreachable\n" + lib.Pformat(getResp))
	}

}
