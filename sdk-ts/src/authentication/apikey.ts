import { Credentials } from "./credentials";
import { CredentialsType } from "./types";

export class ApiKey extends Credentials {
    private apiKey: string;
    constructor(credentials: CredentialsType) {
        super();
        this.apiKey = credentials.apiKey || ''
    }

    get authorization() {
        return `Bearer ${this.apiKey}`
    }
}
