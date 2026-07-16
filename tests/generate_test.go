package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGenerate(t *testing.T) {
	requireServer(t, "127.0.0.1:8800")
	c := client.NewClient([]string{"http://127.0.0.1:8800"}, 30*time.Second)

	resp, err := c.Generate(context.Background(), &v1.GenerateRequest{
		RequestId: "002",
		ModelId:   "Qwen/Qwen3-0.6B",
		Prompt:    "what is your name? what is your name? what is your name? what is your name? what is your name?",
		MaxTokens: 16,
		TimeoutMs: 60000,
		Labels:    nil,
	})

	require.NoError(t, err)
	r, err := protojson.MarshalOptions{
		Indent:          "  ",
		EmitUnpopulated: true,
	}.Marshal(resp)
	require.NoError(t, err)
	require.Empty(t, resp.ErrorMessage)
	require.NotEqual(t, v1.FinishReasonError, resp.FinishReason)
	t.Log(string(r))
}

func TestGenerateStream(t *testing.T) {
	requireServer(t, "127.0.0.1:8800")
	c := client.NewClient([]string{"http://127.0.0.1:8800"}, 30*time.Second)

	ttftStart := time.Now()
	ttftDone := false

	stream, err := c.GenerateStream(context.Background(), &v1.GenerateRequest{
		UserId:    "Bob",
		RequestId: "003",
		ModelId:   "Qwen/Qwen3-0.6B",
		Prompt: "what is your name? what is your name? what is your name? what is your name? what is your name?" +
			"what is your name? what is your name? what is your name? what is your name? what is your name?" +
			"what is your name? what is your name? what is your name? what is your name? what is your name?",
		MaxTokens: 16,
		TimeoutMs: 60000,
		Labels:    nil,
	})
	require.NoError(t, err)

	for stream.Receive() {
		if !ttftDone {
			t.Logf("TTFT: %s", time.Since(ttftStart))
			ttftDone = true
		}
		chunk := stream.Msg()
		fmt.Print(chunk.DeltaText)
		//r, err := protojson.MarshalOptions{
		//	Indent:          "  ",
		//	EmitUnpopulated: true,
		//}.Marshal(chunk)
		//t.Log(string(r))
		require.NoError(t, err)
		require.Empty(t, chunk.ErrorMessage)
		require.NotEqual(t, v1.FinishReasonError, chunk.FinishReason)
	}
	require.NoError(t, stream.Err())
}
