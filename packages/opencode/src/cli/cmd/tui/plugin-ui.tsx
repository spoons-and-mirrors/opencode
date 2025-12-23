import { createSignal, ErrorBoundary, For, Show } from "solid-js"
import { Dynamic } from "solid-js/web"
import { useTheme } from "@tui/context/theme"
import { useKeyboard } from "@opentui/solid"
import type {
  PluginUINode,
  PluginUIComponent,
  TextNode,
  BoxNode,
  ChecklistNode,
  ButtonNode,
  CollapsibleNode,
} from "@opencode-ai/plugin"
import { Log } from "@/util/log"

const log = Log.create({ service: "plugin-ui" })

// Signal to trigger re-renders when dismissed state changes
const [dismissed, setDismissed] = createSignal(new Set<string>())

// Global signal for plugin modals (used when DialogProvider context isn't available)
export const [pluginModalRequest, setPluginModalRequest] = createSignal<{
  node: PluginUINode
  metadata: Record<string, any>
} | null>(null)

// Plugin UI Registry - stores templates fetched from plugins
export const PluginRegistry = {
  templates: new Map<string, { node: PluginUINode; replaceInput: boolean }>(),
  baseUrl: "",
  register(name: string, template: PluginUINode, replaceInput?: boolean) {
    this.templates.set(name, { node: template, replaceInput: replaceInput ?? false })
  },
  get(name: string) {
    const entry = this.templates.get(name)
    if (!entry) log.warn("template not found", { name })
    return entry?.node
  },
  shouldReplaceInput(name: string) {
    return this.templates.get(name)?.replaceInput ?? false
  },
  dismiss(componentKey: string) {
    setDismissed((prev) => {
      const next = new Set(prev)
      next.add(componentKey)
      return next
    })
  },
  isDismissed(componentKey: string) {
    return dismissed().has(componentKey)
  },
  async emit(component: string, event: string, data: Record<string, any>) {
    try {
      await fetch(`${this.baseUrl}/plugins/ui/event`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ component, event, data }),
      })
    } catch (e) {
      log.error("emit failed", { error: e })
    }
  },

  async load(baseUrl: string) {
    this.baseUrl = baseUrl
    try {
      const res = await fetch(`${baseUrl}/plugins/ui`)
      if (!res.ok) throw new Error(`status ${res.status}`)
      const components: PluginUIComponent[] = await res.json()
      log.info("loaded", { count: components.length })
      for (const c of components) this.register(c.name, c.template, c.replaceInput)
    } catch (e) {
      log.error("load failed", { error: e })
    }
  },
}

// Interpolate {{key}} placeholders with metadata values
function interpolate(text: string, metadata: Record<string, any>): string {
  return text.replace(/\{\{(\w+)\}\}/g, (_, key) => String(metadata[key] ?? ""))
}

// Helper to resolve theme colors - returns both getColor and theme
function useColors() {
  const { theme } = useTheme()
  const getColor = (color?: string) => {
    if (!color) return undefined
    return (theme as any)[color] ?? color
  }
  return { getColor, theme }
}

type RendererProps<T> = { node: T; metadata: Record<string, any> }

function PluginText(props: RendererProps<TextNode>) {
  const { getColor, theme } = useColors()
  const content = () => interpolate(props.node.content, props.metadata)
  return (
    <text fg={getColor(props.node.fg) ?? theme.text}>{props.node.bold ? <strong>{content()}</strong> : content()}</text>
  )
}

function PluginBox(props: RendererProps<BoxNode>) {
  const { getColor, theme } = useColors()
  const n = props.node
  const hasBorder = n.border !== undefined
  return (
    <box
      flexDirection={n.direction ?? "row"}
      gap={n.gap ?? 0}
      backgroundColor={getColor(n.bg)}
      border={n.border}
      borderStyle={hasBorder ? n.borderStyle : undefined}
      borderColor={hasBorder ? (getColor(n.borderColor) ?? theme.border) : undefined}
      marginTop={n.marginTop ?? n.marginY}
      marginBottom={n.marginBottom ?? n.marginY}
      marginLeft={n.marginLeft ?? n.marginX}
      marginRight={n.marginRight ?? n.marginX}
      paddingTop={n.paddingTop ?? n.paddingY}
      paddingBottom={n.paddingBottom ?? n.paddingY}
      paddingLeft={n.paddingLeft ?? n.paddingX}
      paddingRight={n.paddingRight ?? n.paddingX}
      justifyContent={n.justifyContent}
      alignSelf={n.alignSelf}
      minWidth={n.minWidth}
    >
      <For each={n.children}>{(child) => <PluginUIRenderer node={child} metadata={props.metadata} />}</For>
    </box>
  )
}

function PluginChecklist(props: RendererProps<ChecklistNode>) {
  const { getColor, theme } = useColors()

  const rawItems = () =>
    typeof props.node.items === "string"
      ? ((props.metadata[props.node.items] as Array<{ id: string; label: string; checked?: boolean }>) ?? [])
      : (props.node.items as Array<{ id: string; label: string; checked?: boolean }>)

  const [items, setItems] = createSignal(rawItems().map((i) => ({ ...i, checked: i.checked ?? false })))
  const [focusedIndex, setFocusedIndex] = createSignal(0)

  const toggleItem = (index: number) => {
    const item = items()[index]
    if (!item) return
    const updatedItems = items().map((i, idx) => (idx === index ? { ...i, checked: !i.checked } : i))
    setItems(updatedItems)
    if (props.node.onToggle && props.metadata._component) {
      PluginRegistry.emit(props.metadata._component, props.node.onToggle, {
        id: item.id,
        checked: !item.checked,
        items: updatedItems,
      })
    }
  }

  useKeyboard((evt) => {
    if (evt.name === "up" || evt.name === "k") {
      setFocusedIndex((i) => Math.max(0, i - 1))
      return true
    }
    if (evt.name === "down" || evt.name === "j") {
      setFocusedIndex((i) => Math.min(items().length - 1, i + 1))
      return true
    }
    if (evt.name === "space") {
      toggleItem(focusedIndex())
      return true
    }
    return false
  })

  return (
    <box flexDirection="column" gap={0}>
      <For each={items()}>
        {(item, index) => (
          <box
            flexDirection="row"
            justifyContent="space-between"
            backgroundColor={item.checked ? (getColor(props.node.bgChecked) ?? theme.backgroundElement) : undefined}
            paddingLeft={1}
            paddingRight={1}
            onMouseOver={() => setFocusedIndex(index())}
            onMouseDown={() => toggleItem(index())}
          >
            <box flexDirection="row">
              <text
                content={item.checked ? "● " : "○ "}
                fg={getColor(props.node.borderColorChecked) ?? theme.warning}
              />
              <text
                content={item.label}
                fg={
                  item.checked
                    ? (getColor(props.node.fgChecked) ?? theme.text)
                    : (getColor(props.node.fg) ?? theme.textMuted)
                }
              />
            </box>
            <text content={focusedIndex() === index() ? "◀" : ""} fg={theme.warning} />
          </box>
        )}
      </For>
    </box>
  )
}

function PluginButton(props: RendererProps<ButtonNode>) {
  const { getColor, theme } = useColors()
  const [hovered, setHovered] = createSignal(false)
  const [clicked, setClicked] = createSignal(false)

  const handleActivate = () => {
    if (clicked()) return
    setClicked(true)
    if (props.node.onModal) {
      setPluginModalRequest({ node: props.node.onModal, metadata: props.metadata })
    }
    if (props.node.onPress && props.metadata._component) {
      PluginRegistry.emit(props.metadata._component, props.node.onPress, {})
      if (props.metadata._partId) {
        setTimeout(() => PluginRegistry.dismiss(props.metadata._partId!), 0)
      }
    }
  }

  useKeyboard((evt) => {
    if (clicked()) return false
    if (props.node.shortcut && evt.name === props.node.shortcut.toLowerCase()) {
      handleActivate()
      return true
    }
    return false
  })

  const isActive = () => clicked() || hovered()
  const hasBackground = props.node.bg !== undefined

  return (
    <box flexDirection="row">
      <Show when={hasBackground}>
        <box minWidth={1}>
          <text content={isActive() ? "┃" : " "} fg={getColor(props.node.borderColorHover) ?? theme.warning} />
        </box>
      </Show>
      <box
        backgroundColor={hasBackground ? (getColor(props.node.bg) ?? theme.accent) : undefined}
        paddingLeft={hasBackground ? 1 : 0}
        paddingRight={hasBackground ? 2 : 0}
        onMouseOver={() => setHovered(true)}
        onMouseOut={() => setHovered(false)}
        onMouseDown={handleActivate}
      >
        <text
          content={props.node.label}
          fg={
            isActive()
              ? (getColor(props.node.fgHover) ?? theme.warning)
              : (getColor(props.node.fg) ?? (hasBackground ? theme.background : theme.text))
          }
        />
      </box>
    </box>
  )
}

function PluginCollapsible(props: RendererProps<CollapsibleNode>) {
  const { getColor, theme } = useColors()
  const [expanded, setExpanded] = createSignal(props.node.expanded ?? false)
  const title = () => interpolate(props.node.title, props.metadata)
  const fg = () =>
    expanded() ? (getColor(props.node.fgExpanded) ?? theme.text) : (getColor(props.node.fg) ?? theme.textMuted)

  return (
    <box flexDirection="column" gap={0}>
      <box flexDirection="row" gap={1} onMouseDown={() => setExpanded((e) => !e)}>
        <text content={expanded() ? (props.node.iconExpanded ?? "▼") : (props.node.icon ?? "▶")} fg={fg()} />
        <text content={title()} fg={fg()} />
      </box>
      <Show when={expanded()}>
        <box flexDirection="column" paddingLeft={2}>
          <For each={props.node.children}>{(child) => <PluginUIRenderer node={child} metadata={props.metadata} />}</For>
        </box>
      </Show>
    </box>
  )
}

// Component map for dynamic rendering
const RENDERERS: Record<string, (props: RendererProps<any>) => any> = {
  text: PluginText,
  box: PluginBox,
  checklist: PluginChecklist,
  button: PluginButton,
  collapsible: PluginCollapsible,
}

// Render a plugin UI template
export function PluginUIRenderer(props: { node: PluginUINode; metadata: Record<string, any> }) {
  const { theme } = useColors()
  const Renderer = RENDERERS[props.node.type]
  return (
    <ErrorBoundary fallback={<text fg={theme.error}>[plugin render error]</text>}>
      <Show when={Renderer} fallback={<text fg={theme.error}>[unknown: {props.node.type}]</text>}>
        <Dynamic component={Renderer} node={props.node} metadata={props.metadata} />
      </Show>
    </ErrorBoundary>
  )
}
