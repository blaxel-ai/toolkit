import { afterEach, beforeEach, describe, expect, it } from "@jest/globals";
import { init } from "../src/common";
import { getFunctions } from "../src/functions/common";
import fs from "fs";

const functionsDir = "tmp/functions";

describe("FunctionsCommon", () => {
  beforeEach(() => {
      process.env.BL_AGENT_FUNCTIONS_DIRECTORY = functionsDir;
      init();
    });
  afterEach(() => {
    delete process.env.BL_AGENT_FUNCTIONS_DIRECTORY;
    fs.rmSync(`${__dirname}/../${functionsDir}`, { recursive: true });
  });

  it("should find every function in the directory", async () => {
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
export const add = wrapFunction(function add(a: number, b: number) { return a + b; })
export const sub = wrapFunction(function sub(a: number, b: number) { return a - b; })
    `
    );
    const functions = await getFunctions();
    expect(functions.length).toBe(2);
    expect(functions).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name: "add" }),
        expect.objectContaining({ name: "sub" })
      ])
    );
  });
});
