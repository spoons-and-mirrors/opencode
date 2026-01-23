import { Instance } from "../project/instance"
import { Plugin } from "../plugin"
import type { AutocompleteOption } from "@opencode-ai/plugin"

const RESOLVE_TIMEOUT_MS = 500

function timeout(ms: number): Promise<never> {
  return new Promise((_, reject) => setTimeout(() => reject(new Error("Timeout")), ms))
}

export namespace AutocompleteRegistry {
  type Provider = {
    triggers: string[]
    resolve: (trigger: string, query: string) => Promise<AutocompleteOption[]>
  }

  const state = Instance.state(async () => {
    const providers: Provider[] = []
    const plugins = await Plugin.list()

    for (const plugin of plugins) {
      if (plugin.autocomplete) {
        providers.push(plugin.autocomplete)
      }
    }

    return { providers }
  })

  export async function triggers() {
    const { providers } = await state()
    const all = new Set<string>()

    for (const provider of providers) {
      for (const trigger of provider.triggers) {
        all.add(trigger)
      }
    }

    // Sort by length descending so longer triggers match first
    return [...all].sort((a, b) => b.length - a.length)
  }

  export async function resolve(trigger: string, query: string) {
    const { providers } = await state()
    const matching = providers.filter((p) => p.triggers.includes(trigger))

    const results = await Promise.allSettled(
      matching.map((p) => Promise.race([p.resolve(trigger, query), timeout(RESOLVE_TIMEOUT_MS)])),
    )

    return results.flatMap((r) => (r.status === "fulfilled" && Array.isArray(r.value) ? r.value : []))
  }
}
