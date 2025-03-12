/**
 * Converts a string into a URL-friendly slug.
 * @param name - The string to slugify.
 * @returns The slugified string.
 */
export function slugify(name: string): string {
  return name.toLowerCase().replaceAll(" ", "-").replaceAll("_", "-");
}
