package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
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

	PREDEFINED_KEYS = []string{"inquiry", "tool", "thought", "keyphrases", "observation", "answer", "topic"}

	REASON_PROMPT = `Use Google to search for the answer. Think step by step.
Always output your thought in following format`
	REASON_GUIDELINE = map[string]string{
		"tool":        "the search engine to use (must be Google)",
		"thought":     "describe your thoughts about the inquiry",
		"keyphrases":  "the important key phrases to search for",
		"observation": "the concise result of the search tool",
		"topic":       "the specific topic covering the inquiry",
	}
	REASON_EXAMPLE_INQUIRY = `
Example:

Given an inquiry "What is Pitch Lake in Trinidad famous for?", you will output:`
	REASON_EXAMPLE_OUTPUT = map[string]string{
		"tool":        "Google",
		"thought":     "This is about geography, I will use Google search",
		"keyphrases":  "Pitch Lake in Trinidad fame",
		"observation": "Pitch Lake in Trinidad is the largest natural deposit of asphalt",
		"topic":       "geography",
	}
	REASON_SCHEMA = map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"tool": map[string]interface{}{
				"type": "string",
			},
			"thought": map[string]interface{}{
				"type": "string",
			},
			"keyphrases": map[string]interface{}{
				"type": "string",
			},
			"observation": map[string]interface{}{
				"type": "string",
			},
			"topic": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{
			"tool",
			"thought",
			"keyphrases",
			"observation",
			"topic",
		},
	}

	RESPOND_PROMPT = `You are an assistant for question-answering tasks.
You are digesting the most recent user's inquiry, thought, and observation.
Your task is to use the observation to answer the inquiry politely and concisely.
You may need to refer to the user's conversation history to understand some context.
There is no need to mention "based on the observation" or "based on the previous conversation" in your answer.
Your answer is in simple English, and at max 3 sentences.
Do not make any apology or other commentary.
Do not use other sources of information, including your memory.
Do not make up new names or come up with new facts.`
	RESPOND_GUIDELINE = `
Always answer in JSON with the following format:

{
    "answer": // accurate and polite answer
}`
	RESPOND_SCHEMA = map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"answer": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{
			"answer",
		},
	}

	REPLY_PROMPT = `You are a helpful answering assistant.
Your task is to reply and respond to the user politely and concisely.
Answer in plain text and not in Markdown format.`
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
	TEMPERATURE        = 0 // produces most deterministic
)

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

type Context struct {
	History     []History
	Inquiry     string
	Thought     string
	Keyphrases  string
	Topic       string
	Observation string
	Answer      string
	Delegates   Delegates
}

type History struct {
	Inquiry     string
	Thought     string
	Keyphrases  string
	Topic       string
	Observation string
	Answer      string
	Duration    int64
	Stages      []Stage
}

type Delegates struct {
	Enter  func(string)
	Leave  func(string, map[string]interface{})
	Stream func(string)
}

// Stage represents the record of an atomic processing.
type Stage struct {
	Name      string
	Timestamp int64
	Duration  int64
	Fields    map[string]interface{}
}

// Span represents a match span with index and length.
type Span struct {
	Index  int
	Length int
}

// review prints the pipeline stages, mostly for troubleshooting.
func review(stages []Stage) {
	fmt.Println()
	fmt.Println("Pipeline review")
	fmt.Println("---------------")
	for index, stage := range stages {
		fmt.Printf("Stage #%d %s [%d ms]\n", index+1, stage.Name, stage.Duration)
		for key, value := range stage.Fields {
			fmt.Printf("%s: %v\n", key, value)
		}
	}
	fmt.Println()
}

// construct constructs a multi-line text based on a number of key-value pairs.
func construct(kv map[string]string) string {
	if LLMJsonSchema != "" {
		jsonData, _ := json.MarshalIndent(kv, "", "  ")
		return string(jsonData)
	}

	var result []string
	for _, key := range PREDEFINED_KEYS {
		if value, exists := kv[key]; exists && len(value) > 0 {
			result = append(result, key+": "+value)
		}
	}
	return strings.Join(result, "\n")
}

// breakdown breaks down the completion into a dictionary containing the thought process, important keyphrases, observation, and topic.
func breakdown(hint, completion string) map[string]string {

	// Deconstruct breaks down a multi-line text based on a number of predefined keys.
	deconstruct := func(text string, markers []string) map[string]string {
		if markers == nil {
			markers = PREDEFINED_KEYS
		}

		reverse := func(s []string) {
			for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
				s[i], s[j] = s[j], s[i]
			}
		}

		parts := make(map[string]string)
		keys := make([]string, len(markers))
		copy(keys, markers)
		reverse(keys)
		anchor := markers[len(markers)-1]
		start := strings.LastIndex(text, anchor+":")
		if start >= 0 {
			parts[anchor] = strings.TrimSpace(strings.Replace(text[start:], anchor+":", "", 1))
			str := text[:start]
			for _, marker := range keys {
				pos := strings.LastIndex(str, marker+":")
				if pos >= 0 {
					substr := strings.TrimSpace(str[pos+len(marker)+1:])
					value := strings.SplitN(substr, "\n", 2)[0]
					str = str[:pos]
					parts[marker] = value
				}
			}
		}
		return parts
	}

	convertMap := func(original map[string]interface{}) map[string]string {
		converted := make(map[string]string)
		for key, value := range original {
			converted[key] = fmt.Sprintf("%v", value)
		}
		return converted
	}

	text := hint + completion
	if strings.HasPrefix(text, "{") {
		result := unJSON(text)
		if result != nil {
			return convertMap(result)
		}
		if LLMDebugChat != "" {
			fmt.Printf("Failed to parse JSON: %s\n", strings.ReplaceAll(text, "\n", ""))
		}
	}
	result := deconstruct(text, nil)
	if topic, exists := result["topic"]; !exists || len(topic) == 0 {
		result = deconstruct(text+"\n"+"TOPIC: general knowledge.", nil)
	}
	return result
}

// structure returns a formatted string based on the given object.
func structure(prefix string, object map[string]string) string {
	if LLMJsonSchema != "" {
		format := prefix
		if prefix != "" {
			format += " (JSON with this schema)"
		}
		jsonData, _ := json.MarshalIndent(object, "", "  ")
		return format + "\n" + string(jsonData) + "\n"
	}

	return prefix + "\n\n" + construct(object) + "\n"
}

// simplify collapses every pair of stages (enter and leave) into one stage,
// and computes its duration instead of individual timestamps.
func simplify(stages []Stage) []Stage {
	isOdd := func(x int) bool {
		return x%2 != 0
	}

	var simplified []Stage
	for i, stage := range stages {
		if isOdd(i) {
			before := stages[i-1]
			duration := stage.Timestamp - before.Timestamp
			simplified = append(simplified, Stage{
				Name:      stage.Name,
				Timestamp: stage.Timestamp,
				Duration:  duration,
				Fields:    stage.Fields,
			})
		}
	}
	return simplified
}

// regexify converts an expected answer into a suitable regular expression array.
func regexify(match string) []*regexp.Regexp {
	filler := func(text string, index int) int {
		i := index
		for i < len(text) {
			if text[i] == '/' {
				break
			}
			i++
		}
		return i
	}

	pattern := func(text string, index int) int {
		i := index
		if text[i] == '/' {
			i++
			for i < len(text) {
				if text[i] == '/' && text[i-1] != '\\' {
					break
				}
				i++
			}
		}
		return i
	}

	var regexes []*regexp.Regexp
	pos := 0
	for pos < len(match) {
		pos = filler(match, pos)
		next := pattern(match, pos)
		if next > pos && next < len(match) {
			sub := match[pos+1 : next]
			regex, err := regexp.Compile("(?i)" + sub)
			if err == nil {
				regexes = append(regexes, regex)
			}
			pos = next + 1
		} else {
			break
		}
	}

	if len(regexes) == 0 {
		regex, err := regexp.Compile("(?i)" + match)
		if err == nil {
			regexes = append(regexes, regex)
		}
	}

	return regexes
}

// match returns all possible matches given a list of regular expressions.
func match(text string, regexes []*regexp.Regexp) []Span {
	var spans []Span
	for _, regex := range regexes {
		matches := regex.FindStringSubmatchIndex(text)
		if matches != nil {
			spans = append(spans, Span{
				Index:  matches[0],
				Length: matches[1] - matches[0],
			})
		}
	}
	return spans
}

// highlight formats the input (using ANSI colors) to highlight the spans.
func highlight(text string, spans []Span, color string) string {
	result := text
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Index > spans[j].Index
	})
	for _, span := range spans {
		prefix := result[:span.Index]
		content := result[span.Index : span.Index+span.Length]
		suffix := result[span.Index+span.Length:]
		result = fmt.Sprintf("%s%s%s%s%s", prefix, color, content, NORMAL, suffix)
	}
	return result
}

// pipe creates a new function by chaining multiple functions from left to right.
func pipe(fns ...func(ctx Context) (*Context, error)) func(ctx Context) (*Context, error) {
	return func(ctx Context) (*Context, error) {
		var err error
		result := &ctx
		for _, fn := range fns {
			result, err = fn(*result)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
}

// sleep suspends the execution for a specified amount of time.
func sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// unJSON tries to parse a string as JSON, but if that fails, tries adding a
// closing curly brace or double quote to fix the JSON.
func unJSON(text string) map[string]interface{} {
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

func chat(
	messages []Message,
	schema map[string]interface{},
	handler func(string),
	maxRetryAttempt *int,
) (string, error) {
	composeRequest := func(
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

	sendRequest := func(req *http.Request) (*http.Response, error) {
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

	isGemini := strings.Contains(LLMAPIBaseURL, "generativelanguage.google")
	isStreaming := LLMStreaming && handler != nil

	req, err := composeRequest(messages, schema, isGemini, isStreaming)
	if err != nil {
		return "", err
	}
	resp, err := sendRequest(req)
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
		handleResponseStream := func(resp http.Response, handler func(string)) (string, error) {
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

		return handleResponseStream(*resp, handler)
	}
}

// reply generates a response based on the context's inquiry and chat history.
func reply(context Context) (*Context, error) {
	history := context.History
	delegates := context.Delegates

	if delegates.Enter != nil {
		delegates.Enter("Reply")
	}

	messages := []Message{
		{Role: "system", Content: REPLY_PROMPT},
	}

	relevant := history[len(history)-5:]
	for _, msg := range relevant {
		messages = append(messages, Message{Role: "user", Content: msg.Inquiry})
		messages = append(messages, Message{Role: "assistant", Content: msg.Answer})
	}

	messages = append(messages, Message{
		Role:    "user",
		Content: context.Inquiry,
	})
	answer, err := chat(messages, nil, delegates.Stream, nil)
	if err != nil {
		return nil, err
	}

	if delegates.Leave != nil {
		delegates.Leave("Reply", map[string]interface{}{
			"inquiry": context.Inquiry,
			"answer":  answer,
		})
	}

	context.Answer = answer
	return &context, nil
}

// reason performs a basic step-by-step reasoning, in the style of Chain of Thought.
// The updated context will contain new information such as `keyphrases` and `observation`.
// If the generated keyphrases are empty, the pipeline will retry the reasoning.
func reason(context Context) (*Context, error) {
	history := context.History
	delegates := context.Delegates

	if delegates.Enter != nil {
		delegates.Enter("Reason")
	}

	schema := func() map[string]interface{} {
		if LLMJsonSchema == "" {
			return nil
		}
		return REASON_SCHEMA
	}()
	prompt := structure(REASON_PROMPT, REASON_GUIDELINE)
	relevant := history[len(history)-3:]
	if len(relevant) == 0 {
		prompt += structure(REASON_EXAMPLE_INQUIRY, REASON_EXAMPLE_OUTPUT)
	}

	var messages []Message
	messages = append(messages, Message{Role: "system", Content: prompt})
	for _, msg := range relevant {
		messages = append(messages, Message{Role: "user", Content: msg.Inquiry})
		assistant := construct(map[string]string{
			"tool":        "Google",
			"thought":     msg.Thought,
			"keyphrases":  msg.Keyphrases,
			"observation": msg.Answer,
			"topic":       msg.Topic,
		})
		messages = append(messages, Message{Role: "assistant", Content: assistant})
	}

	inquiry := context.Inquiry
	messages = append(messages, Message{Role: "user", Content: inquiry})
	hint := ""
	if schema == nil {
		hint = "tool: Google\nthought: "
		messages = append(messages, Message{Role: "assistant", Content: hint})
	}
	completion, err := chat(messages, schema, nil, nil)
	if err != nil {
		return &context, err
	}
	result := breakdown(hint, completion)
	if schema == nil && (result["keyphrases"] == "" || len(result["keyphrases"]) == 0) {
		if LLMDebugChat != "" {
			fmt.Println("--> Invalid keyphrases. Trying again...")
		}
		hint = "tool: Google\nthought: " + result["thought"] + "\nkeyphrases: "
		messages = messages[:len(messages)-1]
		messages = append(messages, Message{Role: "assistant", Content: hint})
		completion, err = chat(messages, schema, nil, nil)
		if err != nil {
			return &context, err
		}
		result = breakdown(hint, completion)
	}
	topic := result["topic"]
	thought := result["thought"]
	keyphrases := result["keyphrases"]
	observation := result["observation"]
	if delegates.Leave != nil {
		delegates.Leave("Reason", map[string]interface{}{
			"topic":       topic,
			"thought":     thought,
			"keyphrases":  keyphrases,
			"observation": observation,
		})
	}

	context.History = append(context.History, History{
		Inquiry:     inquiry,
		Thought:     thought,
		Keyphrases:  keyphrases,
		Topic:       topic,
		Observation: observation,
	})
	return &context, nil
}

// respond responds to the user's recent message using an LLM.
// The response from the LLM is available as `answer` in the updated context.
func respond(context Context) (*Context, error) {
	history := context.History
	delegates := context.Delegates

	if delegates.Enter != nil {
		delegates.Enter("Respond")
	}

	schema := RESPOND_SCHEMA
	if LLMJsonSchema == "" {
		schema = nil
	}
	prompt := RESPOND_PROMPT
	if schema != nil {
		prompt += RESPOND_GUIDELINE
	}
	relevant := history[len(history)-2:]
	if len(relevant) > 0 {
		prompt += "\n\nFor your reference, you and the user have the following Q&A discussion:\n"
		for _, msg := range relevant {
			prompt += fmt.Sprintf("* %s %s\n", msg.Inquiry, msg.Answer)
		}
	}

	var messages []Message
	messages = append(messages, Message{Role: "system", Content: prompt})
	inquiry := context.Inquiry
	observation := context.Observation
	messages = append(messages, Message{Role: "user", Content: construct(map[string]string{"inquiry": inquiry, "observation": observation})})
	if schema == nil {
		messages = append(messages, Message{Role: "assistant", Content: "Answer: "})
	}
	completion, err := chat(messages, schema, delegates.Stream, nil)
	if err != nil {
		return &context, err
	}
	answer := completion
	if schema != nil {
		answer = breakdown("", completion)["answer"]
	}

	if delegates.Leave != nil {
		delegates.Leave("Respond", map[string]interface{}{
			"inquiry":     inquiry,
			"observation": observation,
			"answer":      answer,
		})
	}

	context.History = append(context.History, History{
		Inquiry:     inquiry,
		Observation: observation,
		Answer:      answer,
	})
	return &context, nil
}

// evaluate evaluates a test file and executes the test cases.
func evaluate(filename string) {
	history := make([]History, 0)
	total := 0
	failures := 0

	handle := func(line string) error {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			return nil
		}
		role := parts[0]
		content := strings.TrimSpace(parts[1])

		if role == "Story" {
			fmt.Println()
			fmt.Println("-----------------------------------")
			fmt.Printf("Story: %s%s%s\n", MAGENTA, BOLD, content, NORMAL)
			fmt.Println("-----------------------------------")
			history = make([]History, 0)

		} else if role == "User" {
			inquiry := content
			stages := make([]Stage, 0)
			enter := func(name string) {
				stages = append(stages, Stage{Name: name, Timestamp: time.Now().UnixNano() / int64(time.Millisecond)})
			}
			leave := func(name string, fields map[string]interface{}) {
				stages = append(stages, Stage{Name: name, Timestamp: time.Now().UnixNano() / int64(time.Millisecond), Fields: fields})
			}

			context := Context{
				Inquiry: inquiry,
				History: history,
				Delegates: Delegates{
					Enter: enter,
					Leave: leave,
				},
			}
			fmt.Printf("  %s\r", inquiry)
			start := time.Now()
			pipeline := func() func(Context) (*Context, error) {
				if LLMZeroShot != "" {
					return reply
				} else {
					return pipe(reason, respond)
				}
			}()
			result, err := pipeline(context)
			if err != nil {
				return nil
			}
			duration := time.Since(start).Milliseconds()

			history = append(history, History{
				Inquiry:    inquiry,
				Thought:    result.Thought,
				Keyphrases: result.Keyphrases,
				Topic:      result.Topic,
				Answer:     result.Answer,
				Duration:   duration,
				Stages:     stages,
			})
			total++

		} else if role == "Assistant" {
			expected := content
			if len(history) == 0 {
				fmt.Println("There is no answer yet!")
				os.Exit(-1)
			}
			last := history[len(history)-1]

			inquiry := last.Inquiry
			answer := last.Answer
			duration := last.Duration
			stages := last.Stages
			target := answer
			regexes := regexify(expected)
			matches := match(target, regexes)

			if len(matches) == len(regexes) {
				fmt.Printf("%s%s %s%s %s[%d ms]%s\n", GREEN, CHECK, CYAN, inquiry, GRAY, duration, NORMAL)
				fmt.Println(" ", highlight(target, matches, GREEN))
				if LLMDebugPipeline != "" {
					review(simplify(stages))
				}
			} else {
				failures++
				fmt.Printf("%s%s %s%s %s[%d ms]%s\n", RED, CROSS, YELLOW, inquiry, GRAY, duration, NORMAL)
				fmt.Printf("Expected %s to contain: %s%s%s\n", role, CYAN, regexes, NORMAL)
				fmt.Printf("Actual %s: %s%s%s\n", role, MAGENTA, target, NORMAL)
				review(simplify(stages))
				if LLMDebugFailExit != "" {
					os.Exit(-1)
				}
			}

		} else if LLMZeroShot == "" {
			if role == "Pipeline.Reason.Keyphrases" || role == "Pipeline.Reason.Topic" {
				expected := content
				if len(history) == 0 {
					fmt.Println("There is no answer yet!")
					os.Exit(-1)
				} else {
					last := history[len(history)-1]
					var target string
					if role == "Pipeline.Reason.Keyphrases" {
						target = last.Keyphrases
					} else {
						target = last.Topic
					}
					regexes := regexify(expected)
					matches := match(target, regexes)
					if len(matches) == len(regexes) {
						fmt.Printf("%s    %s %s: %s\n", GRAY, ARROW, role, highlight(target, matches, GREEN))
					} else {
						failures++
						fmt.Printf("%sExpected %s to contain: %s%s%s\n", RED, role, CYAN, regexes, NORMAL)
						fmt.Printf("%sActual %s: %s%s%s\n", RED, role, MAGENTA, target, NORMAL)
						review(simplify(last.Stages))
						if LLMDebugFailExit != "" {
							os.Exit(-1)
						}
					}
				}
			} else {
				fmt.Printf("Unknown role: %s!\n", role)
				os.Exit(-1)
			}
		}
		return nil
	}

	trim := func(input string) string {
		text := strings.TrimSpace(input)
		marker := strings.Index(text, "#")
		if marker >= 0 {
			return strings.TrimSpace(text[:marker])
		}
		return text
	}

	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(-1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		err := handle(trim(scanner.Text()))
		if err != nil {
			fmt.Println("ERROR:", err)
			os.Exit(-1)
		}
	}

	if failures <= 0 {
		fmt.Printf("%s%s%s SUCCESS: %s%d test(s)%s.\n", GREEN, CHECK, NORMAL, GREEN, total, NORMAL)
	} else {
		fmt.Printf("%s%s%s FAIL: %s%d test(s), %s%d failure(s)%s.\n", RED, CROSS, NORMAL, GRAY, total, RED, failures, NORMAL)
		os.Exit(-1)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(-1)
	}
}
