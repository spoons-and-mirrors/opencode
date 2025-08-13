import { App } from "../app/app"
import { Config } from "../config/config"
import z from "zod"
import { Provider } from "../provider/provider"
import { generateObject, type ModelMessage } from "ai"
import PROMPT_GENERATE from "./generate.txt"
import { SystemPrompt } from "../session/system"
import { mergeDeep } from "remeda"

export namespace Agent {
  export const Info = z
    .object({
      name: z.string(),
      description: z.string().optional(),
      mode: z.union([z.literal("subagent"), z.literal("primary"), z.literal("all")]),
      topP: z.number().optional(),
      temperature: z.number().optional(),
      permission: z.object({
        edit: Config.Permission,
        bash: z.record(z.string(), Config.Permission),
        webfetch: Config.Permission.optional(),
      }),
      model: z
        .object({
          modelID: z.string(),
          providerID: z.string(),
        })
        .optional(),
      prompt: z.string().optional(),
      tools: z.record(z.boolean()),
      options: z.record(z.string(), z.any()),
    })
    .openapi({
      ref: "Agent",
    })
  export type Info = z.infer<typeof Info>

  const state = App.state("agent", async () => {
    const cfg = await Config.get()
    const defaultPermission: Info["permission"] = {
      edit: "allow",
      bash: {
        "*": "allow",
      },
      webfetch: "allow",
    }
    const result: Record<string, Info> = {
      general: {
        name: "general",
        description:
          "General-purpose agent for researching complex questions, searching for code, and executing multi-step tasks. When you are searching for a keyword or file and are not confident that you will find the right match in the first few tries use this agent to perform the search for you.",
        tools: {
          todoread: false,
          todowrite: false,
        },
        options: {},
        permission: defaultPermission,
        mode: "subagent",
      },
      build: {
        name: "build",
        tools: {},
        options: {},
        permission: defaultPermission,
        mode: "primary",
      },
      plan: {
        name: "plan",
        options: {},
        permission: defaultPermission,
        tools: {
          write: false,
          edit: false,
          patch: false,
        },
        mode: "primary",
      },
      research: {
        name: "research",
        description:
          "Expert research agent that systematically analyzes codebases and breaks down complex research tasks. Excels at codebase exploration, pattern analysis, and delegating focused research to subagents. Does not implement code - only researches and analyzes.",
        prompt: `You are an expert research agent specializing in comprehensive codebase analysis and systematic research methodology. Your core mission is to thoroughly understand, analyze, and research codebases while efficiently delegating specific research tasks to subagents.

## Your Role and Expertise

You are a master of codebase exploration with deep expertise in:
- **Pattern Recognition**: Identifying architectural patterns, design principles, and code organization structures
- **Dependency Analysis**: Understanding how components interact and depend on each other
- **Code Flow Tracing**: Following execution paths and data flows through complex systems
- **Research Methodology**: Breaking down complex questions into focused, actionable research tasks
- **Tool Selection**: Knowing which tools and approaches work best for different types of investigation

## Core Principles

1. **Read-Only Focus**: You NEVER implement, edit, or modify code. You are purely analytical and investigative.

2. **Systematic Approach**: Break down complex research into logical, sequential steps. Start with high-level understanding before diving into specifics.

3. **Efficient Tool Usage**: 
   - Use grep, glob, and ls for initial exploration
   - Use read tool for deep file analysis
   - Use bash for advanced text processing and analysis
   - Use webfetch sparingly and only when local resources are insufficient
   - Avoid redundant searches - be strategic about what you investigate

4. **Strategic Delegation**: When research becomes complex or requires specialized focus, delegate specific sub-tasks to subagents using the task tool. Create clear, focused prompts that give subagents everything they need to succeed.

## Research Methodology

### Phase 1: Initial Reconnaissance
- Understand overall project structure and purpose
- Identify key directories, build systems, and configuration files
- Map out major components and their relationships

### Phase 2: Focused Investigation  
- Drill down into specific areas of interest
- Trace code flows and dependencies
- Analyze patterns and architectural decisions

### Phase 3: Synthesis and Delegation
- Synthesize findings into coherent understanding
- Identify remaining research gaps
- Delegate specific research tasks to subagents with precise, signal-dense prompts

## Delegation Strategy

When delegating to subagents:
- **Be Specific**: Give clear, focused research objectives
- **Provide Context**: Include relevant background and constraints
- **Set Boundaries**: Define what the subagent should and shouldn't explore
- **Signal Density**: Pack maximum relevant information into delegation prompts

Example delegation:
"Research the authentication flow in this Express.js application. Focus on: 1) How JWT tokens are generated in auth/jwt.js, 2) How middleware validates tokens in middleware/auth.js, 3) What user data is stored in tokens. Ignore password reset flows for now."

## Tool Usage Guidelines

- **grep**: Perfect for finding specific patterns, function names, or configuration values across the codebase
- **glob**: Excellent for finding files by pattern or exploring directory structures  
- **read**: Essential for deep understanding of specific files and their logic
- **ls**: Useful for understanding directory organization and file relationships
- **bash**: Powerful for complex analysis (word counts, pattern extraction, cross-referencing)
- **webfetch**: Use only when you need external documentation or context that isn't available locally
- **task**: Your primary tool for delegating research to specialized subagents

## Communication Style

Be concise but thorough. Provide clear insights and actionable findings. When you discover something important, explain its significance in the broader context of the codebase. Always maintain focus on research objectives and avoid unnecessary implementation details.

Your goal is to become the definitive source of understanding about any codebase you investigate, efficiently coordinating research efforts and building comprehensive knowledge through systematic analysis and strategic delegation.`,
        tools: {
          write: false,
          edit: false,
          patch: false,
        },
        options: {},
        permission: {
          edit: "deny",
          bash: {
            "*": "allow",
          },
          webfetch: "allow",
        },
        mode: "primary",
      },
    }
    for (const [key, value] of Object.entries(cfg.agent ?? {})) {
      if (value.disable) {
        delete result[key]
        continue
      }
      let item = result[key]
      if (!item)
        item = result[key] = {
          name: key,
          mode: "all",
          permission: defaultPermission,
          options: {},
          tools: {},
        }
      const { model, prompt, tools, description, temperature, top_p, mode, permission, ...extra } = value
      item.options = {
        ...item.options,
        ...extra,
      }
      if (model) item.model = Provider.parseModel(model)
      if (prompt) item.prompt = prompt
      if (tools)
        item.tools = {
          ...item.tools,
          ...tools,
        }
      if (description) item.description = description
      if (temperature != undefined) item.temperature = temperature
      if (top_p != undefined) item.topP = top_p
      if (mode) item.mode = mode

      if (permission ?? cfg.permission) {
        const merged = mergeDeep(cfg.permission ?? {}, permission ?? {})
        if (merged.edit) item.permission.edit = merged.edit
        if (merged.webfetch) item.permission.webfetch = merged.webfetch
        if (merged.bash) {
          if (typeof merged.bash === "string") {
            item.permission.bash = {
              "*": merged.bash,
            }
          }
          // if granular permissions are provided, default to "ask"
          if (typeof merged.bash === "object") {
            item.permission.bash = mergeDeep(
              {
                "*": "ask",
              },
              merged.bash,
            )
          }
        }
      }
    }
    return result
  })

  export async function get(agent: string) {
    return state().then((x) => x[agent])
  }

  export async function list() {
    return state().then((x) => Object.values(x))
  }

  export async function generate(input: { description: string }) {
    const defaultModel = await Provider.defaultModel()
    const model = await Provider.getModel(defaultModel.providerID, defaultModel.modelID)
    const system = SystemPrompt.header(defaultModel.providerID)
    system.push(PROMPT_GENERATE)
    const existing = await list()
    const result = await generateObject({
      temperature: 0.3,
      prompt: [
        ...system.map(
          (item): ModelMessage => ({
            role: "system",
            content: item,
          }),
        ),
        {
          role: "user",
          content: `Create an agent configuration based on this request: \"${input.description}\".\n\nIMPORTANT: The following identifiers already exist and must NOT be used: ${existing.map((i) => i.name).join(", ")}\n  Return ONLY the JSON object, no other text, do not wrap in backticks`,
        },
      ],
      model: model.language,
      schema: z.object({
        identifier: z.string(),
        whenToUse: z.string(),
        systemPrompt: z.string(),
      }),
    })
    return result.object
  }
}
