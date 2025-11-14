# Edit Tool "Read Before Write" Bug

This document explains:

- How the current `FileTime` + edit/write/patch tools behave
- Why multiple edits on the same file (rapid / concurrent) fail
- The design goals for a fix (support fast/concurrent edits safely)
- A concrete implementation plan that you can follow later

All paths below are relative to `packages/opencode`.

---

## Current Behavior and Invariants

### FileTime: read-before-write tracking

`src/file/time.ts`

```ts
export namespace FileTime {
  export const state = Instance.state(() => {
    const read: {
      [sessionID: string]: {
        [path: string]: Date | undefined
      }
    } = {}
    return { read }
  })

  export function read(sessionID: string, file: string) {
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
        `File ${filepath} has been modified since it was last read.\n` +
          `Last modification: ${stats.mtime.toISOString()}\n` +
          `Last read: ${time.toISOString()}\n\n` +
          `Please read the file again before modifying it.`,
      )
    }
  }
}
```

Key points:

- State is in-memory per `Instance.directory`:
  - `Instance.state` → `State.create` keying on `Instance.directory`.
- For each `(sessionID, filePath)` we store a **single** `Date` representing the last time the file was “trusted” as read.
- `assert` enforces two invariants for writes in that session:
  1. **A prior read must exist** for that `(sessionID, file)`.
  2. **The file must not have changed on disk since that read**, based on `mtime`.

This is the core safety feature: tools are not allowed to blindly overwrite files they haven’t read in this session, and they cannot overwrite something that has changed since the last read.

### Tools that use FileTime

1. **ReadTool** — primes the state

`src/tool/read.ts`

```ts
// after reading the file and formatting output
LSP.touchFile(filepath, false)
FileTime.read(ctx.sessionID, filepath)
```

So a successful `read` marks that file as “known” for this `sessionID`.

2. **EditTool** — enforces read-before-edit

`src/tool/edit.ts`

```ts
const file = Bun.file(filePath)
const stats = await file.stat().catch(() => {})
if (!stats) throw new Error(`File ${filePath} not found`)
if (stats.isDirectory()) throw new Error(`Path is a directory, not a file: ${filePath}`)
await FileTime.assert(ctx.sessionID, filePath)
contentOld = await file.text()
contentNew = replace(contentOld, params.oldString, params.newString, params.replaceAll)

// later
await file.write(contentNew)
// ... diagnostics, diff, etc.
FileTime.read(ctx.sessionID, filePath)
```

Important:

- For non-empty `oldString`, **every edit** requires a prior `FileTime.read` in the same `sessionID`.
- After a successful write, the tool calls `FileTime.read` again, updating the last-read timestamp.

3. **WriteTool** — same pattern

`src/tool/write.ts`

```ts
const file = Bun.file(filepath)
const exists = await file.exists()
if (exists) await FileTime.assert(ctx.sessionID, filepath)

await Bun.write(filepath, params.content)
// ... events, diagnostics
FileTime.read(ctx.sessionID, filepath)
```

4. **PatchTool** — uses assert + read + write + read

`src/tool/patch.ts`

For update/delete cases:

```ts
// update
await FileTime.assert(ctx.sessionID, filePath)
const oldContent = await fs.readFile(filePath, "utf-8")
// compute newContent
// later, when applying changes
await fs.writeFile(change.filePath, change.newContent, "utf-8")
FileTime.read(ctx.sessionID, change.filePath)

// delete
await FileTime.assert(ctx.sessionID, filePath)
const contentToDelete = await fs.readFile(filePath, "utf-8")
// later
await fs.unlink(change.filePath)
FileTime.read(ctx.sessionID, change.filePath)
```

### Attachments that prime FileTime

`src/session/prompt.ts` (when a file attachment is processed):

```ts
const file = Bun.file(filepath)
FileTime.read(input.sessionID, filepath)
```

So **either** a `read` tool call or attaching a file can mark it as “read” for this session.

---

## The Bug: Rapid / Concurrent Edits on the Same File

Observed behavior from usage:

- Call `read` (or attach a file) once.
- Then issue multiple `edit` calls on the same file in rapid succession, often in parallel (e.g. the model plans several edits and they all get sent to the tool executor quickly).
- Result:
  - The **first** edit succeeds.
  - Some or all **subsequent** edits for that file fail with:
    - "You must read the file ... before overwriting it. Use the Read tool first" **or**
    - "File ... has been modified since it was last read ... Please read the file again".

This is not what we want. We want:

- A single read to enable **multiple edits** to the same file in a workflow, even if they’re issued quickly / concurrently.
- Failures only when there is **no read at all** in this session, or when the file changed **externally** since that read.

### Why it fails: per-session state + races

1. **Per-session, per-file state**

- `FileTime.read(sessionID, path)` stores `new Date()`.
- `FileTime.assert(sessionID, path)` requires that entry to exist and that `mtime <= lastReadTime`.

Implications:

- If an integration uses **different `sessionID`s** for `read` and `edit` calls on the same file:
  - `read` under session `S1` → `read[S1][path]` set
  - `edit` under session `S2` → `read[S2][path]` missing
  - `FileTime.assert(S2, path)` throws the "You must read the file ..." error.
- This is correct from the code’s point of view, but surprising if the integration thinks these calls are part of the same workflow.

2. **Concurrent edits share a single last-read time**

Imagine this sequence with one session `S` and file `F`:

- `read(S, F)` is called once, storing `T0`.
- Two `edit` calls on `F` start at about the same time (in parallel):
  - Both hit `FileTime.assert(S, F)`.
  - Both see the same `lastReadTime = T0`.
- One edit (E1) finishes its work first:
  - Reads the file (state at T0).
  - Computes new content.
  - Writes the file, which updates `mtime` to `T1 > T0`.
  - Calls `FileTime.read(S, F)`, setting last-read time to (approximately) `T1`.
- The other edit (E2) is still in flight:
  - Depending on timing, when it runs `FileTime.assert`, it may see either:
    - `lastReadTime = T0` (if the write completed but `FileTime.read` hasn’t run yet), or
    - `lastReadTime = ~T1` (if it runs after E1 updated the time).

In the first case (common), E2 sees:

- `mtime(F) = T1` (updated by E1’s write)
- `lastReadTime = T0`

Because `T1 > T0`, `assert` throws:

> File F has been modified since it was last read. Please read the file again before modifying it.

This is the **race**: we’re comparing global file `mtime` to a single shared logical read time, with no ordering between overlapping writes.

3. **In-memory state is per-directory, not per-call**

- `FileTime.state` lives in an `Instance.state`, keyed by the current `Instance.directory`.
- For a given instance (directory), all sessions share the same `FileTime.state`, but their entries are separated by `sessionID`.
- There is no automatic per-file serialization of operations.
- Hosts are allowed to fire multiple tools on the same file concurrently (and the model naturally does this when emitting parallel tool calls).

All of this leads to:

- Concurrent edits in the **same** session racing with each other on `mtime` vs `lastReadTime`.
- Per-session separation meaning a read in one session does not count for edits in another.

---

## Design Goals for a Fix

Given the above, we want a solution that:

1. **Supports rapid / concurrent edits on the same file**
   - A single read should enable many edits in quick succession.
   - The host/model can issue multiple edits concurrently; they shouldn’t fail just because they overlap in time.

2. **Preserves the safety invariant**
   - There must be a read in this session before writes.
   - If the file is changed externally (another process, another editor, or another session) after that read, a write should fail until it’s re-read.

3. **Is localized and easy to reason about**
   - Changes should live mostly in: `FileTime` + write-like tools (`edit`, `write`, `patch`).
   - No large changes to the session or server infrastructure.

4. **Keeps cross-session isolation**
   - Reads/writes in one session should not silently bless edits from a different session.
   - If session `S2` wants to edit a file that session `S1` read, `S2` must read it too.

---

## Chosen Approach: Per-File Locking + Existing Invariants

After reviewing alternatives, the best approach that satisfies all goals is:

1. **Keep the read-before-write contract and per-session semantics as-is.**
2. **Add a per-file, per-instance lock for write operations.**

### Why not relax `FileTime.assert`?

Some ideas considered and rejected (or deferred):

- Allow writes without a prior read
  - This would remove the guard that prevents blind overwrites.
- Ignore tool-owned writes when checking `mtime`
  - Requires tracking “who wrote when”, and still doesn’t fully eliminate races if that metadata becomes stale.
- Switch `FileTime.read` to use `stat().mtime` instead of `new Date()`
  - Better alignment of times, but doesn’t fix the fundamental race when different edits overlap.

All of these either weaken safety guarantees or fail to address concurrent edits robustly.

### Why per-file locking works well

Introduce a small per-file lock so that for any given file path within an instance:

- Only **one** write-like operation executes its critical section at a time.
- The critical section includes:
  - `FileTime.assert(sessionID, filepath)`
  - Reading the file contents
  - Computing new content
  - Writing the file
  - `FileTime.read(sessionID, filepath)` after the write

This ensures that:

- Two edits on the same file in the same session cannot interleave their assertions and writes in an unsafe way.
- The second edit will see a consistent state: either
  - Fails because the file changed externally since the last read, or
  - Succeeds after the previous edit has fully updated both the file and the `FileTime.read` value.
- Edits to **different** files remain concurrent; we only serialize per file, which is exactly where the race happens.

The host/model can still send concurrent edit calls:

- They will be queued for that file and executed one after another, not rejected.
- The last one wins in terms of final file content, which is the expected behavior when multiple edits are requested.

---

## Concrete Implementation Plan

Below is a step-by-step plan that you (or another contributor) can follow later, without needing this session.

### 1. Ensure integrations use a consistent `sessionID`

Before code changes, document and check integration behavior:

- For any workflow where a tool is expected to read and then edit the same file, **all tool calls must use the same `sessionID`**.
- If an integration (MCP server, VS Code, GitHub Action, etc.) currently generates a new `sessionID` per call, adjust it so that:
  - A user “session” or “conversation” maps to `sessionID`.
  - `read` and subsequent `edit`/`write`/`patch` calls for a file share that `sessionID`.

This avoids the trivial case where we _always_ get "You must read the file ..." simply because the session ID changed.

_No code changes required for this step, but it’s important context for the rest._

### 2. Introduce a per-file lock helper

Create a small helper that lives in the `FileTime` namespace or near it, using `Instance.state` for per-directory storage.

Example shape (pseudocode-ish, not exact):

```ts
// src/file/time.ts

export namespace FileTime {
  const state = Instance.state(() => {
    const read: Record<string, Record<string, Date | undefined>> = {}
    const locks = new Map<string, Promise<void>>()
    return { read, locks }
  })

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
      resolveNext()
      return result
    } catch (error) {
      resolveNext()
      throw error
    } finally {
      // Optional: clean up if no one is waiting anymore
    }
  }

  // existing read/get/assert stay as-is
}
```

Notes:

- This uses `locks` stored alongside `read` within the same `Instance.state` call.
- For each file path, we maintain a promise chain so that calls for the same path run serially.
- This is one of several possible mutex implementations; the key concept is one-at-a-time per file.

### 3. Wrap EditTool write path with the lock

In `src/tool/edit.ts`, adjust the non-empty `oldString` branch to use `FileTime.withLock` around its core mutation.

Currently:

```ts
const file = Bun.file(filePath)
const stats = await file.stat().catch(() => {})
if (!stats) throw new Error(`File ${filePath} not found`)
if (stats.isDirectory()) throw new Error(`Path is a directory, not a file: ${filePath}`)
await FileTime.assert(ctx.sessionID, filePath)
contentOld = await file.text()
contentNew = replace(...)
// build diff, ask permissions
await file.write(contentNew)
// publish events, diagnostics
FileTime.read(ctx.sessionID, filePath)
```

Proposed structure:

```ts
await FileTime.withLock(filePath, async () => {
  const file = Bun.file(filePath)
  const stats = await file.stat().catch(() => {})
  if (!stats) throw new Error(`File ${filePath} not found`)
  if (stats.isDirectory()) throw new Error(`Path is a directory, not a file: ${filePath}`)

  await FileTime.assert(ctx.sessionID, filePath)

  contentOld = await file.text()
  contentNew = replace(contentOld, params.oldString, params.newString, params.replaceAll)

  // compute diff before permission prompt
  diff = trimDiff(createTwoFilesPatch(filePath, filePath, contentOld, contentNew))

  if (agent.permission.edit === "ask") {
    await Permission.ask({
      /* unchanged */
    })
  }

  await file.write(contentNew)

  // Optionally: re-stat and use mtime to set read time
  FileTime.read(ctx.sessionID, filePath)
})

// rest of code: diagnostics, Snapshot.FileDiff, return value
```

Key points:

- Only the critical mutation path is inside `withLock`.
- Diagnostics, LSP, diff summary, etc., can use the `contentOld`/`contentNew` captured from inside.
- Multiple `edit` calls on the same file will queue; they will not step on each other’s `assert`+`write`.

### 4. Wrap WriteTool’s overwrite path

In `src/tool/write.ts`:

```ts
const file = Bun.file(filepath)
const exists = await file.exists()
if (exists) await FileTime.assert(ctx.sessionID, filepath)

// permissions
await Bun.write(filepath, params.content)
FileTime.read(ctx.sessionID, filepath)
```

Change to something like:

```ts
await FileTime.withLock(filepath, async () => {
  const file = Bun.file(filepath)
  const exists = await file.exists()
  if (exists) await FileTime.assert(ctx.sessionID, filepath)

  if (agent.permission.edit === "ask") {
    await Permission.ask({
      /* unchanged */
    })
  }

  await Bun.write(filepath, params.content)
  FileTime.read(ctx.sessionID, filepath)
})
```

This prevents concurrent `write` calls from racing on the same file and sharing stale read times.

### 5. Wrap PatchTool per-file changes

`src/tool/patch.ts` is a bit more involved because it works on multiple files, but the idea is similar.

When applying changes (roughly lines ~157+):

```ts
for (const change of fileChanges) {
  switch (change.type) {
    case "add":
      // no prior file, no assert needed
      // create dirs and write file
      break

    case "update":
      await fs.writeFile(change.filePath, change.newContent, "utf-8")
      break

    case "move":
      // write to new path, delete old
      break

    case "delete":
      await fs.unlink(change.filePath)
      break
  }

  FileTime.read(ctx.sessionID, change.filePath)
  if (change.movePath) {
    FileTime.read(ctx.sessionID, change.movePath)
  }
}
```

Change to:

- For each `change`, if it’s `update`, `move`, or `delete`, wrap the assert + write + `FileTime.read` in `withLock(change.filePath, ...)`.
- For `move`, also lock the destination path when writing and updating its `FileTime.read`.

Example sketch for `update`:

```ts
await FileTime.withLock(change.filePath, async () => {
  // we already did FileTime.assert + fs.readFile earlier when building fileChanges
  await fs.writeFile(change.filePath, change.newContent, "utf-8")
  FileTime.read(ctx.sessionID, change.filePath)
})
```

This keeps patch application safe when multiple patches target the same file concurrently.

### 6. Optional: align read time with mtime

With locks in place, `FileTime.read(ctx.sessionID, file)` being `new Date()` is generally sufficient.

If you want tighter alignment:

- After a write, you can `stat` the file and set the read time based on `stats.mtime` instead of `new Date()`.
- This makes the comparison in `assert` purely based on filesystem timestamps and avoids edge-cases where clocks differ.

Example (inside the lock, after write):

```ts
const stats = await Bun.file(filepath).stat()
const { read } = FileTime.state()
read[ctx.sessionID] = read[ctx.sessionID] || {}
read[ctx.sessionID][filepath] = stats.mtime
```

However, this is not strictly necessary once the operations are serialized per file.

### 7. Preserve and clarify error semantics

Keep the existing two error messages as-is; they are meaningful:

- No `FileTime.read` entry → "You must read the file ... Use the Read tool first".
- `mtime > lastReadTime` → "File ... has been modified since it was last read ... Please read the file again".

With per-file locking:

- The second error should now primarily indicate **external** modifications:
  - Another process or editor changed the file after it was read.
  - Another session modified the file after your session’s last read.

As a follow-up, we can improve the copy to say explicitly:

- “This can happen if the file was edited in another editor or session. Re-read it with the Read tool and try again.”

### 8. Add targeted tests

To guard against regressions, add tests to the `packages/opencode` test suite (wherever edit/write/patch tools are already tested).

Scenarios to cover:

1. **Sequential edits on one file, one session**
   - `read` file
   - `edit` call A
   - `edit` call B immediately after A completes
   - Expect both succeed; final content matches B.

2. **Concurrent edits on one file, one session**
   - `read` file
   - Fire N `edit` calls in parallel on the same file (with different replacements)
   - With the new lock:
     - All edit calls complete without “read before write” or “modified since last read” errors.
     - Final content matches the edit that logically runs last (deterministic order given by lock queue).

3. **Cross-session read isolation**
   - Session `S1`: `read` file
   - Session `S2`: `edit` same file, without a `read` in `S2`
   - Expect `FileTime.assert` error: "You must read the file ... Use the Read tool first".

4. **External modification detection**
   - Session `S`: `read` file
   - Modify the file on disk outside of tools (simulate user editor / fs write)
   - `edit` in session `S`
   - Expect “File ... has been modified since it was last read ... Please read the file again".

---

## Expected Outcome After Implementing This Plan

Once the per-file locking and related changes are implemented:

- A **single** read in a session enables multiple edits in that same session on the same file, even if the edits are issued quickly or concurrently.
- The tools do **not** randomly throw “You must read the file” or “modified since last read” errors just because the model sent edits in parallel.
- The safety invariant remains:
  - You must read before writing, per session.
  - External or cross-session modifications after that read are detected and require re-reading.
- The solution is localized to `FileTime` + `edit`/`write`/`patch`, and does not require large restructuring of session or server logic.

This plan is intentionally detailed so you can pick it up later, without needing this chat context, and implement it step by step in the repo.
