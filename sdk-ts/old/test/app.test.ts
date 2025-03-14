import { describe, expect, it } from "@jest/globals";
import fs from "fs";
import path from "path";
import { createApp } from "../src/serve/app";

describe("App", () => {
  it("should create app with default function and return test in body", async () => {
    const func = () => {
      return "test";
    };
    const app = await createApp(func);
    const response = await app.inject({
      method: "POST",
      url: "/",
    });
    expect(response.statusCode).toBe(200);
    expect(response.body).toBe("test");
  });

  it("should create app with async function and return test in body", async () => {
    const func = async () => {
      return "test";
    };
    const app = await createApp(func);
    const response = await app.inject({
      method: "POST",
      url: "/",
    });
    expect(response.statusCode).toBe(200);
    expect(response.body).toBe("test");
  });

  it("should create app  from main.ts return test in body", async () => {
    process.env.BL_SERVER_MODULE = "main.agent";
    const mainFile = path.join(__dirname, "../main.ts");
    fs.writeFileSync(
      mainFile,
      'export default function main() { return "test"; }'
    );
    try {
      const app = await createApp();
      const response = await app.inject({
        method: "POST",
        url: "/",
      });
      expect(response.statusCode).toBe(200);
      expect(response.body).toBe("test");
    } finally {
      fs.unlinkSync(mainFile);
    }
  });

  it("should create app  from an agent", async () => {
    process.env.BL_SERVER_MODULE = "main.agent";
    const mainFile = path.join(__dirname, "../main.ts");
    fs.writeFileSync(
      mainFile,
      `
import { wrapAgent } from "./src/agents";
export default wrapAgent(async () => { return "test"; }, {overrideAgent: true});
    `
    );
    const app = await createApp();
    const response = await app.inject({
      method: "POST",
      url: "/",
    });
    expect(response.statusCode).toBe(200);
    expect(response.body).toBe("test");
    delete process.env.BL_SERVER_MODULE;
    fs.unlinkSync(mainFile);
  });
});
