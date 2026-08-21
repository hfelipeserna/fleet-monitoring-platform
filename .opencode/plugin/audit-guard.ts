import { mkdirSync, appendFileSync, readFileSync, readdirSync, statSync, existsSync } from "node:fs"
import { join, resolve, sep } from "node:path"
import type { Plugin } from "@opencode-ai/plugin"

const HARD_SECRET_PATTERNS: RegExp[] = [
  /AKIA[0-9A-Z]{16}/,
  /-----BEGIN( RSA| EC| OPENSSH| PGP)? PRIVATE KEY-----/,
]

const SOFT_SECRET_PATTERNS: RegExp[] = [
  /(client_secret|api_key|apikey|password|passwd)["']?\s*[:=]\s*["'][^"']{16,}/i,
]

const ALLOWED_SUFFIXES = [".example", ".md", ".opencode.json", ".tfvars.example"]

const toolNameOf = (input: any, output: any) => output?.toolName ?? input?.tool ?? (input as any)?.toolName

function isAllowedSecretPath(filePath: string): boolean {
  return ALLOWED_SUFFIXES.some((s) => filePath.endsWith(s))
}

function secretGuardMode(): "strict" | "warn" {
  return process.env.FMP_SECRET_GUARD === "strict" ? "strict" : "warn"
}

function entryForEditLog(filePath: string, ts: string): string {
  return `- ts: ${ts}\n  file: ${filePath}\n`
}

function collectGoFiles(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry)
    const st = statSync(p)
    if (st.isDirectory()) collectGoFiles(p, out)
    else if (p.endsWith(".go")) out.push(p)
  }
  return out
}

function auditArchitecture(root: string): string {
  const base = resolve(root)
  if (!existsSync(base)) {
    return `No existe ${root}. Todavía no hay backend Go; crea backend/ o pasa otro root (ej. audit_architecture root=cmd/ingestor).`
  }

  const goMod = join(base, "go.mod")
  if (!existsSync(goMod)) {
    return `No hay go.mod en ${root}. El módulo Go aún no está inicializado; executa: go mod init <module>`
  }
  const module = /^module\s+(\S+)/m.exec(readFileSync(goMod, "utf8"))?.[1]
  if (!module) return `go.mod sin 'module' en ${root}.`

  const forbidden: Record<string, string[]> = {
    domain: ["/application/", "/adapters/", "/infra/"],
    application: ["/adapters/", "/infra/"],
  }

  const files = collectGoFiles(base)
  if (files.length === 0) return `Sin archivos .go en ${root}.`

  const violations: string[] = []
  const layerOf = (rel: string): string | undefined => {
    for (const layer of Object.keys(forbidden)) {
      if (rel.includes(`/${layer}/`)) return layer
    }
    return undefined
  }
  for (const file of files) {
    const rel = resolve(file).replace(base, sep)
    const layer = layerOf(rel)
    const deny = layer ? forbidden[layer] : undefined
    if (!deny) continue
    const src = readFileSync(file, "utf8")
    const singleImport = /^\s*import\s+"([^"]+)"/gm
    const blockImport = /^\s*"([^"]+)"/gm
    const allImports = [...src.matchAll(singleImport), ...src.matchAll(blockImport)]
    for (const m of allImports) {
      const imp = m[1]
      if (!imp.startsWith(module)) continue
      if (deny.some((seg) => imp.includes(seg))) {
        violations.push(`${file.replace(base + sep, "")}: import ${imp}`)
      }
    }
  }

  if (violations.length === 0) {
    return `Auditoría clean architecture OK en ${root} (${files.length} archivos .go analizados en ${module}).`
  }
  return `VIOLACIONES de clean architecture en ${root} (${violations.length}) deber ser corregidas:\n${violations.join("\n")}`
}

export default (async () => {
  return {
    "tool.execute.after": async (input: any, output: any) => {
      try {
        const tool = toolNameOf(input, output)
        if (tool !== "edit" && tool !== "write") return
        const filePath = output?.args?.filePath
        if (!filePath) return
        mkdirSync(resolve(".ai-audit"), { recursive: true })
        appendFileSync(
          resolve(".ai-audit/edit-log.yaml"),
          entryForEditLog(filePath, new Date().toISOString()),
        )
      } catch {
        /* el registro de auditoría nunca debe romper la edición */
      }
    },
    "tool.execute.before": async (input: any, output: any) => {
      try {
        const tool = toolNameOf(input, output)
        if (tool !== "edit" && tool !== "write") return
        const filePath: string = output?.args?.filePath ?? ""
        const content: string = output?.args?.content ?? output?.args?.newString ?? ""
        if (!content || isAllowedSecretPath(filePath)) return
        const mode = secretGuardMode()
        const hard = HARD_SECRET_PATTERNS.find((re) => re.test(String(content)))
        if (hard) {
          const msg = `[guard] posible secreto inequívoco en ${filePath} (${hard}). NO lo commitees.`
          if (mode === "strict" || process.env.FMP_SECRET_GUARD === "strict") {
            throw new Error(msg)
          }
          console.error(msg)
          return
        }
        const soft = SOFT_SECRET_PATTERNS.find((re) => re.test(String(content)))
        if (soft) {
          const msg = `[guard] posible secreto en ${filePath} (${soft}). NO lo commitees.`
          if (process.env.FMP_SECRET_GUARD === "strict") {
            throw new Error(msg)
          }
          console.error(msg)
        }
      } catch (e) {
        if (e instanceof Error && e.message.startsWith("[guard]")) throw e
      }
    },
    tool: {
      audit_architecture: {
        description:
          "Audita el backend Go por violaciones de clean architecture: imports prohibidos entre capas (domain/application no pueden importar adapters/infra). Devuelve un reporte con las violaciones.",
        args: {
          type: "object",
          properties: {
            root: {
              type: "string",
              description: "Directorio raíz del backend Go a auditar (por defecto 'backend')",
            },
          },
          required: [],
        },
        execute: async (args: any) => auditArchitecture(args?.root ?? "backend"),
      },
    },
  }
}) satisfies Plugin