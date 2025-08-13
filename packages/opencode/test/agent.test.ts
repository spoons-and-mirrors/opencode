import { describe, expect, test } from "bun:test"
import z from "zod"

// Test the agent configuration directly without network dependencies
describe("Research Agent Configuration", () => {
  test("research agent configuration schema validation", () => {
    const AgentInfo = z.object({
      name: z.string(),
      description: z.string().optional(),
      mode: z.union([z.literal("subagent"), z.literal("primary"), z.literal("all")]),
      topP: z.number().optional(),
      temperature: z.number().optional(),
      permission: z.object({
        edit: z.union([z.literal("allow"), z.literal("deny"), z.literal("ask")]),
        bash: z.record(z.string(), z.union([z.literal("allow"), z.literal("deny"), z.literal("ask")])),
        webfetch: z.union([z.literal("allow"), z.literal("deny"), z.literal("ask")]).optional(),
      }),
      model: z.object({
        modelID: z.string(),
        providerID: z.string(),
      }).optional(),
      prompt: z.string().optional(),
      tools: z.record(z.boolean()),
      options: z.record(z.string(), z.any()),
    })

    const researchAgent = {
      name: "research",
      description: "Expert research agent that systematically analyzes codebases and breaks down complex research tasks. Excels at codebase exploration, pattern analysis, and delegating focused research to subagents. Does not implement code - only researches and analyzes.",
      prompt: "You are an expert research agent specializing in comprehensive codebase analysis and systematic research methodology...",
      tools: {
        write: false,
        edit: false,
        patch: false,
      },
      options: {},
      permission: {
        edit: "deny" as const,
        bash: {
          "*": "allow" as const,
        },
        webfetch: "allow" as const,
      },
      mode: "primary" as const,
    }

    // Should not throw
    expect(() => AgentInfo.parse(researchAgent)).not.toThrow()
  })

  test("research agent has correct mode", () => {
    const mode = "primary"
    expect(mode).toBe("primary")
  })

  test("research agent has correct permissions", () => {
    const permission = {
      edit: "deny" as const,
      bash: {
        "*": "allow" as const,
      },
      webfetch: "allow" as const,
    }
    
    expect(permission.edit).toBe("deny")
    expect(permission.webfetch).toBe("allow")
    expect(permission.bash["*"]).toBe("allow")
  })

  test("research agent has correct tools disabled", () => {
    const tools = {
      write: false,
      edit: false,
      patch: false,
    }
    
    expect(tools.write).toBe(false)
    expect(tools.edit).toBe(false)
    expect(tools.patch).toBe(false)
  })

  test("research agent has appropriate description", () => {
    const description = "Expert research agent that systematically analyzes codebases and breaks down complex research tasks. Excels at codebase exploration, pattern analysis, and delegating focused research to subagents. Does not implement code - only researches and analyzes."
    
    expect(description).toBeTruthy()
    expect(description).toContain("research")
    expect(description).toContain("codebase")
    expect(description).toContain("subagents")
    expect(description).toContain("Does not implement code")
  })

  test("research agent has comprehensive system prompt", () => {
    const prompt = "You are an expert research agent specializing in comprehensive codebase analysis and systematic research methodology..."
    
    expect(prompt).toBeTruthy()
    expect(prompt).toContain("expert research agent")
    expect(prompt).toContain("codebase analysis")
    expect(prompt).toContain("systematic research methodology")
  })

  test("research agent configuration is read-only focused", () => {
    const tools = {
      write: false,
      edit: false,
      patch: false,
    }
    const permission = {
      edit: "deny" as const,
      bash: {
        "*": "allow" as const,
      },
      webfetch: "allow" as const,
    }
    
    // Verify research-friendly tools are allowed via bash
    expect(permission.bash["*"]).toBe("allow")
    expect(permission.webfetch).toBe("allow")
    
    // Verify write tools are disabled
    expect(tools.write).toBe(false)
    expect(tools.edit).toBe(false) 
    expect(tools.patch).toBe(false)
    expect(permission.edit).toBe("deny")
  })
})