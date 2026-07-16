package tests

import (
	"context"
	"testing"
	"time"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGenerate(t *testing.T) {
	requireServer(t, "127.0.0.1:8800")
	c := client.NewClientWithTimeout([]string{"http://127.0.0.1:8800"}, 30*time.Second)

	resp, err := c.Generate(context.Background(), &v1.GenerateRequest{
		RequestId: "002",
		ModelId:   "Qwen/Qwen3-0.6B",
		Prompt:    "what is your name?",
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
