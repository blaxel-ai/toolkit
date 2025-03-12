/**
 * Represents an HTTP error with a status code and message.
 */
export class HTTPError extends Error {
  /**
   * Constructs a new HTTPError instance.
   * @param status_code - The HTTP status code.
   * @param message - The error message.
   */
  constructor(public status_code: number, public message: string) {
    super(message);
    // Set the prototype explicitly to maintain proper inheritance
    Object.setPrototypeOf(this, HTTPError.prototype);
  }

  /**
   * Returns a string representation of the HTTPError.
   * @returns A string combining the status code and message.
   */
  toString(): string {
    return `${this.status_code} ${this.message}`;
  }
}
