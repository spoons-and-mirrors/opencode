import type { Plugin } from "@opencode-ai/plugin"

const plugin: Plugin = async () => {
  const snippets = [
    { name: "todo", value: "TODO: ", description: "TODO comment" },
    { name: "fixme", value: "FIXME: ", description: "FIXME comment" },
    { name: "note", value: "NOTE: ", description: "NOTE comment" },
    { name: "hack", value: "HACK: ", description: "HACK comment" },
    { name: "question", value: "QUESTION: ", description: "Question comment" },
  ]

  return {
    autocomplete: {
      triggers: ["#"],
      resolve: async (trigger, query) => {
        return snippets
          .filter((s) => s.name.toLowerCase().includes(query.toLowerCase()))
          .map((s) => ({
            display: `#${s.name}`,
            value: s.value,
            description: s.description,
          }))
      },
    },
  }
}

export default plugin
