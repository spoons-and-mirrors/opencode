import type { Plugin } from "@opencode-ai/plugin"

export const WorkflowPlugin: Plugin = async (ctx) => {
  return {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    "plugin.command": {
      workflow: {
        description: "Run reduce, prune, implement, and commit in order",
        sessionOnly: true,
        async execute({ sessionID, client }: { sessionID?: string; client: any }) {
          if (!sessionID) return

          await client.session.command({
            path: { id: sessionID },
            body: {
              agent: "build",
              command: "reduce",
              arguments: "",
            },
          })

          await client.session.command({
            path: { id: sessionID },
            body: {
              agent: "build",
              command: "prune",
              arguments: "",
            },
          })

          await client.session.command({
            path: { id: sessionID },
            body: {
              agent: "build",
              command: "implement",
              arguments: "",
            },
          })

          await client.session.command({
            path: { id: sessionID },
            body: {
              agent: "build",
              command: "commit",
              arguments: "",
            },
          })

          await client.tui.showToast({
            body: {
              message: "Workflow complete: reduce, prune, implement, commit",
              variant: "success",
            },
          })
        },
      },
    },
  } as unknown as import("@opencode-ai/plugin").Hooks
}
