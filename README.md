# Query LLM

> **⚠️ This project is archived.** Modern language models now have built-in reasoning capabilities, making the traditional Chain of Thought approach implemented in this tool unnecessary. Please use the sister project [chat-llm](https://github.com/ariya/chat-llm) instead, which leverages these built-in reasoning capabilities.

**Query LLM** was a CLI tool for querying large language models (LLMs) using the Chain of Thought method. It supported both cloud-based LLM services and locally hosted LLMs.

Basic usage:
```bash
./query-llm.js
echo "Your question here?" | ./query-llm.js
```

## Local and Cloud LLM Support

This tool supported various local LLM servers ([llama.cpp](https://github.com/ggerganov/llama.cpp), [Ollama](https://ollama.com), [LM Studio](https://lmstudio.ai), etc.) and cloud services ([OpenAI](https://platform.openai.com), [Groq](https://groq.com), [OpenRouter](https://openrouter.ai), etc.).

Configuration was done via environment variables:
```bash
# Example for local server
export LLM_API_BASE_URL=http://127.0.0.1:8080/v1
export LLM_CHAT_MODEL='llama3.2'

# Example for cloud service
export LLM_API_BASE_URL=https://api.openai.com/v1
export LLM_API_KEY="your-api-key"
export LLM_CHAT_MODEL="gpt-4o-mini"
```

## Why This Project Is Archived

Many modern language models now have built-in reasoning capabilities that make explicit Chain of Thought prompting unnecessary in most cases. These models can perform complex reasoning internally and generate more accurate responses without step-by-step guidance.

For current LLM interaction needs, please use [chat-llm](https://github.com/ariya/chat-llm), which is designed to work with these newer models and their built-in reasoning capabilities.

## Historical Note

This tool was created when Chain of Thought prompting was a necessary technique to improve reasoning in earlier LLM generations. As language models have evolved, this explicit approach has become less necessary.

For any questions or historical reference, the code remains available in this archived repository.

