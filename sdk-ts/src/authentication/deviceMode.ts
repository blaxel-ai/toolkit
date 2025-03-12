import { oauthToken } from "../client/authentication";
import { CredentialsType } from "./types";

export class DeviceMode {
    private refreshToken: string;
    private deviceCode: string;
    private accessToken: string;
    // private expireIn: number;
    constructor(credentials: CredentialsType) {
        this.refreshToken = credentials.refresh_token || ''
        this.deviceCode = credentials.device_code || ''
        this.accessToken = credentials.access_token || ''
        // this.expireIn = credentials.expires_in || 7200
    }

    async authenticate() {
        await this.refresh()
    }

    async refresh() {
        // TODO: Implement expire usage, to avoid calling the API every time we refresh the token
        const response = await oauthToken({
            body: {
                grant_type: 'refresh_token',
                device_code: this.deviceCode,
                refresh_token: this.refreshToken
            }
        })
        if(response.error) {
            throw new Error(response.error.error)
        }
        this.accessToken = response.data?.access_token || ''
        // this.expireIn = response.data?.expires_in || 7200
    }

    get authorization() {
        return `Bearer ${this.accessToken}`
    }
}
