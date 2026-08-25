// Covers [SPEC-003: AC-006, AC-007, BR-007, BR-008]
import { describe, it, expect } from "vitest";
import fs from "fs";
import path from "path";

describe("Depguard + Compose guard", () => {
  // Covers [SPEC-003: AC-006, AC-007, BR-007, BR-008]

  it("web/src/chat no importa genkit/pgx/nats (depguard)", async () => {
    // Arrange
    const chatDir = path.resolve(__dirname);
    const files = fs.existsSync(chatDir)
      ? fs
          .readdirSync(chatDir)
          .filter((f) => f.endsWith(".ts") || f.endsWith(".tsx"))
          .filter((f) => !f.includes(".test."))
      : [];

    // Act
    let hasForbidden = false;
    let offending = "";
    for (const file of files) {
      const content = fs.readFileSync(path.join(chatDir, file), "utf-8");
      if (/from\s+["'].*genkit/i.test(content) || /import.*genkit/i.test(content)) {
        hasForbidden = true;
        offending = `${file}: genkit`;
        break;
      }
      if (/from\s+["'].*pgx/i.test(content) || /import.*pgx/i.test(content)) {
        hasForbidden = true;
        offending = `${file}: pgx`;
        break;
      }
      if (/from\s+["'].*nats/i.test(content) || /import.*nats/i.test(content)) {
        hasForbidden = true;
        offending = `${file}: nats`;
        break;
      }
    }

    // Assert
    // Si ChatWidget.tsx no existe aún, el test debe considerarse pendiente (RED) — verificamos existencia
    const chatWidgetPath = path.join(chatDir, "ChatWidget.tsx");
    const exists = fs.existsSync(chatWidgetPath);
    // Si no existe, es RED (aún no implementado) -> este expect hará FAIL hasta que exista
    // Para distinguir de depguard violado, primero verifica existencia
    expect(exists, "ChatWidget.tsx debe existir (TDD RED hasta implementar)").toBe(true);
    expect(hasForbidden, offending || "web/src/chat debe no importar genkit/pgx/nats").toBe(false);
  });

  it("docker compose config no leak GEMINI_API_KEY (placeholder)", async () => {
    // Arrange
    const composePath = path.resolve(__dirname, "../../../docker-compose.yml");
    // also try alternative path when running from web/
    const altPath = path.resolve(process.cwd(), "docker-compose.yml");
    const p = fs.existsSync(composePath) ? composePath : altPath;
    const content = fs.existsSync(p) ? fs.readFileSync(p, "utf-8") : "";

    // Act
    const hasPlaceholder = content.includes("${GEMINI_API_KEY}");
    const hasHardcoded = /AIza[0-9A-Za-z_-]{20,}/.test(content);

    // Assert
    expect(content.length).toBeGreaterThan(0);
    expect(hasPlaceholder).toBe(true);
    expect(hasHardcoded).toBe(false);
  });

  it("nginx config has location /internal/ {return 404;}", async () => {
    // Arrange
    const nginxPath = path.resolve(__dirname, "../../../infra/nginx/nginx.conf");
    const altPath = path.resolve(process.cwd(), "infra/nginx/nginx.conf");
    const p = fs.existsSync(nginxPath) ? nginxPath : altPath;
    const content = fs.existsSync(p) ? fs.readFileSync(p, "utf-8") : "";

    // Act
    const hasInternal404 = content.includes("location /internal/") && content.includes("return 404");
    const hasChatProxy = content.includes("location /api/chat") && content.includes("proxy_pass");
    const hasBufferingOff = content.includes("proxy_buffering off");

    // Assert
    expect(hasInternal404).toBe(true);
    expect(hasChatProxy).toBe(true);
    expect(hasBufferingOff).toBe(true);
  });

  it("compose agent service exists with 127.0.0.1 bindings", async () => {
    // Arrange
    const composePath = path.resolve(__dirname, "../../../docker-compose.yml");
    const altPath = path.resolve(process.cwd(), "docker-compose.yml");
    const p = fs.existsSync(composePath) ? composePath : altPath;
    const content = fs.existsSync(p) ? fs.readFileSync(p, "utf-8") : "";

    // Act
    const hasAgent = content.includes("agent:");
    const hasTargetAgent = content.includes("target: agent") || content.includes("target: agent");
    const hasGeminiEnv = content.includes("GEMINI_API_KEY") && content.includes("GEMINI_MODEL");
    const has127 = content.includes("127.0.0.1:");

    // Assert
    expect(hasAgent).toBe(true);
    expect(hasTargetAgent).toBe(true);
    expect(hasGeminiEnv).toBe(true);
    expect(has127).toBe(true);
  });
});
