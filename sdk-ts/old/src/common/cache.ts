export async function handleControlplaneCache(
  req: Request
): Promise<Response | null> {
  if (req.method !== "GET") {
    return null;
  }
  const allowedHosts = new Set(["api.blaxel.ai", "api.blaxel.dev"]);
  const url = new URL(req.url);
  if (!allowedHosts.has(url.hostname)) {
    return null;
  }

  const pathSegments = req.url.split("/");
  if (pathSegments.length > 6) {
    return null;
  }
  const objectType = pathSegments[4];
  const name = pathSegments[5];
  const requirePath = `${process.cwd()}/.blaxel/cache/${objectType}/${name}.json`;

  let fs;
  try {
    fs = await import("fs");
  } catch {
    return null;
  }

  try {
    const cache = fs.readFileSync(requirePath, "utf8");
    return new Response(cache, {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  } catch {
    return null;
  }
}
