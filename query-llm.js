#!/usr/bin/env node

const fs = require('fs');
const readline = require('readline');
const config = require('./configs/config.json');

const TIMEOUT = config.chat.timeoutInSeconds
const MAX_RETRY_ATTEMPT = config.chat.maxRetryAttempt;
const MAX_TOKENS = config.chat.maxTokens
const TEMPERATURE = config.chat.temperature
const REPLY_PROMPT = config.reply.prompt
const PREDEFINED_KEYS = config.predefinedKeys

const REASON_PROMPT = config.reason.prompt
const REASON_GUIDELINE = config.reason.guideline
const REASON_EXAMPLE_INQUIRY = config.reason.example.inquiry
const REASON_EXAMPLE_OUTPUT = config.reason.example.output
const REASON_SCHEMA = config.reason.schema

const RESPOND_PROMPT = config.respond.prompt
const RESPOND_GUIDELINE = config.respond.guideline
const RESPOND_SCHEMA = config.respond.schema

const LLM_API_BASE_URL = process.env.LLM_API_BASE_URL || 'https://api.openai.com/v1';
const LLM_API_KEY = process.env.LLM_API_KEY || process.env.OPENAI_API_KEY;
const LLM_CHAT_MODEL = process.env.LLM_CHAT_MODEL;
const LLM_STREAMING = process.env.LLM_STREAMING !== 'no';
const LLM_JSON_SCHEMA = process.env.LLM_JSON_SCHEMA;

const LLM_ZERO_SHOT = process.env.LLM_ZERO_SHOT;
const LLM_DEBUG_CHAT = process.env.LLM_DEBUG_CHAT;
const LLM_DEBUG_PIPELINE = process.env.LLM_DEBUG_PIPELINE;
const LLM_DEBUG_FAIL_EXIT = process.env.LLM_DEBUG_FAIL_EXIT;

const NORMAL = '\x1b[0m';
const BOLD = '\x1b[1m';
const YELLOW = '\x1b[93m';
const MAGENTA = '\x1b[35m';
const RED = '\x1b[91m';
const GREEN = '\x1b[92m';
const CYAN = '\x1b[36m';
const GRAY = '\x1b[90m';
const ARROW = '⇢';
const CHECK = '✓';
const CROSS = '✘';

/**
 * Creates a new function by chaining multiple async functions from left to right.
 *
 * @param  {...any} fns - Functions to chain
 * @returns {function}
 */
const pipe = (...fns) => arg => fns.reduce((d, fn) => d.then(fn), Promise.resolve(arg));

/**
 * Suspends the execution for a specified amount of time.
 *
 * @param {number} ms - The amount of time to suspend execution in milliseconds.
 * @returns {Promise<void>} - A promise that resolves after the specified time has elapsed.
 */
const sleep = async (ms) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Tries to parse a string as JSON, but if that fails, tries adding a
 * closing curly brace or double quote to fix the JSON.
 *
 * @param {string} text
 * @returns {Object}
 */
const unJSON = (text) => {
    try {
        return JSON.parse(text);
    } catch (e) {
        try {
            return JSON.parse(text + '}');
        } catch (e) {
            try {
                return JSON.parse(text + '"}');
            } catch (e) {
                return {};
            }
        }
    }
};


/**
 * Represents a chat message.
 *
 * @typedef {Object} Message
 * @property {'system'|'user'|'assistant'} role
 * @property {string} content
 */

/**
 * A callback function to stream then completion.
 *
 * @callback CompletionHandler
 * @param {string} text
 * @returns {void}
 */

/**
 * Generates a chat completion using a RESTful LLM API service.
 *
 * @param {Array<Message>} messages - List of chat messages.
 * @param {Object} schema - An optional JSON schema for the completion.
 * @param {CompletionHandler=} handler - An optional callback to stream the completion.
 * @returns {Promise<string>} The completion generated by the LLM.
 */

const chat = async (messages, schema, handler = null, attempt = MAX_RETRY_ATTEMPT) => {
    const gemini = LLM_API_BASE_URL.indexOf('generativelanguage.google') > 0;
    const stream = LLM_STREAMING && typeof handler === 'function';
    const model = LLM_CHAT_MODEL || 'gpt-4o-mini';
    const generate = stream ? 'streamGenerateContent?alt=sse&' : 'generateContent?'
    const url = gemini ? `${LLM_API_BASE_URL}/models/${model}:${generate}key=${LLM_API_KEY}` : `${LLM_API_BASE_URL}/chat/completions`
    const auth = (LLM_API_KEY && !gemini) ? { 'Authorization': `Bearer ${LLM_API_KEY}` } : {};
    const stop = ['<|im_end|>', '<|end|>', '<|eot_id|>'];

    const response_format = schema ? {
        type: 'json_schema',
        json_schema: {
            schema,
            name: 'response',
            strict: true
        }
    } : undefined;

    const geminify = schema => ({ ...schema, additionalProperties: undefined });
    const response_schema = response_format ? geminify(schema) : undefined;
    const response_mime_type = response_schema ? 'application/json' : 'text/plain';

    const bundles = messages.map(({ role, content }) => {
        return { role, parts: [{ text: content }] };
    });
    const contents = bundles.filter(({ role }) => role === 'user');
    const system_instruction = bundles.filter(({ role }) => role === 'system').shift();
    const generationConfig = { TEMPERATURE, response_mime_type, response_schema, maxOutputTokens: MAX_TOKENS };

    const body = gemini ?
        { system_instruction, contents, generationConfig } :
        { messages, response_format, model, stop, MAX_TOKENS, TEMPERATURE, stream }

    LLM_DEBUG_CHAT &&
        messages.forEach(({ role, content }) => {
            console.log(`${MAGENTA}${role}:${NORMAL} ${content}`);
        });

    try {

        const response = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', ...auth },
            body: JSON.stringify(body)
        });
        if (!response.ok) {
            throw new Error(`HTTP error with the status: ${response.status} ${response.statusText}`);
        }

        const extract = (data) => {
            const { choices, candidates } = data;
            const first = choices ? choices[0] : candidates[0];
            if (first?.content || first?.message) {
                const content = first?.content ? first.content : first.message.content;
                const parts = content?.parts;
                const answer = parts ? parts.map(part => part.text).join('') : content;
                return answer;
            }
            return '';
        }

        if (!stream) {
            const data = await response.json();
            const answer = extract(data).trim();
            if (LLM_DEBUG_CHAT) {
                if (LLM_JSON_SCHEMA) {
                    const parsed = unJSON(answer);
                    const empty = Object.keys(parsed).length === 0;
                    const formatted = empty ? answer : JSON.stringify(parsed, null, 2);
                    console.log(`${YELLOW}${formatted}${NORMAL}`);
                } else {
                    console.log(`${YELLOW}${answer}${NORMAL}`);
                }
            }
            (answer.length > 0) && handler && handler(answer);
            return answer;
        }

        const parse = (line) => {
            let partial = null;
            const prefix = line.substring(0, 6);
            if (prefix === 'data: ') {
                const payload = line.substring(6);
                try {
                    const data = JSON.parse(payload);
                    const { choices, candidates } = data;
                    if (choices) {
                        const [choice] = choices;
                        const { delta } = choice;
                        partial = delta?.content;
                    } else if (candidates) {
                        partial = extract(data);
                    }
                } catch (e) {
                    // ignore
                } finally {
                    return partial;
                }
            }
            return partial;
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();

        let answer = '';
        let buffer = '';
        while (true) {
            const { value, done } = await reader.read();
            if (done) {
                break;
            }
            const lines = decoder.decode(value).split('\n');
            for (let i = 0; i < lines.length; ++i) {
                const line = buffer + lines[i];
                if (line[0] === ':') {
                    buffer = '';
                    continue;
                }
                if (line === 'data: [DONE]') {
                    break;
                }
                if (line.length > 0) {
                    const partial = parse(line.trim());
                    if (partial === null) {
                        buffer = line;
                    } else if (partial && partial.length > 0) {
                        buffer = '';
                        if (answer.length < 1) {
                            const leading = partial.trim();
                            answer = leading;
                            handler && (leading.length > 0) && handler(leading);
                        } else {
                            answer += partial;
                            handler && handler(partial);
                        }
                    }
                }
            }
        }
        return answer;
    } catch (e) {
        if (e.name === 'TimeoutError') {
            LLM_DEBUG_CHAT && console.log(`Timeout with LLM chat after ${TIMEOUT} seconds`);
        }
        if (attempt > 1 && (e.name === 'TimeoutError' || e.name === 'EvalError')) {
            LLM_DEBUG_CHAT && console.log('Retrying...');
            await sleep((MAX_RETRY_ATTEMPT - attempt + 1) * 1500);
            return await chat(messages, schema, handler, attempt - 1);
        } else {
            throw e;
        }
    }
}


/**
 * Replies to the user. This is zero-shot style.
 *
 * @param {Context} context - Current pipeline context.
 * @returns {Context} Updated pipeline context.
 */

const reply = async (context) => {
    const { inquiry, history, delegates } = context;
    const { enter, leave, stream } = delegates;
    enter && enter('Reply');

    const schema = null;
    const messages = [];
    messages.push({ role: 'system', content: REPLY_PROMPT });
    const relevant = history.slice(-5);
    relevant.forEach(msg => {
        const { inquiry, answer } = msg;
        messages.push({ role: 'user', content: inquiry });
        messages.push({ role: 'assistant', content: answer });
    });

    messages.push({ role: 'user', content: inquiry });
    const answer = await chat(messages, schema, stream);

    leave && leave('Reply', { inquiry, answer });
    return { answer, ...context };
}

/**
 * Break downs a multi-line text based on a number of predefined keys.
 *
 * @param {string} text
 * @returns {Array<string>}
 */

const deconstruct = (text, markers = PREDEFINED_KEYS) => {
    const parts = {};
    const keys = [...markers].reverse();
    const anchor = markers.slice().pop();
    const start = text.lastIndexOf(anchor + ':');
    if (start >= 0) {
        parts[anchor] = text.substring(start).replace(anchor + ':', '').trim();
        let str = text.substring(0, start);
        for (let i = 0; i < keys.length; ++i) {
            const marker = keys[i];
            const pos = str.lastIndexOf(marker + ':');
            if (pos >= 0) {
                const substr = str.substr(pos + marker.length + 1).trim();
                const value = substr.split('\n').shift();
                str = str.slice(0, pos);
                parts[marker] = value;
            }
        }
    }
    return parts;
}

/**
 * Constructs a multi-line text based on a number of key-value pairs.
 *
 * @param {Object} key-value pairs
 * @return {text}
 */
const construct = (kv) => {
    if (LLM_JSON_SCHEMA) {
        return JSON.stringify(kv, null, 2);
    }
    return PREDEFINED_KEYS.filter(key => kv[key]).map(key => {
        const value = kv[key];
        if (value && value.length > 0) {
            return `${key}: ${value}`;
        }
        return null;
    }).join('\n');
}


/**
 * Breaks down the completion into a dictionary containing the thought process,
 * important keyphrases, observation, and topic.
 *
 * @param {string} hint - The hint or example given to the LLM.
 * @param {string} completion - The completion generated by the LLM.
 * @returns {Object} Breakdown of the thought process into a dictionary.
 */
const breakdown = (hint, completion) => {
    const text = hint + completion;
    if (text.startsWith('{')) {
        try {
            return unJSON(text);
        } catch (error) {
            LLM_DEBUG_CHAT && console.error(`Failed to parse JSON: ${text.replaceAll('\n', '')}`);
        }
    }
    let result = deconstruct(text);
    const { topic } = result;
    if (!topic || topic.length === 0) {
        result = deconstruct(text + '\n' + 'TOPIC: general knowledge.');
    }
    return result;
}

/**
 * Returns a formatted string based on the given object.
 *
 * @param {string} [prefix] - An optional prefix string
 * @param {Object} object - The object to format
 * @returns {string} The formatted string
 */
const structure = (prefix, object) => {
    if (LLM_JSON_SCHEMA) {
        const format = prefix ? prefix + ' (JSON with this schema)' : '';
        return format + '\n' + JSON.stringify(object, null, 2) + '\n';
    }

    return (prefix || '') + '\n\n' + construct(object) + '\n';
}

/**
 * Represents the record of an atomic processing.
 *
 * @typedef {Object} Stage
 * @property {string} name
 * @property {number} timestamp (Unix epoch)
 * @property {number} duration (in ms)
 */

/**
 * Represents the contextual information for each pipeline stage.
 *
 * @typedef {Object} Context
 * @property {Array<object>} history
 * @property {string} inquiry
 * @property {string} thought
 * @property {string} keyphrases
 * @property {string} observation
 * @property {string} answer
 * @property {Object.<string, function>} delegates - Impure functions to access the outside world.
 */

/**
 * Performs a basic step-by-step reasoning, in the style of Chain of Thought.
 * The updated context will contains new information such as `keyphrases` and `observation`.
 *
 * @param {Context} context - Current pipeline context.
 * @returns {Context} Updated pipeline context.
 */

/**
 * Performs a basic step-by-step reasoning, in the style of Chain of Thought.
 * The updated context will contains new information such as `keyphrases` and `observation`.
 * If the generated keyphrases is empty, the pipeline will retry the reasoning.
 *
 * @param {Context} context - Current pipeline context.
 * @returns {Context} Updated pipeline context.
 */
const reason = async (context) => {
    const { history, delegates } = context;
    const { enter, leave } = delegates;
    enter && enter('Reason');

    const schema = LLM_JSON_SCHEMA ? REASON_SCHEMA : null;
    let prompt = structure(REASON_PROMPT, REASON_GUIDELINE);
    const relevant = history.slice(-3);
    if (relevant.length === 0) {
        prompt += structure(REASON_EXAMPLE_INQUIRY, REASON_EXAMPLE_OUTPUT);
    }

    const messages = [];
    messages.push({ role: 'system', content: prompt });
    relevant.forEach(msg => {
        const { inquiry, topic, thought, keyphrases, answer } = msg;
        const observation = answer;
        messages.push({ role: 'user', content: inquiry });
        const assistant = construct({ tool: 'Google', thought, keyphrases, observation, topic });
        messages.push({ role: 'assistant', content: assistant });
    });

    const { inquiry } = context;

    messages.push({ role: 'user', content: inquiry });
    const hint = schema ? '' : ['tool: Google', 'thought: '].join('\n');
    (!schema) && messages.push({ role: 'assistant', content: hint });
    const completion = await chat(messages, schema);
    let result = breakdown(hint, completion);
    if (!schema && (!result.keyphrases || result.keyphrases.length === 0)) {
        LLM_DEBUG_CHAT && console.log(`-->${RED}Invalid keyphrases. Trying again...`);
        const hint = ['tool: Google', 'thought: ' + result.thought, 'keyphrases: '].join('\n');
        messages.pop();
        messages.push({ role: 'assistant', content: hint });
        const completion = await chat(messages, schema);
        result = breakdown(hint, completion);
    }
    const { topic, thought, keyphrases, observation } = result;
    leave && leave('Reason', { topic, thought, keyphrases, observation });
    return { topic, thought, keyphrases, observation, ...context };
}

/**
 * Responds to the user's recent message using an LLM.
 * The response from the LLM is available as `answer` in the updated context.
 *
 * @param {Context} context - Current pipeline context.
 * @returns {Context} Updated pipeline context.
 */

const respond = async (context) => {
    const { history, delegates } = context;
    const { enter, leave, stream } = delegates;
    enter && enter('Respond');

    const schema = LLM_JSON_SCHEMA ? RESPOND_SCHEMA : null;
    let prompt = schema ? RESPOND_PROMPT + RESPOND_GUIDELINE : RESPOND_PROMPT;
    const relevant = history.slice(-2);
    if (relevant.length > 0) {
        prompt += '\n';
        prompt += '\n';
        prompt += 'For your reference, you and the user have the following Q&A discussion:\n';
        relevant.forEach(msg => {
            const { inquiry, answer } = msg;
            prompt += `* ${inquiry} ${answer}\n`;
        });
    }

    const messages = [];
    messages.push({ role: 'system', content: prompt });
    const { inquiry, observation } = context;
    messages.push({ role: 'user', content: construct({ inquiry, observation }) });
    (!schema) && messages.push({ role: 'assistant', content: 'Answer: ' });
    const completion = await chat(messages, schema, stream);
    const answer = schema ? breakdown('', completion).answer : completion;

    leave && leave('Respond', { inquiry, observation, answer });
    return { answer, ...context };
}

/**
 * Prints the pipeline stages, mostly for troubleshooting.
 *
 * @param {Array<Stage>} stages
 */
const review = (stages) => {
    console.log();
    console.log(`${MAGENTA}Pipeline review ${NORMAL}`);
    console.log('---------------');
    stages.map((stage, index) => {
        const { name, duration, timestamp, ...fields } = stage;
        console.log(`${GREEN}${ARROW} Stage #${index + 1} ${YELLOW}${name} ${GRAY}[${duration} ms]${NORMAL}`);
        Object.keys(fields).map(key => {
            console.log(`${GRAY}${key}: ${NORMAL}${fields[key]}`);
        });
    });
    console.log();
}

/**
 * Collapses every pair of stages (enter and leave) into one stage,
 * and compute its duration instead of invididual timestamps.
 *
 * @param {Array<object} stage
 * @returns {Array<object>}
 */
const simplify = (stages) => {
    const isOdd = (x) => { return (x % 2) !== 0 };
    return stages.map((stage, index) => {
        if (isOdd(index)) {
            const before = stages[index - 1];
            const duration = stage.timestamp - before.timestamp;
            return { ...stage, duration };
        }
        return stage;
    }).filter((_, index) => isOdd(index));
}

/**
 * Converts an expected answer into a suitable regular expression array.
 *
 * @param {string} match
 * @returns {Array<RegExp>}
 */
const regexify = (match) => {
    const filler = (text, index) => {
        let i = index;
        while (i < text.length) {
            if (text[i] === '/') {
                break;
            }
            ++i;
        }
        return i;
    };

    const pattern = (text, index) => {
        let i = index;
        if (text[i] === '/') {
            ++i;
            while (i < text.length) {
                if (text[i] === '/' && text[i - 1] !== '\\') {
                    break;
                }
                ++i;
            }
        }
        return i;
    };

    const regexes = [];
    let pos = 0;
    while (pos < match.length) {
        pos = filler(match, pos);
        const next = pattern(match, pos);
        if (next > pos && next < match.length) {
            const sub = match.substring(pos + 1, next);
            const regex = RegExp(sub, 'gi');
            regexes.push(regex);
            pos = next + 1;
        } else {
            break;
        }
    }

    if (regexes.length === 0) {
        regexes.push(RegExp(match, 'gi'));
    }

    return regexes;
}

/**
 * Returns all possible matches given a list of regular expressions.
 *
 * @param {string} text
 * @param {Array<RegExp>} regexes
 * @returns {Array<Span>}
 */
const match = (text, regexes) => {
    return regexes.map(regex => {
        const match = regex.exec(text);
        if (!match) {
            return null;
        }
        const [first] = match;
        const { index } = match;
        const { length } = first;
        return { index, length };
    }).filter(span => span !== null);
}

/**
 * Formats the input (using ANSI colors) to highlight the spans.
 *
 * @param {string} text
 * @param {Array<Span>} spans
 * @param {string} color
 * @returns {string}
 */

const highlight = (text, spans, color = BOLD + GREEN) => {
    let result = text;
    spans.sort((p, q) => q.index - p.index).forEach((span) => {
        const { index, length } = span;
        const prefix = result.substring(0, index);
        const content = result.substring(index, index + length);
        const suffix = result.substring(index + length);
        result = `${prefix}${color}${content}${NORMAL}${suffix}`;
    });
    return result;
}

/**
 * Evaluates a test file and executes the test cases.
 *
 * @param {string} filename - The path to the test file.
 */
const evaluate = async (filename) => {
    try {
        let history = [];
        let total = 0;
        let failures = 0;

        const handle = async (line) => {
            const parts = (line && line.length > 0) ? line.split(':') : [];
            if (parts.length >= 2) {
                const role = parts[0];
                const content = line.slice(role.length + 1).trim();
                if (role === 'Story') {
                    console.log();
                    console.log('-----------------------------------');
                    console.log(`Story: ${MAGENTA}${BOLD}${content}${NORMAL}`);
                    console.log('-----------------------------------');
                    history = [];
                } else if (role === 'User') {
                    const inquiry = content;
                    const stages = [];
                    const enter = (name) => { stages.push({ name, timestamp: Date.now() }) };
                    const leave = (name, fields) => { stages.push({ name, timestamp: Date.now(), ...fields }) };
                    const delegates = { enter, leave };
                    const context = { inquiry, history, delegates };
                    process.stdout.write(`  ${inquiry}\r`);
                    const start = Date.now();
                    const pipeline = LLM_ZERO_SHOT ? reply : pipe(reason, respond);
                    const result = await pipeline(context);
                    const duration = Date.now() - start;
                    const { topic, thought, keyphrases, answer } = result;
                    history.push({ inquiry, thought, keyphrases, topic, answer, duration, stages });
                    ++total;
                } else if (role === 'Assistant') {
                    const expected = content;
                    const last = history.slice(-1).pop();
                    if (!last) {
                        console.error('There is no answer yet!');
                        process.exit(-1);
                    } else {
                        const { inquiry, answer, duration, stages } = last;
                        const target = answer;
                        const regexes = regexify(expected);
                        const matches = match(target, regexes);
                        if (matches.length === regexes.length) {
                            console.log(`${GREEN}${CHECK} ${CYAN}${inquiry} ${GRAY}[${duration} ms]${NORMAL}`);
                            console.log(' ', highlight(target, matches));
                            LLM_DEBUG_PIPELINE && review(simplify(stages));
                        } else {
                            ++failures;
                            console.error(`${RED}${CROSS} ${YELLOW}${inquiry} ${GRAY}[${duration} ms]${NORMAL}`);
                            console.error(`Expected ${role} to contain: ${CYAN}${regexes.join(',')}${NORMAL}`);
                            console.error(`Actual ${role}: ${MAGENTA}${target}${NORMAL}`);
                            review(simplify(stages));
                            LLM_DEBUG_FAIL_EXIT && process.exit(-1);
                        }
                    }
                } else if (!LLM_ZERO_SHOT) {
                    if (role === 'Pipeline.Reason.Keyphrases' || role === 'Pipeline.Reason.Topic') {
                        const expected = content;
                        const last = history.slice(-1).pop();
                        if (!last) {
                            console.error('There is no answer yet!');
                            process.exit(-1);
                        } else {
                            const { keyphrases, topic, stages } = last;
                            const target = (role === 'Pipeline.Reason.Keyphrases') ? keyphrases : topic;
                            const regexes = regexify(expected);
                            const matches = match(target, regexes);
                            if (matches.length === regexes.length) {
                                console.log(`${GRAY}    ${ARROW} ${role}:`, highlight(target, matches, GREEN));
                            } else {
                                ++failures;
                                console.error(`${RED}Expected ${role} to contain: ${CYAN}${regexes.join(',')}${NORMAL}`);
                                console.error(`${RED}Actual ${role}: ${MAGENTA}${target}${NORMAL}`);
                                review(simplify(stages));
                                LLM_DEBUG_FAIL_EXIT && process.exit(-1);
                            }
                        }
                    } else {
                        console.error(`Unknown role: ${role}!`);
                        handle.exit(-1);
                    }
                }
            }
        };

        const trim = (input) => {
            const text = input.trim();
            const marker = text.indexOf('#');
            if (marker >= 0) {
                return text.substr(0, marker).trim();
            }
            return text;
        }

        const lines = fs.readFileSync(filename, 'utf-8').split('\n').map(trim);
        for (const i in lines) {
            await handle(lines[i]);
        }
        if (failures <= 0) {
            console.log(`${GREEN}${CHECK}${NORMAL} SUCCESS: ${GREEN}${total} test(s)${NORMAL}.`);
        } else {
            console.log(`${RED}${CROSS}${NORMAL} FAIL: ${GRAY}${total} test(s), ${RED}${failures} failure(s)${NORMAL}.`);
            process.exit(-1);
        }
    } catch (e) {
        console.error('ERROR:', e.toString());
        process.exit(-1);
    }
}

const interact = async () => {
    const history = [];

    let loop = true;
    const io = readline.createInterface({ input: process.stdin, output: process.stdout });
    io.on('close', () => { loop = false; });

    const qa = () => {
        io.question(`${YELLOW}>> ${CYAN}`, async (inquiry) => {
            process.stdout.write(NORMAL);
            if (inquiry === '!review' || inquiry === '/review') {
                const last = history.slice(-1).pop();
                if (!last) {
                    console.log('Nothing to review yet!');
                    console.log();
                } else {
                    const { stages } = last;
                    review(simplify(stages));
                }

            } else {
                let input = '';
                let output = '';
                const stream = (text) => {
                    if (LLM_JSON_SCHEMA) {
                        input += text;
                        const { answer } = unJSON(input);
                        if (answer && answer.length > 0) {
                            process.stdout.write(answer.substring(output.length));
                            output = answer;
                        }
                    } else {
                        process.stdout.write(text);
                    }
                }

                const stages = [];
                const update = (stage, fields) => {
                    if (stage === 'Reason') {
                        const { keyphrases } = fields;
                        if (keyphrases && keyphrases.length > 0) {
                            console.log(`${GRAY}${ARROW} Searching for ${keyphrases}...${NORMAL}`);
                        }
                    }
                }
                const enter = (name) => { stages.push({ name, timestamp: Date.now() }) };
                const leave = (name, fields) => { update(name, fields); stages.push({ name, timestamp: Date.now(), ...fields }) };
                const delegates = { stream, enter, leave };
                const context = { inquiry, history, delegates };
                const start = Date.now();
                const pipeline = LLM_ZERO_SHOT ? reply : pipe(reason, respond);
                const result = await pipeline(context);
                const { topic, thought, keyphrases } = result;
                const duration = Date.now() - start;
                const { answer } = result;
                history.push({ inquiry, thought, keyphrases, topic, answer, duration, stages });
                console.log();
            }
            loop && qa();
        })
    }

    qa();
}


(async () => {
    console.log(`Using LLM at ${LLM_API_BASE_URL} (model: ${GREEN}${LLM_CHAT_MODEL || 'default'}${NORMAL}).`);

    const args = process.argv.slice(2);
    args.forEach(evaluate);
    if (args.length == 0) {
        await interact();
    }
})();
