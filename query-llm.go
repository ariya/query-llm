package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	LLMAPIBaseURL = func() string {
		if os.Getenv("LLM_API_BASE_URL") != "" {
			return os.Getenv("LLM_API_BASE_URL")
		} else {
			return "https://api.openai.com/v1"
		}
	}()
	LLMAPIKey = func() string {
		if os.Getenv("LLM_API_KEY") != "" {
			return os.Getenv("LLM_API_KEY")
		} else {
			return os.Getenv("OPENAI_API_KEY")
		}
	}()
	LLMChatModel = func() string {
		if os.Getenv("LLM_CHAT_MODEL") != "" {
			return os.Getenv("LLM_CHAT_MODEL")
		} else {
			return "gpt-4o-mini"
		}
	}()
	LLMStreaming  = os.Getenv("LLM_STREAMING") != "no"
	LLMJsonSchema = os.Getenv("LLM_JSON_SCHEMA")

	LLMZeroShot      = os.Getenv("LLM_ZERO_SHOT")
	LLMDebugChat     = os.Getenv("LLM_DEBUG_CHAT")
	LLMDebugPipeline = os.Getenv("LLM_DEBUG_PIPELINE")
	LLMDebugFailExit = os.Getenv("LLM_DEBUG_FAIL_EXIT")
)

const (
	NORMAL  = "\x1b[0m"
	BOLD    = "\x1b[1m"
	YELLOW  = "\x1b[93m"
	MAGENTA = "\x1b[35m"
	RED     = "\x1b[91m"
	GREEN   = "\x1b[92m"
	CYAN    = "\x1b[36m"
	GRAY    = "\x1b[90m"
	ARROW   = "⇢"
	CHECK   = "✓"
	CROSS   = "✘"

	MAX_RETRY_ATTEMPT  = 3
	TIMEOUT_IN_SECONDS = 17
	MAX_TOKENS         = 200
	TEMPERATURE        = 0 // produces most deterministic output
)

/*
Pipe creates a new function by chaining multiple functions from left to right.

@param fns - Functions to chain.
@returns func(interface{}) interface{} - A function that takes an argument and applies the chained functions to it.
*/
func Pipe(fns ...func(interface{}) interface{}) func(interface{}) interface{} {
	return func(arg interface{}) interface{} {
		result := arg
		for _, fn := range fns {
			result = fn(result)
		}
		return result
	}
}

/*
Sleep suspends the execution for a specified amount of time.

@param ms - The amount of time to suspend execution in milliseconds.
*/
func Sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

/*
UnJSON tries to parse a string as JSON, but if that fails, tries adding a
closing curly brace or double quote to fix the JSON.

@param text - The string to parse as JSON.
@returns map[string]interface{} - The parsed JSON object.
*/
func UnJSON(text string) map[string]interface{} {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err == nil {
		return result
	}
	if err := json.Unmarshal([]byte(text+"}"), &result); err == nil {
		return result
	}
	if err := json.Unmarshal([]byte(text+"\"}"), &result); err == nil {
		return result
	}
	return map[string]interface{}{}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages       []Message              `json:"messages"`
	ResponseFormat map[string]interface{} `json:"response_format"`
	Model          string                 `json:"model"`
	Stop           []string               `json:"stop"`
	MaxTokens      int                    `json:"max_tokens"`
	Temperature    float64                `json:"temperature"`
	Stream         bool                   `json:"stream"`
}

type ChatRequestGemini struct {
	SystemInstruction *GeminiContent         `json:"systemInstruction"`
	Contents          []GeminiContent        `json:"contents"`
	GenerationConfig  GeminiGenerationConfig `json:"generationConfig"`
}

type GeminiContent struct {
	Role  string              `json:"role"`
	Parts []GeminiContentPart `json:"parts"`
}

type GeminiContentPart struct {
	Text string `json:"text"`
}

type GeminiGenerationConfig struct {
	Temperature      float64                `json:"temperature"`
	ResponseMimeType string                 `json:"responseMimeType"`
	ResponseSchema   map[string]interface{} `json:"responseSchema"`
	MaxOutputTokens  int                    `json:"maxOutputTokens"`
}

type Choice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

func chat(
	messages []Message,
	schema map[string]interface{},
	handler func(string),
	maxRetryAttempt int,
) (string, error) {
	isGemini := strings.Contains(LLMAPIBaseURL, "generativelanguage.google")
	isStreaming := LLMStreaming && handler != nil

	req, err := composeRequest(messages, schema, isGemini, isStreaming)
	if err != nil {
		return "", err
	}
	resp, err := sendRequest(err, req)
	if err != nil {
		return "", err
	}

	if !isStreaming {
		var data struct {
			Choices []Choice `json:"choices"`
		}
		err := json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			return "", err
		}
		answer := data.Choices[0].Message.Content
		if handler != nil {
			handler(answer)
		}
		return answer, nil

	} else {
		return handleResponseStream(*resp, handler)
	}
}

func composeRequest(
	messages []Message,
	schema map[string]interface{},
	isGemini bool,
	isStreaming bool,
) (*http.Request, error) {
	url := func() string {
		if isGemini {
			generationType := func() string {
				if isStreaming {
					return "streamGenerateContent?alt=sse&"
				} else {
					return "generateContent?"
				}
			}()
			return fmt.Sprintf("%s/models/%s:%skey=%s", LLMAPIBaseURL, LLMChatModel, generationType, LLMAPIKey)
		}
		return fmt.Sprintf("%s/chat/completions", LLMAPIBaseURL)
	}()
	authHeader := func() string {
		if LLMAPIKey == "" || isGemini {
			return ""
		}
		return fmt.Sprintf("Bearer %s", LLMAPIKey)
	}()

	requestBody := func() any {
		if isGemini {
			var systemInstruction *GeminiContent
			userContents := make([]GeminiContent, 0)

			for _, msg := range messages {
				content := GeminiContent{
					Role: msg.Role,
					Parts: []GeminiContentPart{
						{
							Text: msg.Content,
						},
					},
				}
				if msg.Role == "system" && systemInstruction == nil {
					systemInstruction = &content
				} else if msg.Role == "user" {
					userContents = append(userContents, content)
				}
			}

			responseMimeType := func() string {
				if schema != nil {
					return "application/json"
				} else {
					return "text/plain"
				}
			}()
			responseSchema := func() map[string]interface{} {
				if schema == nil {
					return nil
				}

				newSchema := make(map[string]interface{})
				for k, v := range schema {
					newSchema[k] = v
				}
				newSchema["additionalProperties"] = nil
				return newSchema
			}()

			return ChatRequestGemini{
				SystemInstruction: systemInstruction,
				Contents:          userContents,
				GenerationConfig: GeminiGenerationConfig{
					Temperature:      TEMPERATURE,
					ResponseMimeType: responseMimeType,
					ResponseSchema:   responseSchema,
					MaxOutputTokens:  MAX_TOKENS,
				},
			}
		}

		responseFormat := func() map[string]interface{} {
			if schema == nil {
				return nil
			}

			return map[string]interface{}{
				"type": "json_schema",
				"json_schema": map[string]interface{}{
					"schema": schema,
					"name":   "response",
					"strict": true,
				},
			}
		}()
		return ChatRequest{
			Messages:       messages,
			ResponseFormat: responseFormat,
			Model:          LLMChatModel,
			Stop:           []string{"<|im_end|>", "<|end|>", "<|eot_id|>"},
			MaxTokens:      MAX_TOKENS,
			Temperature:    TEMPERATURE,
			Stream:         isStreaming,
		}
	}()

	if LLMDebugChat != "" {
		for _, message := range messages {
			fmt.Printf("%s%s:%s %s\n", MAGENTA, message.Role, NORMAL, message.Content)
		}
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	return req, nil
}

func sendRequest(err error, req *http.Request) (*http.Response, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	return resp, nil
}

func handleResponseStream(resp http.Response, handler func(string)) (string, error) {
	answer := ""
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		if line[0] == ':' {
			continue
		}
		if line == "data: [DONE]" {
			break
		}
		if strings.HasPrefix(line, "data: ") {
			payload := line[6:]
			var data struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			err := json.Unmarshal([]byte(payload), &data)
			if err != nil {
				return "", err
			}
			partial := data.Choices[0].Delta.Content
			answer += partial
			if handler != nil {
				handler(partial)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return answer, nil
}
