import type { LLMProvider } from "@/lib/types";

export const LLM_PROVIDERS: {
  value: LLMProvider;
  label: string;
  baseUrl: string;
  model: string;
  models: string[];
  contextWindowK: number;
}[] = [
  {
    value: "anthropic",
    label: "Anthropic",
    baseUrl: "https://api.anthropic.com",
    model: "claude-opus-4-8",
    models: ["claude-opus-4-8", "claude-sonnet-4-6"],
    contextWindowK: 0,
  },
  {
    value: "openai",
    label: "OpenAI-compatible",
    baseUrl: "https://api.openai.com/v1",
    model: "gpt-4o",
    models: ["gpt-4o"],
    contextWindowK: 0,
  },
  {
    value: "openai-responses",
    label: "OpenAI Responses",
    baseUrl: "https://api.openai.com/v1",
    model: "gpt-4o",
    models: ["gpt-4o"],
    contextWindowK: 0,
  },
  {
    value: "grok",
    label: "Grok (xAI)",
    baseUrl: "https://api.x.ai/v1",
    model: "grok-4.7",
    models: ["grok-4.7", "grok-4.6"],
    contextWindowK: 1000,
  },
  {
    value: "deepseek",
    label: "DeepSeek",
    baseUrl: "https://api.deepseek.com",
    model: "deepseek-flash",
    models: ["deepseek-flash", "deepseek-v4-pro"],
    contextWindowK: 1000,
  },
];

export function providerPreset(provider: LLMProvider) {
  return LLM_PROVIDERS.find((p) => p.value === provider) ?? LLM_PROVIDERS[0];
}
