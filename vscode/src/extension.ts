// The module 'vscode' contains the VS Code extensibility API
// Import the module and reference it with the alias vscode in your code below
import * as vscode from "vscode";
import { BeamlitExplorer } from "./beamlitExplorer";
import { BeamlitWorkspaceProvider } from "./beamlitWorkspaceProvider";
import { BeamlitResourceVirtualFileSystemProvider } from "./beamlitresource.virtualfs";

// This method is called when your extension is activated
// Your extension is activated the very first time the command is executed
export function activate(context: vscode.ExtensionContext) {
  // Create and register the resource data provider
  const resourceProvider = new BeamlitWorkspaceProvider();
  const treeDataProvider = new BeamlitExplorer(resourceProvider);

  resourceProvider
    .refreshResources()
    .then(() => treeDataProvider.refresh())
    .catch((err) => {
      vscode.window.showErrorMessage(
        `Beamlit: Failed to refresh resources because ${err.message}`
      );
    });

  // Register the TreeDataProvider
  const treeView = vscode.window.createTreeView("extension.vsBeamlitExplorer", {
    treeDataProvider: treeDataProvider,
    showCollapseAll: true,
  });
  const resourceDocProvider = new BeamlitResourceVirtualFileSystemProvider();

  // Register commands
  context.subscriptions.push(
    vscode.commands.registerCommand("beamlit.refresh", () =>
      refresh(treeDataProvider, resourceProvider)
    ),
    vscode.commands.registerCommand("beamlit.selectResource", selectResource),
    treeView,
    vscode.workspace.registerFileSystemProvider(
      "beamlit",
      resourceDocProvider,
      { isReadonly: true }
    )
  );
}

async function selectResource(
  resourceType: string,
  resourceId: string,
  environment?: string
) {
  try {
    const uri = vscode.Uri.parse(
      `beamlit://${resourceId}/${resourceId}.yaml?resourceType=${resourceType}&resourceId=${resourceId}&environment=${environment}`
    );
    const doc = await vscode.workspace.openTextDocument(uri);
    await vscode.window.showTextDocument(doc);
  } catch (err) {
    vscode.window.showErrorMessage(
      `Beamlit: Failed to select resource because ${(err as Error).message}`
    );
  }
}

async function refresh(
  treeDataProvider: BeamlitExplorer,
  resourceProvider: BeamlitWorkspaceProvider
) {
  const statusBarMessage = vscode.window.setStatusBarMessage(
    "Refreshing resources..."
  );
  resourceProvider
    .refreshResources()
    .then(() => treeDataProvider.refresh())
    .catch((err) => {
      vscode.window.showErrorMessage(
        `Beamlit: Failed to refresh resources because ${err.message}`
      );
    })
    .finally(() => {
      statusBarMessage.dispose();
    });
}

// This method is called when your extension is deactivated
export function deactivate() {}
