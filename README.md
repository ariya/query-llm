# Query LLM

Query LLM is a simple, zero-dependency CLI tool for querying an LLM with questions. It works seamlessly with both cloud-based managed LLM services (e.g. [OpenAI GPT](https://platform.openai.com/docs), [Groq](https://groq.com), [OpenRouter](https://openrouter.ai)) and locally hosted LLM servers (e.g. [llama.cpp](https://github.com/ggerganov/llama.cpp), [LocalAI](https://localai.io), [Ollama](https://ollama.com), etc). Internally, it guides the LLM to perform step-by-step reasoning following the [Chain of Thought approach](https://www.promptingguide.ai/techniques/cot).

You’ll need either [Node.js](https://nodejs.org) (>= v18) or [Bun](https://bun.sh) to run Query LLM.

```bash
./query-llm.js
```

For quick answers, pipe your question directly:
```bash
echo "Indonesia travel destinations?" | ./query-llm.js
```

To perform specific tasks:
```bash
echo "Translate into German: thank you" | ./query-llm.js
```

## Using Local LLM Servers

Supported local LLM servers include [llama.cpp](https://github.com/ggerganov/llama.cpp), [Jan](https://jan.ai), [Ollama](https://ollama.com), and [LocalAI](https://localai.io).

To utilize [llama.cpp](https://github.com/ggerganov/llama.cpp) locally with its inference engine, ensure to load a quantized model such as [Phi-3.5 Mini](https://huggingface.co/bartowski/Phi-3.5-mini-instruct-GGUF), or [Llama-3.1 8B](https://huggingface.co/lmstudio-community/Meta-Llama-3.1-8B-Instruct-GGUF). Adjust the environment variable `LLM_API_BASE_URL` accordingly:
```bash
/path/to/llama-server -m Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf
export LLM_API_BASE_URL=http://127.0.0.1:8080/v1
```

To use [Jan](https://jan.ai) with its local API server, refer to [its documentation](https://jan.ai/docs/local-api) and load a model like [Phi-3 Mini](https://huggingface.co/microsoft/Phi-3-mini-4k-instruct-gguf), [LLama-3 8B](https://huggingface.co/QuantFactory/Meta-Llama-3-8B-Instruct-GGUF), or [OpenHermes 2.5](https://huggingface.co/TheBloke/OpenHermes-2.5-Mistral-7B-GGUF) and set the environment variable `LLM_API_BASE_URL`:
```bash
export LLM_API_BASE_URL=http://127.0.0.1:1337/v1
export LLM_CHAT_MODEL='llama3-8b-instruct'
```

To use [Ollama](https://ollama.com) locally, load a model and configure the environment variable `LLM_API_BASE_URL`:
```bash
ollama pull phi3.5
export LLM_API_BASE_URL=http://127.0.0.1:11434/v1
export LLM_CHAT_MODEL='phi3.5'
```

For [LocalAI](https://localai.io), initiate its container and adjust the environment variable `LLM_API_BASE_URL`:
```bash
docker run -ti -p 8080:8080 localai/localai tinyllama-chat
export LLM_API_BASE_URL=http://localhost:3928/v1
```

## Using Managed LLM Services

Supported LLM services include [AI21](https://studio.ai21.com), [Deep Infra](https://deepinfra.com), [DeepSeek](https://platform.deepseek.com/), [Fireworks](https://fireworks.ai), [Groq](https://groq.com), [Hyperbolic](https://www.hyperbolic.xyz), [Lepton](https://lepton.ai), [Mistral](https://console.mistral.ai), [Novita](https://novita.ai), [Octo](https://octo.ai), [OpenAI](https://platform.openai.com), [OpenRouter](https://openrouter.ai), and [Together](https://www.together.ai).

For configuration specifics, refer to the relevant section. The examples use Llama-3.1 8B (or GPT-4o Mini for OpenAI), but any LLM with at least 7B parameters should work just as well, such as Mistral 7B, Qwen-2 7B, or Gemma-2 9B.

* [AI21](https://studio.ai21.com)
```bash
export LLM_API_BASE_URL=https://api.ai21.com/studio/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL=jamba-1.5-mini
```

* [Deep Infra](https://deepinfra.com)
```bash
export LLM_API_BASE_URL=https://api.deepinfra.com/v1/openai
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="meta-llama/Meta-Llama-3.1-8B-Instruct"
```

* [DeepSeek](https://platform.deepseek.com)
```bash
export LLM_API_BASE_URL=https://api.deepseek.com/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="deepseek-chat"
```

* [Fireworks](https://fireworks.ai/)
```bash
export LLM_API_BASE_URL=https://api.fireworks.ai/inference/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="accounts/fireworks/models/llama-v3p1-8b-instruct"
```

* [Groq](https://groq.com/)
```bash
export LLM_API_BASE_URL=https://api.groq.com/openai/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="llama-3.1-8b-instant"
```

* [Hyperbolic](https://www.hyperbolic.xyz)
```bash
export LLM_API_BASE_URL=https://api.hyperbolic.xyz/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="meta-llama/Meta-Llama-3.1-8B-Instruct"
```

* [Lepton](https://lepton.ai)
```bash
export LLM_API_BASE_URL=https://llama3-1-8b.lepton.run/api/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="llama3-1-8b"
```

* [Mistral](https://console.mistral.ai)
```bash
export LLM_API_BASE_URL=https://api.mistral.ai/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="open-mistral-7b"
```

* [Novita](https://novita.ai)
```bash
export LLM_API_BASE_URL=https://api.novita.ai/v3/openai
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="meta-llama/llama-3.1-8b-instruct"
```

* [Octo](https://octo.ai)
```bash
export LLM_API_BASE_URL=https://text.octoai.run/v1/
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="meta-llama-3.1-8b-instruct"
```

* [OpenAI](https://platform.openai.com)
```bash
export LLM_API_BASE_URL=https://api.openai.com/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="gpt-4o-mini"
```

* [OpenRouter](https://openrouter.ai/)
```bash
export LLM_API_BASE_URL=https://openrouter.ai/api/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="meta-llama/llama-3.1-8b-instruct"
```

* [Together](https://www.together.ai/)
```bash
export LLM_API_BASE_URL=https://api.together.xyz/v1
export LLM_API_KEY="yourownapikey"
export LLM_CHAT_MODEL="meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo"
```

## Evaluating Questions

If there is a text file containing pairs of `User` and `Assistant` messages, it can be evaluated with Query LLM:

```
User: Which planet is the largest?
Assistant: The largest planet is /Jupiter/.

User: and the smallest?
Assistant: The smallest planet is /Mercury/.
```

Assuming the above content is in `qa.txt`, executing the following command will initiate a multi-turn conversation with the LLM, asking questions sequentially and verifying answers using regular expressions:
```bash
./query-llm.js qa.txt
```

For additional examples, please refer to the `tests/` subdirectory.

Two environment variables can be used to modify the behavior:

* `LLM_DEBUG_FAIL_EXIT`: When set, Query LLM will exit immediately upon encountering an incorrect answer, and subsequent questions in the file will not be processed.

* `LLM_DEBUG_PIPELINE`: When set, and if the expected regular expression does not match the answer, the internal LLM pipeline will be printed to stdout.
