import z from "zod/v4"
import { Tool } from "./tool"
import DESCRIPTION from "./batch.txt"

export const BatchTool = Tool.define("batch", async () => {
  return {
    description: DESCRIPTION,
    parameters: z.object({
      tool_calls: z
        .array(
          z.object({
            tool: z.string().describe("The name of the tool to execute"),
            parameters: z.record(z.string(), z.any()).describe("Parameters for the tool"),
          }),
        )
        .describe("Array of tool calls to execute in parallel"),
    }),
    async execute(params, ctx) {
      const { Session } = await import("../session")
      const { Identifier } = await import("../id/id")

      const toolCalls = params.tool_calls

      if (toolCalls.length === 0) {
        return {
          title: "No tools to execute",
          output: "",
          metadata: {},
        }
      }

      // Get all available tools
      const { ToolRegistry } = await import("./registry")
      const availableTools = await ToolRegistry.tools("", "")
      const toolMap = new Map(availableTools.map((t: any) => [t.id, t]))

      // Validate all tools exist and have proper schema before starting execution
      for (const call of toolCalls) {
        if (!call.tool || !call.parameters) {
          throw new Error(
            `malformed schema: each tool call must have "tool" and "parameters" fields. Retry with proper payload formatting: [{"tool": "tool_name", "parameters": {...}}]`,
          )
        }
        if (!toolMap.has(call.tool)) {
          const availableTools = Array.from(toolMap.keys()).filter(
            (name) => !["invalid", "batch", "todoread", "patch"].includes(name),
          )
          throw new Error(`tool '${call.tool}' is not available. Available tools: ${availableTools.join(", ")}`)
        }
      }

      // Helper function to execute a single tool call
      const executeCall = async (call: (typeof toolCalls)[0]) => {
        if (ctx.abort.aborted) {
          return { success: false, error: new Error("Aborted") }
        }

        const callStartTime = Date.now()
        const partID = Identifier.ascending("part")

        // Create pending tool part
        await Session.updatePart({
          id: partID,
          messageID: ctx.messageID,
          sessionID: ctx.sessionID,
          type: "tool",
          tool: call.tool,
          callID: partID,
          state: {
            status: "pending",
          },
        })

        try {
          const tool = toolMap.get(call.tool) as any
          const validatedParams = tool.parameters.parse(call.parameters)

          // Update to running state
          await Session.updatePart({
            id: partID,
            messageID: ctx.messageID,
            sessionID: ctx.sessionID,
            type: "tool",
            tool: call.tool,
            callID: partID,
            state: {
              status: "running",
              input: call.parameters,
              time: {
                start: callStartTime,
              },
            },
          })

          // Execute the tool
          const result = await tool.execute(validatedParams, ctx)

          // Update to completed state
          await Session.updatePart({
            id: partID,
            messageID: ctx.messageID,
            sessionID: ctx.sessionID,
            type: "tool",
            tool: call.tool,
            callID: partID,
            state: {
              status: "completed",
              input: call.parameters,
              output: result.output,
              title: result.title,
              metadata: result.metadata,
              time: {
                start: callStartTime,
                end: Date.now(),
              },
            },
          })

          return { success: true }
        } catch (error) {
          // Update to error state
          await Session.updatePart({
            id: partID,
            messageID: ctx.messageID,
            sessionID: ctx.sessionID,
            type: "tool",
            tool: call.tool,
            callID: partID,
            state: {
              status: "error",
              input: call.parameters,
              error: error instanceof Error ? error.message : String(error),
              time: {
                start: callStartTime,
                end: Date.now(),
              },
            },
          })

          return { success: false, error }
        }
      }

      // Group edits by file to execute sequentially per file
      const fileGroups = new Map<string, typeof toolCalls>()
      const nonEditCalls: typeof toolCalls = []

      for (const call of toolCalls) {
        if (call.tool === "edit" && call.parameters["filePath"]) {
          const filePath = call.parameters["filePath"] as string
          if (!fileGroups.has(filePath)) {
            fileGroups.set(filePath, [])
          }
          fileGroups.get(filePath)!.push(call)
        } else {
          nonEditCalls.push(call)
        }
      }

      // Execute all non-edit calls in parallel
      const promises: Promise<{ success: boolean; error?: any } | { success: boolean; error?: any }[]>[] =
        nonEditCalls.map((call) => executeCall(call))

      // Execute edits for each file sequentially, but different files in parallel
      for (const [, calls] of fileGroups) {
        promises.push(
          (async () => {
            const results: { success: boolean; error?: any }[] = []
            for (const call of calls) {
              if (ctx.abort.aborted) {
                break
              }
              results.push(await executeCall(call))
            }
            return results
          })(),
        )
      }

      // Wait for all tools to complete and flatten results
      const results = (await Promise.all(promises)).flat()
      const successfulCalls = results.filter((r) => r.success).length

      return {
        title: `Batch execution (${successfulCalls}/${toolCalls.length} successful)`,
        output: "Keep using the batch tool for optimal performance in your next response!",
        metadata: {},
      }
    },
  }
})
