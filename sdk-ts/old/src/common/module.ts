import { getSettings } from "./settings.js";

/**
 * Imports and returns a module based on the current settings.
 * @param module - The module path to import. If null, uses the setting's module path.
 * @returns The imported module or function.
 */
export function importModule(module: string | null = null): any {
  const settings = getSettings();
  module = module || settings.server.module.replaceAll(".", "/");
  const toRequire =
    process.cwd() + "/" + module.split("/").slice(0, -1).join("/");
  const main_module = require(toRequire); // eslint-disable-line
  const func = main_module[settings.server.module.split(".").slice(-1)[0]];
  if (func) return func;
  return main_module.default;
}
