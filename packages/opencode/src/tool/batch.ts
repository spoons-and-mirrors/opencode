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

      // Filter out disabled tools and validate all tools exist
      const disabledTools = ["batch", "todoread", "patch", "edit"]
      const filteredCalls = toolCalls.filter((call) => {
        if (!call.tool || !call.parameters) {
          throw new Error(
            `malformed schema: each tool call must have "tool" and "parameters" fields. Retry with proper payload formatting: [{"tool": "tool_name", "parameters": {...}}]`,
          )
        }
        if (!toolMap.has(call.tool)) {
          const availableTools = Array.from(toolMap.keys()).filter((name) => !disabledTools.includes(name))
          throw new Error(`tool '${call.tool}' is not available. Available tools: ${availableTools.join(", ")}`)
        }
        return !disabledTools.includes(call.tool)
      })

      if (filteredCalls.length === 0 && toolCalls.length > 0) {
        return {
          title: "No valid tools to execute",
          output: "All provided tools are disabled in batch calls. Use them directly instead.",
          metadata: {},
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

      // Execute all filtered tool calls in parallel
      const promises = filteredCalls.map((call) => executeCall(call))

      // Wait for all tools to complete and flatten results
      const results = (await Promise.all(promises)).flat()
      const successfulCalls = results.filter((r) => r.success).length

      return {
        title: `Batch execution (${successfulCalls}/${filteredCalls.length} successful)`,
        output:
          filteredCalls.length < toolCalls.length
            ? `Executed ${filteredCalls.length} valid tools (skipped ${toolCalls.length - filteredCalls.length} disabled tools). You were rewarded for using the batch tool. ALWAYS Keep using the batch tool!!`
            : "You were rewarded for using the batch tool. ALWAYS Keep using the batch tool!!",
        metadata: {},
      }
    },
  }
})
