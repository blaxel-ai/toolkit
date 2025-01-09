import {
  EnvironmentMetadata,
  Function,
  ListAgentsResponse,
  ListEnvironmentsResponse,
  ListFunctionsResponse,
  ListIntegrationConnectionsResponse,
  ListModelsResponse,
  ListPoliciesResponse,
} from "@beamlit/sdk/src/client/types.gen";
import path from "path";
import * as vscode from "vscode";
import { BeamlitWorkspaceProvider } from "./beamlitWorkspaceProvider";

export class BeamlitExplorer implements vscode.TreeDataProvider<ResourceNode> {
  private _onDidChangeTreeData = new vscode.EventEmitter<
    ResourceNode | undefined
  >();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  constructor(private resourceProvider: BeamlitWorkspaceProvider) {}

  refresh(): void {
    this._onDidChangeTreeData.fire(undefined);
  }

  getTreeItem(element: ResourceNode): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: ResourceNode): Promise<ResourceNode[]> {
    if (!element) {
      // Root level - show resources
      const resources = await this.resourceProvider.getResourceTypes();
      return resources.map((resource) => {
        return new ResourceNode(
          resource.name,
          resource.id,
          resource.description
        );
      });
    }
    let resourceType = `${element.resourceType}`;
    // Child level - show resources for selected resource
    if (resourceType === "functions-env") {
      resourceType = "functions";
    }
    if (resourceType === "models-env") {
      resourceType = "models";
    }
    if (resourceType === "agents-env") {
      resourceType = "agents";
    }

    const resources = await this.resourceProvider.getResources(resourceType);
    if (
      element.resourceType === "functions" ||
      element.resourceType === "models" ||
      element.resourceType === "agents"
    ) {
      return this.getEnvironmentResources(
        element.resourceType,
        resources as
          | ListAgentsResponse
          | ListModelsResponse
          | ListFunctionsResponse
      );
    }
    return this.handleResource(resources, resourceType, element);
  }

  async getEnvironmentResources(
    resourceType: string,
    resources: ListAgentsResponse | ListModelsResponse | ListFunctionsResponse
  ) {
    // For functions, group by environment
    const environments = await this.resourceProvider.getResources(
      "environments"
    );

    return environments.map((env) => {
      const envName = env.metadata?.name || "";
      const envFunctions = resources.filter((resource) => {
        if (resource as Function) {
          return (
            resource.metadata &&
            (resource.metadata as EnvironmentMetadata).environment === envName
          );
        }
        return false;
      });

      return new ResourceNode(
        envName,
        resourceType + "-env",
        `${envFunctions.length} ${resourceType}`,
        "environments"
      );
    });
  }

  handleResource(
    resources:
      | ListAgentsResponse
      | ListModelsResponse
      | ListFunctionsResponse
      | ListEnvironmentsResponse
      | ListPoliciesResponse
      | ListIntegrationConnectionsResponse,
    resourceType: string,
    element: ResourceNode
  ) {
    return resources
      .filter((resource) => {
        if (
          resourceType === "functions" ||
          resourceType === "models" ||
          resourceType === "agents"
        ) {
          return (
            resource.metadata &&
            element.label ===
              (resource.metadata as EnvironmentMetadata).environment
          );
        }
        return true;
      })
      .map(
        (resource) =>
          new ResourceTypeNode(
            resource.metadata?.name ?? resource.metadata?.displayName ?? "",
            resourceType,
            "",
            resource.metadata &&
              (resource.metadata as EnvironmentMetadata).environment
          )
      );
  }
}

class ResourceNode extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly resourceType: string,
    public readonly description: string,
    public readonly forceIcon?: string
  ) {
    super(label, vscode.TreeItemCollapsibleState.Collapsed);
    this.contextValue = "resource";
    this.description = description;
    const icon = forceIcon || resourceType;
    this.iconPath = {
      light: vscode.Uri.file(
        path.join(__dirname, "..", "resources", icon + "-light.svg")
      ),
      dark: vscode.Uri.file(
        path.join(__dirname, "..", "resources", icon + "-dark.svg")
      ),
    };
  }
}

class ResourceTypeNode extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly resourceType: string,
    public readonly description: string,
    public readonly environment?: string
  ) {
    super(label, vscode.TreeItemCollapsibleState.None);
    this.contextValue = "resource";
    this.description = description;
    this.iconPath = {
      light: vscode.Uri.file(
        path.join(__dirname, "..", "resources", "file-light.svg")
      ),
      dark: vscode.Uri.file(
        path.join(__dirname, "..", "resources", "file-dark.svg")
      ),
    };
    this.command = {
      command: "beamlit.selectResource",
      title: "Select Resource",
      arguments: [this.resourceType, this.label, this.environment],
    };
  }
}
