import { beforeEach, describe, expect, it } from "@jest/globals";
import fs from "fs";
import { Settings, init } from "../src/common/settings";
describe("Settings", () => {
  let settings: Settings;

  beforeEach(() => {
    process.env.BL_ENV = "";
    settings = init();
  });

  it("should initialize with default values", () => {
    expect(settings.authentication.apiKey).toBeNull();
    expect(settings.baseUrl).toBe("https://api.blaxel.ai/v0");
  });

  it("should handle boolean and number values", () => {
    process.env.BL_SERVER_PORT = "8080";
    process.env.BL_SERVER_HOST = "0.0.0.0";
    const checkSettings = init();
    expect(checkSettings.server.port).toBe(8080);
    expect(checkSettings.server.host).toBe("0.0.0.0");
    delete process.env.BL_SERVER_PORT;
    delete process.env.BL_SERVER_HOST;
  });

  it("should update nested settings from env", () => {
    process.env.BL_AUTHENTICATION_API_KEY = "test-api-key";
    process.env.BL_AGENT_FUNCTIONS_DIRECTORY = "test-agent-api-key";
    process.env.BL_SERVER_MODULE = "test-server-api-key";
    const checkSettings = init();
    expect(checkSettings.authentication.apiKey).toBe("test-api-key");
    expect(checkSettings.agent.functionsDirectory).toBe("test-agent-api-key");
    expect(checkSettings.server.module).toBe("test-server-api-key");
    delete process.env.BL_AUTHENTICATION_API_KEY;
    delete process.env.BL_AGENT_FUNCTIONS_DIRECTORY;
    delete process.env.BL_SERVER_MODULE;
  });

  it("should update settings from yaml settings", () => {
    fs.writeFileSync(
      "blaxel.yaml",
      `
workspace: main
authentication:
  jwt: test-jwt`
    );
    settings = init();
    expect(settings.workspace).toBe("main");
    expect(settings.authentication.jwt).toBe("test-jwt");
    fs.unlinkSync("blaxel.yaml");
  });

  it("should validate baseUrl format", () => {
    process.env.BL_BASE_URL = "fake-url";
    expect(() => {
      init();
    }).toThrow("Invalid URL format");
    delete process.env.BL_BASE_URL;
  });

  it("should validate baseUrl format", () => {
    process.env.BL_ENV = "dev";
    const checkSettings = init();
    expect(checkSettings.baseUrl).toBe("https://api.blaxel.dev/v0");
    expect(checkSettings.runUrl).toBe("https://run.blaxel.dev");
    expect(checkSettings.registryUrl).toBe("https://eu.registry.blaxel.dev");
    expect(checkSettings.appUrl).toBe("https://app.blaxel.dev");
    delete process.env.BL_ENV;
  });
});
