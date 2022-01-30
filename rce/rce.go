package rce

import (
	"crypto/rand"
	"encoding/hex"

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

type ExecAsyncEvent struct {
	EventType string   `json:"event_type"`
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
