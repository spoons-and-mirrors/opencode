import z from "zod"
import { Tool } from "./tool"
import DESCRIPTION from "./batch.txt"

const DISALLOWED = new Set(["batch", "edit", "todoread"])
const FILTERED_FROM_SUGGESTIONS = new Set(["invalid", "patch", ...DISALLOWED])

export const BatchTool = Tool.define("batch", async () => {
  return {
    description: DESCRIPTION,
    parameters: z.object({
      tool_calls: z
        .array(
          z.object({
            tool: z.string().describe("The name of the tool to execute"),
            parameters: z.object({}).loose().describe("Parameters for the tool"),
          }),
        )
        .min(1, "Provide at least one tool call")
        .describe("Array of tool calls to execute in parallel"),
    }),
    formatValidationError(error) {
      const formattedErrors = error.issues
        .map((issue) => {
          const path = issue.path.length > 0 ? issue.path.join(".") : "root"
          return `  - ${path}: ${issue.message}`
        })
        .join("\n")

      return `Invalid parameters for tool 'batch':\n${formattedErrors}\n\nExpected payload format:\n  [{"tool": "tool_name", "parameters": {...}}, {...}]`
    },
    async execute(params, ctx) {
      const { Session } = await import("../session")
      const { Identifier } = await import("../id/id")

      const toolCalls = params.tool_calls.slice(0, 10)
      const discardedCalls = params.tool_calls.slice(10)

      const { ToolRegistry } = await import("./registry")
      const availableTools = await ToolRegistry.tools("", "")
      const toolMap = new Map(availableTools.map((t) => [t.id, t]))

      const uniqueCalls: typeof toolCalls = []
      const seenCalls = new Set<string>()
      for (const call of toolCalls) {
        const key = JSON.stringify({ tool: call.tool, parameters: call.parameters })
        if (seenCalls.has(key)) continue
        seenCalls.add(key)
        uniqueCalls.push(call)
      }

      const executeCall = async (call: (typeof toolCalls)[0]) => {
        const callStartTime = Date.now()
        const partID = Identifier.ascending("part")
        let done = false
        const pending: Promise<unknown>[] = []

        try {
          if (DISALLOWED.has(call.tool)) {
            throw new Error(
              `Tool '${call.tool}' is not allowed in batch. Disallowed tools: ${Array.from(DISALLOWED).join(", ")}`,
            )
          }

          const tool = toolMap.get(call.tool)
          if (!tool) {
            const availableToolsList = Array.from(toolMap.keys()).filter((name) => !FILTERED_FROM_SUGGESTIONS.has(name))
            throw new Error(`Tool '${call.tool}' not found. Available tools: ${availableToolsList.join(", ")}`)
          }

          const validatedParams = tool.parameters.parse(call.parameters)

          if (!done) {
            const p = Session.updatePart({
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
            pending.push(p)
            p.finally(() => {
              const i = pending.indexOf(p)
              if (i !== -1) pending.splice(i, 1)
            })
          }

          const result = await tool.execute(validatedParams, {
            ...ctx,
            callID: partID,
            metadata: (input) => {
              if (done) return
              const p = Session.updatePart({
                id: partID,

                messageID: ctx.messageID,
                sessionID: ctx.sessionID,
                type: "tool",
                tool: call.tool,
                callID: partID,
                state: {
                  status: "running",
                  input: call.parameters,
                  title: input.title,
                  metadata: input.metadata,
                  time: {
                    start: callStartTime,
                  },
                },
              })
              pending.push(p)
              p.finally(() => {
                const i = pending.indexOf(p)
                if (i !== -1) pending.splice(i, 1)
              })
            },
          })

          done = true
          await Promise.all(pending)

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

          return { success: true as const, tool: call.tool, result }
        } catch (error) {
          done = true
          await Promise.all(pending)

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

      const results = await Promise.all(uniqueCalls.map((call) => executeCall(call)))

      // Add discarded calls as errors
      const now = Date.now()
      for (const call of discardedCalls) {
        const partID = Identifier.ascending("part")
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
            error: "Maximum of 10 tools allowed in batch",
            time: { start: now, end: now },
          },
        })
        results.push({
          success: false as const,
          tool: call.tool,
          error: new Error("Maximum of 10 tools allowed in batch"),
        })
      }

      const successfulCalls = results.filter((r) => r.success).length
      const failedCalls = results.length - successfulCalls

      const outputMessage =
        failedCalls > 0
          ? `Executed ${successfulCalls}/${results.length} tools successfully. ${failedCalls} failed.`
          : `All ${successfulCalls} tools executed successfully.\n\nKeep using the batch tool for optimal performance in your next response!`

      return {
        title: `Batch execution (${successfulCalls}/${results.length} successful)`,
        output: outputMessage,
        metadata: {
          totalCalls: results.length,
          successful: successfulCalls,
          failed: failedCalls,
          tools: params.tool_calls.map((c) => c.tool),
          details: results.map((r) => ({ tool: r.tool, success: r.success })),
        },
      }
    },
  }
})
