import { Instance } from "../project/instance"
import { Log } from "../util/log"

export namespace FileTime {
  const log = Log.create({ service: "file.time" })
  export const state = Instance.state(() => {
    const read: {
      [sessionID: string]: {
        [path: string]: Date | undefined
      }
    } = {}
    const locks = new Map<string, Promise<void>>()
    return {
      read,
      locks,
    }
  })

  export function read(sessionID: string, file: string) {
    log.info("read", { sessionID, file })
    const { read } = state()
    read[sessionID] = read[sessionID] || {}
    read[sessionID][file] = new Date()
  }

  export function get(sessionID: string, file: string) {
    return state().read[sessionID]?.[file]
  }

  export async function assert(sessionID: string, filepath: string) {
    const time = get(sessionID, filepath)
    if (!time) throw new Error(`You must read the file ${filepath} before overwriting it. Use the Read tool first`)
    const stats = await Bun.file(filepath).stat()
    if (stats.mtime.getTime() > time.getTime()) {
      throw new Error(
        `File ${filepath} has been modified since it was last read.\nLast modification: ${stats.mtime.toISOString()}\nLast read: ${time.toISOString()}\n\nPlease read the file again before modifying it.`,
      )
    }
  }

  export async function withLock<T>(filepath: string, fn: () => Promise<T>): Promise<T> {
    const { locks } = state()
    const key = filepath

    const previous = locks.get(key) || Promise.resolve()
    let resolveNext: () => void

    const next = new Promise<void>((resolve) => {
      resolveNext = resolve
    })
    locks.set(
      key,
      previous.then(() => next),
    )

    try {
      await previous
      const result = await fn()
      resolveNext!()
      return result
    } catch (error) {
      resolveNext!()
      throw error
    }
  }
}
