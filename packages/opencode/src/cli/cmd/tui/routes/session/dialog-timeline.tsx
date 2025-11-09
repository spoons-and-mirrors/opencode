import { createMemo, createSignal, onMount } from "solid-js"
import { useSync } from "@tui/context/sync"
import { DialogSelect, type DialogSelectOption } from "@tui/ui/dialog-select"
import type { TextPart, Part, ReasoningPart, ToolPart } from "@opencode-ai/sdk"
import { Locale } from "@/util/locale"
import { DialogMessage } from "./dialog-message"
import { useDialog } from "../../ui/dialog"
import { Keybind } from "@/util/keybind"

type TimelineValue = {
  messageID: string
  partID?: string
  partType?: string
  isAtomic: boolean
}

export function DialogTimeline(props: {
  sessionID: string
  onMove: (messageID: string, partID?: string, partType?: string) => void
}) {
  const sync = useSync()
  const dialog = useDialog()
  const [viewMode, setViewMode] = createSignal<"normal" | "atomic">("normal")

  onMount(() => {
    dialog.setSize("large")
  })

  const normalOptions = createMemo((): DialogSelectOption<TimelineValue>[] => {
    const messages = sync.data.message[props.sessionID] ?? []
    const result = [] as DialogSelectOption<TimelineValue>[]
    for (const message of messages) {
      if (message.role !== "user") continue
      const part = (sync.data.part[message.id] ?? []).find((x) => x.type === "text" && !x.synthetic) as TextPart
      if (!part) continue
      result.push({
        title: part.text.replace(/\n/g, " "),
        value: { messageID: message.id, isAtomic: false },
        footer: Locale.time(message.time.created),
        onSelect: (dialog) => {
          dialog.replace(() => <DialogMessage messageID={message.id} sessionID={props.sessionID} />)
        },
      })
    }
    return result
  })

  const atomicOptions = createMemo((): DialogSelectOption<TimelineValue>[] => {
    const messages = sync.data.message[props.sessionID] ?? []
    const result = [] as DialogSelectOption<TimelineValue>[]

    messages.forEach((message) => {
      const parts = sync.data.part[message.id] ?? []

      parts.forEach((part) => {
        // Filter valid parts
        if (!isPartValid(part)) return

        const content = extractPartPreview(part)
        const partType = getPartType(part, message.role)

        // Create display title - single line with unicode dot prefix
        let displayTitle = "• " + content.replace(/\n/g, " ")
        if (partType === "tool") {
          const toolPart = part as ToolPart
          displayTitle = "• " + toolPart.tool
        }

        result.push({
          title: displayTitle,
          value: { messageID: message.id, partID: part.id, partType: part.type, isAtomic: true },
          footer: message.time.created ? Locale.time(message.time.created) : undefined,
          onSelect: (dialog) => {
            dialog.replace(() => <DialogMessage messageID={message.id} sessionID={props.sessionID} />)
          },
        })
      })
    })

    return result
  })

  function isPartValid(part: Part): boolean {
    switch (part.type) {
      case "text": {
        const textPart = part as TextPart
        return !textPart.synthetic && textPart.text.trim() !== ""
      }
      case "reasoning": {
        const reasoningPart = part as ReasoningPart
        return reasoningPart.text.trim() !== ""
      }
      case "tool":
        return true
      default:
        return false
    }
  }

  function extractPartPreview(part: Part): string {
    switch (part.type) {
      case "text": {
        const textPart = part as TextPart
        return textPart.text.trim()
      }
      case "reasoning": {
        const reasoningPart = part as ReasoningPart
        return reasoningPart.text.trim()
      }
      case "tool": {
        const toolPart = part as ToolPart
        return toolPart.tool || "Tool call"
      }
      default:
        return "Unknown part"
    }
  }

  function getPartType(part: Part, messageRole: string): "user_text" | "assistant_text" | "reasoning" | "tool" {
    if (part.type === "text") {
      return messageRole === "user" ? "user_text" : "assistant_text"
    } else if (part.type === "reasoning") {
      return "reasoning"
    } else if (part.type === "tool") {
      return "tool"
    }
    return "assistant_text"
  }

  return (
    <DialogSelect
      onMove={(option) => {
        props.onMove(option.value.messageID, option.value.partID, option.value.partType)
      }}
      title={viewMode() === "normal" ? "Timeline" : "Timeline (Atomic)"}
      options={viewMode() === "normal" ? normalOptions() : atomicOptions()}
      keybind={[
        {
          keybind: Keybind.parse("a")[0],
          title: viewMode() === "normal" ? "atomic" : "normal",
          onTrigger: () => {
            setViewMode((mode) => (mode === "normal" ? "atomic" : "normal"))
          },
        },
      ]}
    />
  )
}
