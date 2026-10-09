// PoC04b: host OpenCode, `bash` tool overridden to run inside an OpenShell sandbox.
// Sandbox name from env Z3R0_SANDBOX. Other built-in tools (read/edit/write/grep/...) still run on host.
import { type Plugin, tool } from "@opencode-ai/plugin"

export const SandboxBash: Plugin = async ({ $ }) => ({
  tool: {
    bash: tool({
      description: "Run a shell command inside the Z3R0 sandbox (cwd /sandbox).",
      args: { command: tool.schema.string() },
      async execute(args) {
        const name = process.env.Z3R0_SANDBOX ?? "poc04b"
        const r = await $`openshell sandbox exec -n ${name} -- sh -c ${"cd /sandbox && " + args.command}`.nothrow().quiet()
        return r.stdout.toString() + r.stderr.toString()
      },
    }),
  },
})
