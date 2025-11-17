import type { Plugin } from "@opencode-ai/plugin"
import { Session } from "../../packages/opencode/src/session"

export const PrunePlugin: Plugin = async (ctx) => {
  return {
    "plugin.command": {
      prune: {
        description: "Clear all tool outputs from the session",
        aliases: ["clear-output", "clear-tools"],
        sessionOnly: true,
        async execute({ sessionID, client }) {
          if (!sessionID) return

          // Get all messages in the session
          const msgs = await Session.messages({ sessionID })
          let prunedCount = 0

          // Iterate through all messages and their parts
          for (const msg of msgs) {
            if (msg.info.role !== "assistant") continue

            for (const part of msg.parts) {
              if (part.type !== "tool") continue
              if (part.state.status !== "completed") continue
              if (part.state.time.compacted) continue

              // Mark as compacted by setting the timestamp
              part.state.time.compacted = Date.now()
              await Session.updatePart(part)
              prunedCount++
            }
          }

          // Show success toast
          await client.tui.showToast({
            body: {
              message: `Pruned ${prunedCount} tool outputs!`,
              variant: "success",
            },
          })
        },
      },
    },
  }
}
