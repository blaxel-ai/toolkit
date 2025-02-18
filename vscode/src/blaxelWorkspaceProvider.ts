import {
  init,
  listAgents,
  ListAgentsResponse,
  listFunctions,
  ListFunctionsResponse,
  listIntegrationConnections,
  ListIntegrationConnectionsResponse,
  listModels,
  ListModelsResponse,
  listPolicies,
  ListPoliciesResponse,
  newClient,
} from "@blaxel/sdk";

export class BlaxelWorkspaceProvider {
  private functions: ListFunctionsResponse;
  private models: ListModelsResponse;
  private agents: ListAgentsResponse;
  private policies: ListPoliciesResponse;
  private integrations: ListIntegrationConnectionsResponse;

  constructor() {
    this.functions = [];
    this.models = [];
    this.agents = [];
    this.policies = [];
    this.integrations = [];
    init();
  }

  async getResourceTypes() {
    return [
      { name: "Agents", id: "agents", description: "Agents" },
      { name: "Model APIs", id: "models", description: "Model APIs" },
      { name: "Functions", id: "functions", description: "Functions" },
      { name: "Policies", id: "policies", description: "Policies" },
      { name: "Integrations", id: "integrations", description: "Integrations" },
    ];
  }

  async refreshResources() {
    // We reset the client in case users have changed their workspace
    const client = newClient();
    const responseAgents = await listAgents({
      client,
      throwOnError: true,
    });
    const responseModels = await listModels({
      client,
      throwOnError: true,
    });
    const responseFunctions = await listFunctions({
      client,
      throwOnError: true,
    });
    const responsePolicies = await listPolicies({
      client,
      throwOnError: true,
    });
    const responseIntegrations = await listIntegrationConnections({
      client,
      throwOnError: true,
    });
    this.agents = responseAgents.data ?? [];
    this.models = responseModels.data ?? [];
    this.functions = responseFunctions.data ?? [];
    this.policies = responsePolicies.data ?? [];
    this.integrations = responseIntegrations.data ?? [];
  }

  async getResources(
    type: string
  ): Promise<
    | ListAgentsResponse
    | ListModelsResponse
    | ListFunctionsResponse
    | ListPoliciesResponse
    | ListIntegrationConnectionsResponse
  > {
    if (type === "agents") {
      return this.agents;
    } else if (type === "models") {
      return this.models;
    } else if (type === "functions") {
      return this.functions;
    } else if (type === "policies") {
      return this.policies;
    } else if (type === "integrations") {
      return this.integrations;
    }
    return [];
  }
}
