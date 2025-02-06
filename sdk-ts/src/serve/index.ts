import { instrumentApp } from "../common/instrumentation.js";
import { createApp, runApp } from "./app.js";

/**
 * Initializes and runs the Fastify application.
 */
createApp()
  .then((app) => runApp(app))
  .catch((err) => console.error(err));
