import { experimental_createMCPClient, type Tool } from "ai"
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js"
import { SSEClientTransport } from "@modelcontextprotocol/sdk/client/sse.js"
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js"
import { Config } from "../config/config"
import { Log } from "../util/log"
import { NamedError } from "../util/error"
import { z } from "zod"
import { Bus } from "../bus"
import { Session } from "../session"
import { MCP } from "../mcp"

export namespace AgentMCP {
  const log = Log.create({ service: "agent-mcp" })

  export const Failed = NamedError.create(
    "AgentMCPFailed",
    z.object({
      name: z.string(),
      agent: z.string(),
    }),
  )

  // Cache agent-specific clients (only for clients not available globally)
  const agentClients: Map<string, Map<string, Awaited<ReturnType<typeof experimental_createMCPClient>>>> = new Map()

  export async function tools(agentName: string, mcpConfig: Record<string, Config.Mcp>): Promise<Record<string, Tool>> {
    const result: Record<string, Tool> = {}

    // First, check global MCP clients for reuse
    const globalClients = await MCP.clients()

    for (const [serverName, mcp] of Object.entries(mcpConfig)) {
      if (mcp.enabled === false) {
        log.info("agent mcp server disabled", { server: serverName, agent: agentName })
        continue
      }

      let client: Awaited<ReturnType<typeof experimental_createMCPClient>> | undefined

      // Check if this server already exists globally
      if (globalClients[serverName]) {
        log.info("reusing global mcp client", { server: serverName, agent: agentName })
        client = globalClients[serverName]
      } else {
        // Launch agent-specific client if not available globally
        client = await getOrCreateAgentClient(agentName, serverName, mcp)
      }

      if (client) {
        try {
          const serverTools = await client.tools()
          for (const [toolName, tool] of Object.entries(serverTools)) {
            // Use same naming as global MCP: serverName_toolName
            const sanitizedServerName = serverName.replace(/\s+/g, "_")
            const toolKey = `${sanitizedServerName}_${toolName}`
            result[toolKey] = tool
          }
        } catch (error) {
          log.error("failed to get mcp tools", {
            agent: agentName,
            server: serverName,
            error,
          })
        }
      }
    }

    return result
  }

  async function getOrCreateAgentClient(agentName: string, serverName: string, mcp: Config.Mcp) {
    if (!agentClients.has(agentName)) {
      agentClients.set(agentName, new Map())
    }

    const clients = agentClients.get(agentName)!

    if (clients.has(serverName)) {
      return clients.get(serverName)!
    }

    log.info("initializing agent-specific mcp client", { server: serverName, agent: agentName, type: mcp.type })

    try {
      let client: Awaited<ReturnType<typeof experimental_createMCPClient>> | undefined

      if (mcp.type === "remote") {
        const transports = [
          new StreamableHTTPClientTransport(new URL(mcp.url), {
            requestInit: {
              headers: mcp.headers,
            },
          }),
          new SSEClientTransport(new URL(mcp.url), {
            requestInit: {
              headers: mcp.headers,
            },
          }),
        ]

        for (const transport of transports) {
          client = await experimental_createMCPClient({
            name: serverName, // Use same name as global MCP
            transport,
          }).catch(() => undefined)
          if (client) break
        }
      }

      if (mcp.type === "local") {
        const [cmd, ...args] = mcp.command
        client = await experimental_createMCPClient({
          name: serverName, // Use same name as global MCP
          transport: new StdioClientTransport({
            stderr: "ignore",
            command: cmd,
            args,
            env: {
              ...process.env,
              ...(cmd === "opencode" ? { BUN_BE_BUN: "1" } : {}),
              ...mcp.environment,
            },
          }),
        }).catch(() => undefined)
      }

      if (client) {
        clients.set(serverName, client)
        log.info("agent mcp client initialized", { server: serverName, agent: agentName })
        return client
      } else {
        log.error("failed to initialize agent mcp client", { server: serverName, agent: agentName })
        Bus.publish(Session.Event.Error, {
          error: {
            name: "UnknownError",
            data: {
              message: `Agent MCP server ${serverName} (agent: ${agentName}) failed to start`,
            },
          },
        })
      }
    } catch (error) {
      log.error("agent mcp initialization error", { server: serverName, agent: agentName, error })
    }

    return undefined
  }

  export async function cleanup(agentName: string) {
    const clients = agentClients.get(agentName)
    if (!clients) return

    for (const [serverName, client] of clients.entries()) {
      try {
        client.close()
        log.info("agent mcp client closed", { agent: agentName, server: serverName })
      } catch (error) {
        log.error("error closing agent mcp client", { agent: agentName, server: serverName, error })
      }
    }

    agentClients.delete(agentName)
  }

  export async function cleanupAll() {
    for (const agentName of agentClients.keys()) {
      await cleanup(agentName)
    }
  }
}
