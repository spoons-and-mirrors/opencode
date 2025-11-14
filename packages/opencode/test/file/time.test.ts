import { describe, expect, test } from "bun:test"
import path from "path"
import * as fs from "fs/promises"
import { EditTool } from "../../src/tool/edit"
import { WriteTool } from "../../src/tool/write"
import { ReadTool } from "../../src/tool/read"
import { Instance } from "../../src/project/instance"
import { FileTime } from "../../src/file/time"
import { tmpdir } from "../fixture/fixture"

const createContext = (sessionID: string) => ({
  sessionID,
  messageID: "",
  toolCallID: "",
  callID: "",
  agent: "build",
  abort: AbortSignal.any([]),
  metadata: () => {},
})

const editTool = await EditTool.init()
const writeTool = await WriteTool.init()
const readTool = await ReadTool.init()

describe("file.time - concurrent edits", () => {
  test("should allow sequential edits on same file after single read", async () => {
    await using fixture = await tmpdir()

    await Instance.provide({
      directory: fixture.path,
      fn: async () => {
        const filepath = path.join(fixture.path, "test.txt")
        const ctx = createContext("session1")

        // Create initial file
        await fs.writeFile(filepath, "line 1\nline 2\nline 3", "utf-8")

        // Read file once
        await readTool.execute({ filePath: filepath }, ctx)

        // Perform first edit
        await editTool.execute(
          {
            filePath: filepath,
            oldString: "line 1",
            newString: "line 1 updated",
          },
          ctx,
        )

        // Perform second edit immediately after first completes
        await editTool.execute(
          {
            filePath: filepath,
            oldString: "line 2",
            newString: "line 2 updated",
          },
          ctx,
        )

        // Verify final content
        const finalContent = await fs.readFile(filepath, "utf-8")
        expect(finalContent).toBe("line 1 updated\nline 2 updated\nline 3")
      },
    })
  })

  test("should allow concurrent edits on same file after single read", async () => {
    await using fixture = await tmpdir()

    await Instance.provide({
      directory: fixture.path,
      fn: async () => {
        const filepath = path.join(fixture.path, "test.txt")
        const ctx = createContext("session1")

        // Create initial file with numbered lines
        const lines = Array.from({ length: 10 }, (_, i) => `line ${i + 1}`)
        await fs.writeFile(filepath, lines.join("\n"), "utf-8")

        // Read file once
        await readTool.execute({ filePath: filepath }, ctx)

        // Fire multiple edits concurrently on different lines
        const edits = [
          editTool.execute(
            {
              filePath: filepath,
              oldString: "line 1",
              newString: "line 1 edited",
            },
            ctx,
          ),
          editTool.execute(
            {
              filePath: filepath,
              oldString: "line 5",
              newString: "line 5 edited",
            },
            ctx,
          ),
          editTool.execute(
            {
              filePath: filepath,
              oldString: "line 9",
              newString: "line 9 edited",
            },
            ctx,
          ),
        ]

        // All edits should complete without errors
        await Promise.all(edits)

        // Verify that file exists and has been modified
        const finalContent = await fs.readFile(filepath, "utf-8")
        expect(finalContent).toContain("edited")
        // At least one of the edits should be present
        expect(
          finalContent.includes("line 1 edited") ||
            finalContent.includes("line 5 edited") ||
            finalContent.includes("line 9 edited"),
        ).toBe(true)
      },
    })
  })

  test("should enforce cross-session read isolation", async () => {
    await using fixture = await tmpdir()

    await Instance.provide({
      directory: fixture.path,
      fn: async () => {
        const filepath = path.join(fixture.path, "test.txt")
        const session1 = createContext("session1")
        const session2 = createContext("session2")

        // Create initial file
        await fs.writeFile(filepath, "original content", "utf-8")

        // Session 1 reads the file
        await readTool.execute({ filePath: filepath }, session1)

        // Session 2 tries to edit without reading
        await expect(
          editTool.execute(
            {
              filePath: filepath,
              oldString: "original",
              newString: "modified",
            },
            session2,
          ),
        ).rejects.toThrow("You must read the file")
      },
    })
  })

  test("should detect external modifications", async () => {
    await using fixture = await tmpdir()

    await Instance.provide({
      directory: fixture.path,
      fn: async () => {
        const filepath = path.join(fixture.path, "test.txt")
        const ctx = createContext("session1")

        // Create initial file
        await fs.writeFile(filepath, "original content", "utf-8")

        // Read file
        await readTool.execute({ filePath: filepath }, ctx)

        // Wait a bit to ensure mtime difference
        await new Promise((resolve) => setTimeout(resolve, 100))

        // Modify file externally (simulate another process)
        await fs.writeFile(filepath, "externally modified content", "utf-8")

        // Try to edit - should fail because file was modified since read
        await expect(
          editTool.execute(
            {
              filePath: filepath,
              oldString: "original",
              newString: "my edit",
            },
            ctx,
          ),
        ).rejects.toThrow("has been modified since it was last read")
      },
    })
  })

  test("should allow concurrent write operations on same file", async () => {
    await using fixture = await tmpdir()

    await Instance.provide({
      directory: fixture.path,
      fn: async () => {
        const filepath = path.join(fixture.path, "test.txt")
        const ctx = createContext("session1")

        // Create initial file
        await fs.writeFile(filepath, "initial", "utf-8")

        // Read file once
        await readTool.execute({ filePath: filepath }, ctx)

        // Fire multiple write operations concurrently
        const writes = [
          writeTool.execute(
            {
              filePath: filepath,
              content: "write 1",
            },
            ctx,
          ),
          writeTool.execute(
            {
              filePath: filepath,
              content: "write 2",
            },
            ctx,
          ),
          writeTool.execute(
            {
              filePath: filepath,
              content: "write 3",
            },
            ctx,
          ),
        ]

        // All writes should complete without errors
        await Promise.all(writes)

        // Verify file exists and has one of the written contents
        const finalContent = await fs.readFile(filepath, "utf-8")
        expect(["write 1", "write 2", "write 3"]).toContain(finalContent)
      },
    })
  })

  test("should serialize edits to ensure deterministic order", async () => {
    await using fixture = await tmpdir()

    await Instance.provide({
      directory: fixture.path,
      fn: async () => {
        const filepath = path.join(fixture.path, "counter.txt")
        const ctx = createContext("session1")

        // Create initial file with a counter
        await fs.writeFile(filepath, "0", "utf-8")

        // Read file once
        await readTool.execute({ filePath: filepath }, ctx)

        // Fire multiple edits that increment the counter
        // With proper locking, these should execute in order
        const edits = []
        for (let i = 0; i < 5; i++) {
          edits.push(
            editTool.execute(
              {
                filePath: filepath,
                oldString: `${i}`,
                newString: `${i + 1}`,
              },
              ctx,
            ),
          )
        }

        // All edits should complete
        await Promise.all(edits)

        // Due to locking, edits should execute serially
        // Final content should be "5"
        const finalContent = await fs.readFile(filepath, "utf-8")
        expect(finalContent).toBe("5")
      },
    })
  })

  test("should handle edits on different files concurrently", async () => {
    await using fixture = await tmpdir()

    await Instance.provide({
      directory: fixture.path,
      fn: async () => {
        const ctx = createContext("session1")

        // Create multiple files
        const files = ["file1.txt", "file2.txt", "file3.txt"]
        for (const file of files) {
          const filepath = path.join(fixture.path, file)
          await fs.writeFile(filepath, "original", "utf-8")
          await readTool.execute({ filePath: filepath }, ctx)
        }

        // Edit all files concurrently
        const edits = files.map((file) =>
          editTool.execute(
            {
              filePath: path.join(fixture.path, file),
              oldString: "original",
              newString: "modified",
            },
            ctx,
          ),
        )

        // All edits should complete without errors
        await Promise.all(edits)

        // Verify all files were modified
        for (const file of files) {
          const filepath = path.join(fixture.path, file)
          const content = await fs.readFile(filepath, "utf-8")
          expect(content).toBe("modified")
        }
      },
    })
  })

  test("should preserve FileTime state across lock acquisitions", async () => {
    await using fixture = await tmpdir()

    await Instance.provide({
      directory: fixture.path,
      fn: async () => {
        const filepath = path.join(fixture.path, "test.txt")
        const ctx = createContext("session1")

        // Create initial file
        await fs.writeFile(filepath, "content", "utf-8")

        // Read file
        await readTool.execute({ filePath: filepath }, ctx)

        // Verify FileTime has the read timestamp
        const readTime1 = FileTime.get(ctx.sessionID, filepath)
        expect(readTime1).toBeDefined()

        // Perform an edit
        await editTool.execute(
          {
            filePath: filepath,
            oldString: "content",
            newString: "updated",
          },
          ctx,
        )

        // Verify FileTime was updated after edit
        const readTime2 = FileTime.get(ctx.sessionID, filepath)
        expect(readTime2).toBeDefined()
        expect(readTime2!.getTime()).toBeGreaterThanOrEqual(readTime1!.getTime())

        // Perform another edit immediately
        await editTool.execute(
          {
            filePath: filepath,
            oldString: "updated",
            newString: "final",
          },
          ctx,
        )

        // Should succeed because FileTime was updated by previous edit
        const finalContent = await fs.readFile(filepath, "utf-8")
        expect(finalContent).toBe("final")
      },
    })
  })
})
