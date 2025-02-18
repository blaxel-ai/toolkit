import {
  ListAgentsResponse,
  ListFunctionsResponse,
  ListIntegrationConnectionsResponse,
  ListModelsResponse,
  ListPoliciesResponse,
} from "@blaxel/sdk";
import path from "path";
import * as vscode from "vscode";
import { BlaxelWorkspaceProvider } from "./blaxelWorkspaceProvider";

export class BlaxelExplorer implements vscode.TreeDataProvider<ResourceNode> {
  private _onDidChangeTreeData = new vscode.EventEmitter<
    ResourceNode | undefined
  >();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  constructor(private resourceProvider: BlaxelWorkspaceProvider) {}

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

    const resources = await this.resourceProvider.getResources(
      element.resourceType
    );
    return this.handleResource(resources, element.resourceType, element);
  }

  handleResource(
    resources:
      | ListAgentsResponse
      | ListModelsResponse
      | ListFunctionsResponse
      | ListPoliciesResponse
      | ListIntegrationConnectionsResponse,
    resourceType: string,
    element: ResourceNode
  ) {
    return resources.map(
      (resource) =>
        new ResourceTypeNode(
          resource.metadata?.name ?? resource.metadata?.displayName ?? "",
          resourceType,
          ""
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
    public readonly description: string
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
      command: "blaxel.selectResource",
      title: "Select Resource",
      arguments: [this.resourceType, this.label],
    };
  }
}
