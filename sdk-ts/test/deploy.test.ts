import { afterEach, beforeEach, describe, expect, it } from "@jest/globals";
import fs from "fs";
import * as yaml from "js-yaml";
import { Agent } from "../src";
import { init, logger } from "../src/common";
import { generateBlaxelDeployment } from "../src/deploy/deploy";

const functionsDir = "tmp/functions";
const agentsDir = "tmp";

describe("Functions", () => {
  beforeEach(() => {
    process.env.BL_AGENT_FUNCTIONS_DIRECTORY = functionsDir;
    process.env.BL_SERVER_MODULE = "agent.agent";
    process.env.BL_SERVER_DIRECTORY = "tmp";
    process.env.BL_DEPLOY = "true";
    init();
  });
  afterEach(() => {
    delete process.env.BL_AGENT_FUNCTIONS_DIRECTORY;
    delete process.env.BL_SERVER_DIRECTORY;
    delete process.env.BL_DEPLOY;
    delete process.env.BL_SERVER_MODULE;

    try {
      fs.rmSync(`tmp`, { recursive: true });
      fs.rmSync(`.blaxel`, { recursive: true });
    } catch (e) {
      logger.error(e);
    }
  });
  it("should create a dockerfile", async () => {
    const dir = `${__dirname}/..`;
    if (!fs.existsSync(`${dir}/tmp`)) {
      fs.mkdirSync(`${dir}/tmp`);
    }
    if (!fs.existsSync(`${dir}/${functionsDir}`)) {
      fs.mkdirSync(`${dir}/${functionsDir}`);
    }
    fs.writeFileSync(
      `${dir}/${functionsDir}/add.ts`,
      `
import { wrapFunction } from "../../src/functions/base";
export const add = wrapFunction(function add(a: number, b: number) { return a + b; }, { description: "Add two numbers", parameters: [{ name: "a", type: "number" }, { name: "b", type: "number" }] })
export const sub = wrapFunction(function sub(a: number, b: number) { return a - b; }, { description: "Subtract two numbers", parameters: [{ name: "a", type: "number" }, { name: "b", type: "number" }] })
    `
    );

    fs.writeFileSync(
      `${dir}/${agentsDir}/agent.ts`,
      `
import { wrapAgent } from "../src/agents/base";
export const agent = wrapAgent((r) => { return; }, { agent: { metadata: { name: "agent-math" }, spec: { model: "gpt-4o-mini" }}, remoteFunctions: ["search"] })
    `
    );

    fs.writeFileSync(
      `${dir}/${agentsDir}/agent_custom.ts`,
      `
import { wrapAgent } from "../src/agents/base";
export const agent = async () => wrapAgent((r) => { return; }, { agent: { metadata: { name: "agent-custom" }, spec: { model: "gpt-4o-mini" }}})
        `
    );
    await generateBlaxelDeployment(".blaxel");
    expect(fs.existsSync(`.blaxel/agents/agent-math/agent.yaml`)).toBe(true);
    expect(fs.existsSync(`.blaxel/agents/agent-custom/agent.yaml`)).toBe(true);
    expect(fs.existsSync(`.blaxel/functions/add/function.yaml`)).toBe(true);
    expect(fs.existsSync(`.blaxel/functions/sub/function.yaml`)).toBe(true);

    // Read the YAML file for agent-math and verify remoteFunctions contains ["search"]
    const agentMathYamlPath = `.blaxel/agents/agent-math/agent.yaml`;
    const agentMathYamlContent = fs.readFileSync(agentMathYamlPath, "utf8");
    const agentMathConfig = yaml.load(agentMathYamlContent) as Agent;

    // Assuming the YAML structure nests the configuration under an "agent" key,
    // check that its spec has a remoteFunctions property with the expected value.
    expect(agentMathConfig?.spec?.functions).toContain("search");
  });
});
