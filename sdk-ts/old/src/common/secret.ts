/**
 * Secret management class
 */
export class Secret {
  /**
   * Get a secret value from environment variables
   * @param name Secret name
   * @returns Secret value or undefined if not found
   */
  static get(name: string): string | undefined {
    return process.env[name] || process.env[`bl_${name}`];
  }

  /**
   * Set a secret value in environment variables
   * @param name Secret name
   * @param value Secret value
   */
  static set(name: string, value: string): void {
    process.env[name] = value;
  }
}
