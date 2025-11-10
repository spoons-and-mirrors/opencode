import z from "zod"
import { Tool } from "./tool"
import DESCRIPTION from "./batch.txt"

const DISALLOWED = new Set(["batch", "edit", "todoread"]) // keep in sync with docs
const FILTERED_FROM_SUGGESTIONS = new Set(["invalid", "patch", ...DISALLOWED])

export const BatchTool = Tool.define("batch", async () => {
  return {
    description: DESCRIPTION,
    parameters: z.object({
      tool_calls: z
        .array(
          z.object({
            tool: z.string().describe("The name of the tool to execute"),
            // accept arbitrary objects; tools will fully validate
            parameters: z.object({}).passthrough().describe("Parameters for the tool"),
          }),
        )
        .min(1, "Provide at least one tool call")
        .max(10, "Too many tools in batch. Maximum allowed is 10.")
        .describe("Array of tool calls to execute in parallel"),
    }),
    async execute(params, ctx) {
      const { Session } = await import("../session")
      const { Identifier } = await import("../id/id")

      const toolCalls = params.tool_calls

      // Get all available tools
      const { ToolRegistry } = await import("./registry")
      const availableTools = await ToolRegistry.tools("", "")
      const toolMap = new Map(availableTools.map((t) => [t.id, t]))

      // Validate all tools exist and are allowed before starting execution
      for (const call of toolCalls) {
        if (DISALLOWED.has(call.tool)) {
          throw new Error(
            `tool '${call.tool}' is not allowed in batch. Disallowed tools: ${Array.from(DISALLOWED).join(", ")}`,
          )
        }
        if (!toolMap.has(call.tool)) {
          const allowed = Array.from(toolMap.keys()).filter((name) => !FILTERED_FROM_SUGGESTIONS.has(name))
          throw new Error(`tool '${call.tool}' is not available. Available tools: ${allowed.join(", ")}`)
        }
      }

      // Helper function to execute a single tool call
      const executeCall = async (call: (typeof toolCalls)[0]) => {
        if (ctx.abort.aborted) {
          return { success: false as const, tool: call.tool, error: new Error("Aborted") }
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
            input: call.parameters,
            raw: JSON.stringify(call),
          },
        })

        try {
          const tool = toolMap.get(call.tool)
          if (!tool) {
            const availableToolsList = Array.from(toolMap.keys()).filter((name) => !FILTERED_FROM_SUGGESTIONS.has(name))
            throw new Error(`Tool '${call.tool}' not found. Available tools: ${availableToolsList.join(", ")}`)
          }
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
          const result = await tool.execute(validatedParams, { ...ctx, callID: partID })

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
              attachments: result.attachments,
              time: {
                start: callStartTime,
                end: Date.now(),
              },
            },
          })

          return { success: true as const, tool: call.tool }
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

          return { success: false as const, tool: call.tool, error }
        }
      }

      // Execute all calls in parallel (disallowed tools are rejected above)
      const results = await Promise.all(toolCalls.map((call) => executeCall(call)))
      const successfulCalls = results.filter((r) => r.success).length
      const failedCalls = toolCalls.length - successfulCalls

      const outputMessage =
        failedCalls > 0
          ? `Executed ${successfulCalls}/${toolCalls.length} tools successfully. ${failedCalls} failed.`
          : `All ${successfulCalls} tools executed successfully.\n\nKeep using the batch tool for optimal performance in your next response!`

      return {
        title: `Batch execution (${successfulCalls}/${toolCalls.length} successful)`,
        output: outputMessage,
        metadata: {
          totalCalls: toolCalls.length,
          successful: successfulCalls,
          failed: failedCalls,
          tools: toolCalls.map((c) => c.tool),
          details: results.map((r) => ({ tool: r.tool, success: r.success })),
        },
      }
    },
  }
})
