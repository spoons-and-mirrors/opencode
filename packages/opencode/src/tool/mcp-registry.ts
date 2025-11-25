import z from "zod"
import { Tool } from "./tool"
import { MCP } from "../mcp"
import DESCRIPTION from "./mcp-registry.txt"

// Track which MCP tools are activated per session (after being described)
// Key: sessionID, Value: Map of toolName -> remaining turns
const activatedTools = new Map<string, Map<string, number>>()
const ACTIVATION_TURNS = 3

export function getActivatedTools(sessionID: string): Set<string> {
  const session = activatedTools.get(sessionID)
  if (!session) return new Set()
  return new Set(session.keys())
}

export function decrementActivatedTools(sessionID: string) {
  const session = activatedTools.get(sessionID)
  if (!session) return
  for (const [tool, turns] of session) {
    if (turns <= 1) {
      session.delete(tool)
    } else {
      session.set(tool, turns - 1)
    }
  }
  if (session.size === 0) {
    activatedTools.delete(sessionID)
  }
}

function activateTool(sessionID: string, toolName: string) {
  let session = activatedTools.get(sessionID)
  if (!session) {
    session = new Map()
    activatedTools.set(sessionID, session)
  }
  session.set(toolName, ACTIVATION_TURNS)
}

export const MCPRegistryTool = Tool.define("mcp_registry", async () => {
  const mcpTools = await MCP.tools()
  const names = Object.keys(mcpTools).map((n) => n.replaceAll("-", "_"))
  const description = `${DESCRIPTION}\n\nAvailable MCP tools: [${names.join(", ")}]`

  return {
    description,
    parameters: z.object({
      activate: z.array(z.string()).describe("Array of exact tool names to activate."),
    }),
    async execute(params, ctx) {
      const currentTools = await MCP.tools()
      const results: string[] = []
      const activated: string[] = []

      for (const name of params.activate) {
        const tool = currentTools[name]
        if (!tool) {
          results.push(`Tool "${name}" not found`)
          continue
        }
        activateTool(ctx.sessionID, name)
        activated.push(name)
      }

      if (activated.length > 0) {
        results.push(
          `Activated ${activated.length} tool(s): ${activated.join(", ")}`,
          "These tools are now temporarily available in your environment.",
        )
      }

      return {
        title: activated.length ? `Activated ${activated.length} tools` : "No tools activated",
        output: results.join("\n"),
        metadata: {},
      }
    },
  }
})
