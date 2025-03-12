import { oauthToken } from "../client/authentication";
import { Credentials } from "./credentials";
import { CredentialsType } from "./types";

export class ClientCredentials extends Credentials {
    private clientCredentials: string;
    private token: string;
    constructor(credentials: CredentialsType) {
        super();
        this.clientCredentials = credentials.clientCredentials || '';
        this.token = ''
    }

    async authenticate() {
        await this.refresh()
    }

    async refresh() {
        const response = await oauthToken({
            headers: {
                'Authorization': `Basic ${this.clientCredentials}`
            },
            body: {
                grant_type: 'client_credentials'
            }
        })
        if(response.error) {
            throw new Error(response.error.error)
        }
        this.token = response.data?.access_token || ''
    }

    get authorization() {
        return `Bearer ${this.token}`
    }
    
}