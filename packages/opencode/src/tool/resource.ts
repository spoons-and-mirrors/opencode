import { z } from "zod"
import * as path from "path"
import { Tool } from "./tool"
import { App } from "../app/app"
import { Filesystem } from "../util/filesystem"

export function createResourceTool(resourcePath: string, basePath: string, customDescription?: string) {
  const resourceName = path.basename(resourcePath, path.extname(resourcePath))
  const toolId = `resource_${resourceName.replace(/[^a-zA-Z0-9]/g, "_")}`

  const description = customDescription
    ? `${customDescription}`
    : `This tool provides access to the resource file: ${resourcePath}`

  return Tool.define(toolId, {
    description,
    parameters: z.object({
      // No parameters needed - tool is specific to one resource file
    }),
    async execute(_params, _ctx) {
      let filepath = resourcePath
      if (!path.isAbsolute(filepath)) {
        filepath = path.resolve(basePath, filepath)
      }

      const app = App.info()
      if (!Filesystem.contains(app.path.cwd, filepath)) {
        throw new Error(`Resource ${filepath} is not in the current working directory`)
      }

      try {
        const content = await Bun.file(filepath).text()
        return {
          title: `Resource: ${path.basename(resourcePath)}`,
          metadata: {
            resourcePath,
            size: content.length,
          },
          output: content,
        }
      } catch (err: any) {
        if (err.code === "ENOENT") {
          throw new Error(`Resource file not found: ${filepath}`)
        }
        throw new Error(`Failed to read resource file: ${err.message}`)
      }
    },
  })
}
