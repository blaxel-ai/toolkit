import { beforeEach, describe, expect, it } from "@jest/globals";
import { init } from "../src/common";
import { wrapFunction } from "../src/functions/base";

describe("Functions", () => {
  beforeEach(() => {
    init();
  });
  it("should wrap function with parameters comming from a zod schema", async () => {
    const func = (a: number, b: number) => {
      return a + b;
    };
    const parameters = [
      {
        name: "a",
        description: "description different",
        required: true,
        type: "number",
      },
      {
        name: "b",
        description: "description different",
        required: true,
        type: "number",
      },
    ];
    const wrapped = await wrapFunction(func, {
      description: "Add two numbers",
      parameters,
    });
    expect(wrapped.function?.spec?.description).toBe("Add two numbers");
    expect(wrapped.function?.spec?.parameters).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ description: "description different" }),
        expect.objectContaining({ description: "description different" }),
      ])
    );
  });
});
