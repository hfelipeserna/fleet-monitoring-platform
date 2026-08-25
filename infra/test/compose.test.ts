// Covers [SPEC-003: AC-006, BR-007, BR-008]
import { describe, it, expect } from "vitest";
import fs from "fs";
import path from "path";

describe("Compose LB + nginx guard", () => {
  // Covers [SPEC-003: AC-006, BR-007]

  it("docker compose config no leak GEMINI_API_KEY", async () => {
    // Arrange
    const composePath = path.resolve(__dirname, "../../docker-compose.yml");
    const content = fs.readFileSync(composePath, "utf-8");

    // Act
    const hasPlaceholder = content.includes("${GEMINI_API_KEY}");
    const hasHardcodedKey = /GEMINI_API_KEY\s*:\s*["']?AIza[0-9A-Za-z_-]{20,}["']?/.test(content);
    const hasDefaultSecret = /GEMINI_API_KEY.*default|GEMINI_API_KEY:-/.test(content) && !content.includes("${GEMINI_API_KEY}");

    // Assert
    expect(hasPlaceholder).toBe(true);
    expect(hasHardcodedKey).toBe(false);
    expect(hasDefaultSecret).toBe(false);
    expect(content).not.toMatch(/AIza[0-9A-Za-z_-]{35}/);
  });

  it("nginx config has location /internal/ {return 404;}", async () => {
    // Arrange
    const nginxPath = path.resolve(__dirname, "../nginx/nginx.conf");
    const content = fs.readFileSync(nginxPath, "utf-8");

    // Act
    const hasInternal404 = content.includes("location /internal/") && content.includes("return 404");
    const hasProxyBufferingOff = content.includes("proxy_buffering off");
    const hasChatProxy = content.includes("location /api/chat") || content.includes("location /api");

    // Assert
    expect(hasInternal404).toBe(true);
    // proxy_buffering off es requerido para /api/chat streaming
    expect(hasProxyBufferingOff).toBe(true);
    expect(hasChatProxy).toBe(true);
  });

  it("compose agent service 127.0.0.1 bindings + healthcheck", async () => {
    // Arrange
    const composePath = path.resolve(__dirname, "../../docker-compose.yml");
    const content = fs.readFileSync(composePath, "utf-8");

    // Act
    const hasAgent = content.includes("agent:");
    const hasBindings = content.includes("127.0.0.1:");
    const hasHealthcheck = content.includes("healthcheck:");

    // Assert
    // Step 6 debe añadir agent con build target agent y bindings 127.0.0.1
    // Este test fallará hasta que Step 6 implemente agent
    expect(hasAgent).toBe(true);
    expect(hasBindings).toBe(true);
    expect(hasHealthcheck).toBe(true);
  });
});
