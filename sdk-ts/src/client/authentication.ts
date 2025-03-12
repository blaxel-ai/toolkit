import { OptionsLegacyParser } from "@hey-api/client-fetch";
import { client } from "./index";

type OauthTokenData = {
  body: {
    grant_type: 'client_credentials' | 'refresh_token' | 'device_code';
    client_id?: string;
    client_secret?: string;
    device_code?: string;
    refresh_token?: string;
  },
  _bypassDelay?: boolean;
}

type OauthTokenResponse = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  token_type: string;
}

type OauthTokenError = {
  error: string;
}

/**
 * Get a new oauth token
 */
export const oauthToken = <ThrowOnError extends boolean = false>(options: OptionsLegacyParser<OauthTokenData, ThrowOnError>) => {
  options._bypassDelay = true
  return (options?.client ?? client).post<OauthTokenResponse, OauthTokenError, ThrowOnError>({
    ...options,
    url: '/oauth/token',

  });
};
