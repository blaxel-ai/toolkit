import authentication from "../authentication";
import { Credentials } from "../authentication/credentials";
class Settings {
    credentials: Credentials;

    constructor() {
        this.credentials = authentication()
    }

    get env() {
        return process.env.BL_ENV || "prod";
    }

    get baseUrl() {
        if (this.env === "prod") {
            return "https://api.blaxel.ai/v0";
        }
        return "https://api.blaxel.dev/v0";
    }

    get workspace() {
        return process.env.BL_WORKSPACE || null;
    }

    get authorization() {
        return this.credentials.authorization;
    }

    get name() {
        return process.env.BL_NAME || "";
    }

    async authenticate() {
        await this.credentials.authenticate();
    }
}

export default new Settings();