import z from "zod"
import type { MessageV2 } from "../session/message-v2"

export namespace Tool {
  interface Metadata {
    [key: string]: any
  }

  export type Context<M extends Metadata = Metadata> = {
    sessionID: string
    messageID: string
    agent: string
    abort: AbortSignal
    callID?: string
    extra?: { [key: string]: any }
    metadata(input: { title?: string; metadata?: M }): void
  }
  export interface Info<Parameters extends z.ZodType = z.ZodType, M extends Metadata = Metadata> {
    id: string
    init: () => Promise<{
      description: string
      parameters: Parameters
      execute(
        args: z.infer<Parameters>,
        ctx: Context,
      ): Promise<{
        title: string
        metadata: M
        output: string
        attachments?: MessageV2.FilePart[]
      }>
    }>
  }

  export type InferParameters<T extends Info> = T extends Info<infer P> ? z.infer<P> : never
  export type InferMetadata<T extends Info> = T extends Info<any, infer M> ? M : never

  export function define<Parameters extends z.ZodType, Result extends Metadata>(
    id: string,
    init: Info<Parameters, Result>["init"] | Awaited<ReturnType<Info<Parameters, Result>["init"]>>,
  ): Info<Parameters, Result> {
    return {
      id,
      init: async () => {
        const toolInfo = init instanceof Function ? await init() : init
        const execute = toolInfo.execute
        toolInfo.execute = (args, ctx) => {
          try {
            toolInfo.parameters.parse(args)
          } catch (error) {
            if (error instanceof z.ZodError) {
              const formattedErrors = error.issues
                .map((issue) => {
                  const path = issue.path.length > 0 ? issue.path.join(".") : "root"
                  return `  - ${path}: ${issue.message}`
                })
                .join("\n")

              let errorMessage = `Invalid parameters for tool '${id}':\n${formattedErrors}`

              // Special handling for batch tool
              if (id === "batch") {
                errorMessage += '\n\nExpected payload format:\n  [{"tool": "tool_name", "parameters": {...}}, {...}]'
              } else {
                errorMessage += "\n\nRefer to the tool description for proper usage and required parameters."
              }

              throw new Error(errorMessage)
            }
            throw error
          }
          return execute(args, ctx)
        }
        return toolInfo
      },
    }
  }
}
