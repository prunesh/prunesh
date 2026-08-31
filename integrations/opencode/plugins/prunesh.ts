import type { Plugin } from "@opencode-ai/plugin"
import { existsSync } from "node:fs"
import { spawnSync } from "node:child_process"

function findGtkai(): string | null {
  const fromPath = Bun.which("prunesh")
  if (fromPath) return fromPath
  const home = Bun.env.HOME
  if (!home) return null
  for (const candidate of [
    `${home}/.local/bin/prunesh`,
    "/usr/local/bin/prunesh",
    "/opt/homebrew/bin/prunesh",
  ]) {
    if (existsSync(candidate)) return candidate
  }
  return null
}

function markerExists(): boolean {
  const r = spawnSync("git", ["rev-parse", "--show-toplevel"], { encoding: "utf8" })
  const root = r.status === 0 ? r.stdout.trim() : process.cwd()
  return existsSync(`${root}/.prunesh`)
}

function runHook(prunesh: string, args: string[], payload: unknown): string {
  const r = Bun.spawnSync([prunesh, ...args], {
    stdin: new TextEncoder().encode(JSON.stringify(payload)),
    stderr: "ignore",
  })
  if (r.exitCode !== 0 || !r.stdout) return ""
  return r.stdout.toString().trim()
}

export const GtkAI: Plugin = async () => {
  const prunesh = findGtkai()
  const argsByCall = new Map<string, Record<string, unknown>>()

  return {
    "tool.execute.before": async (input, output) => {
      argsByCall.set(input.callID, output.args as Record<string, unknown>)
      if (!prunesh) return
      if (!markerExists()) return
      if (input.tool !== "bash" && input.tool !== "shell") return
      const command = (output.args as { command?: unknown }).command
      if (typeof command !== "string" || command === "") return

      const raw = runHook(prunesh, ["hook-pre", "--agent=opencode"], {
        tool_name: input.tool,
        tool_input: output.args,
      })
      if (raw === "") return
      let parsed: { command?: unknown }
      try {
        parsed = JSON.parse(raw) as { command?: unknown }
      } catch {
        return
      }
      if (typeof parsed.command !== "string" || parsed.command === "") return
      output.args.command = parsed.command
    },

    "tool.execute.after": async (input, output) => {
      const args = argsByCall.get(input.callID)
      argsByCall.delete(input.callID)
      if (!prunesh) return
      if (!markerExists()) return
      if (args === undefined) return
      if (input.tool === "bash" || input.tool === "shell") return
      if (output.output === "") return

      const isRead = input.tool === "read"
      const isMCP = input.tool.startsWith("mcp__") || input.tool.startsWith("MCP:")
      if (!isRead && !isMCP) return

      let toolInput: Record<string, unknown> = args
      if (isRead) {
        const filePath = args.filePath
        if (typeof filePath !== "string" || filePath === "") return
        toolInput = { file_path: filePath }
      }

      const raw = runHook(prunesh, ["hook-post", "--agent=opencode"], {
        tool_name: isRead ? "Read" : input.tool,
        tool_input: toolInput,
        tool_response: [{ type: "text", text: output.output }],
      })
      if (raw === "") return
      let parsed: { output?: unknown }
      try {
        parsed = JSON.parse(raw) as { output?: unknown }
      } catch {
        return
      }
      if (typeof parsed.output !== "string" || parsed.output === "") return
      output.output = parsed.output
    },
  }
}
