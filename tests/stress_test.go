//go:build stress

package tests

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
)

const stressTimeout = 180 * time.Second

func TestStressGenerate(t *testing.T) {
	requireServer(t, "127.0.0.1:8800")
	c := client.NewClient([]string{"http://127.0.0.1:8800"}, stressTimeout)
	var wg sync.WaitGroup

	msgNumber := 100
	errCh := make(chan error, msgNumber)

	for i := 0; i < msgNumber; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := c.Generate(context.Background(), &v1.GenerateRequest{
				RequestId: "001" + strconv.Itoa(i),
				UserId:    "001" + strconv.Itoa(i),
				Model:     "deepseek-v4",
				Prompt:    "hello world.",
				MaxTokens: 8,
				TimeoutMs: uint32(stressTimeout.Milliseconds()),
			})
			if err != nil {
				errCh <- err
				return
			}
			if resp == nil {
				errCh <- fmt.Errorf("nil response")
				return
			}
			if resp.ErrorMessage != "" {
				errCh <- fmt.Errorf("requestId: %s err: %s", resp.RequestId, resp.ErrorMessage)
			}
		}(i)
	}
	wg.Wait()
	reportStressErrors(t, errCh)
}

func TestStressGenerateWithPrefixCache(t *testing.T) {
	requireServer(t, "127.0.0.1:8800")
	c := client.NewClient([]string{"http://127.0.0.1:8800"}, stressTimeout)
	const prompt = "hello world, hello world, hello world, hello world, " +
		"hello world, hello world, hello world, hello world, " +
		"hello world, hello world, hello world, hello world, " +
		"hello world, hello world, hello world, hello world, " +
		"hello world, hello world, hello world, hello world."

	runStressRequests(t, c, 10, func(i int) *v1.GenerateRequest {
		return stressRequest("measured-"+strconv.Itoa(i), "001"+strconv.Itoa(i), prompt)
	})
	runStressRequests(t, c, 100, func(i int) *v1.GenerateRequest {
		return stressRequest("001"+strconv.Itoa(i), "001"+strconv.Itoa(i%10), prompt)
	})
}

func runStressRequests(
	t *testing.T,
	c *client.InferenceClient,
	count int,
	request func(int) *v1.GenerateRequest,
) {
	t.Helper()

	var wg sync.WaitGroup
	errCh := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := c.Generate(context.Background(), request(i))
			if err != nil {
				errCh <- err
				return
			}
			if resp == nil {
				errCh <- fmt.Errorf("nil response")
				return
			}
			if resp.ErrorMessage != "" {
				errCh <- fmt.Errorf("requestId: %s err: %s", resp.RequestId, resp.ErrorMessage)
			}
		}(i)
	}
	wg.Wait()
	reportStressErrors(t, errCh)
}

func stressRequest(requestID, userID, prompt string) *v1.GenerateRequest {
	return &v1.GenerateRequest{
		RequestId: requestID,
		UserId:    userID,
		Model:     "deepseek-v4",
		Prompt:    prompt,
		MaxTokens: 8,
		TimeoutMs: uint32(stressTimeout.Milliseconds()),
	}
}

func reportStressErrors(t *testing.T, errCh chan error) {
	t.Helper()

	close(errCh)
	errNum := 1
	for err := range errCh {
		t.Errorf("errNum: %d error: %v", errNum, err)
		errNum++
	}
}
