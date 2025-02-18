// The module 'vscode' contains the VS Code extensibility API
// Import the module and reference it with the alias vscode in your code below
import * as vscode from "vscode";
import { BlaxelExplorer } from "./blaxelExplorer";
import { BlaxelWorkspaceProvider } from "./blaxelWorkspaceProvider";

// This method is called when your extension is activated
// Your extension is activated the very first time the command is executed
export function activate(context: vscode.ExtensionContext) {
  // Create and register the resource data provider
  const resourceProvider = new BlaxelWorkspaceProvider();
  const treeDataProvider = new BlaxelExplorer(resourceProvider);

  resourceProvider
    .refreshResources()
    .then(() => treeDataProvider.refresh())
    .catch((err) => {
      vscode.window.showErrorMessage(
        `Blaxel: Failed to refresh resources because ${err.message}`
      );
    });

  // Register the TreeDataProvider
  const treeView = vscode.window.createTreeView("extension.vsBlaxelExplorer", {
    treeDataProvider: treeDataProvider,
    showCollapseAll: true,
  });
  const resourceDocProvider = new BlaxelResourceVirtualFileSystemProvider();

  // Register commands
  context.subscriptions.push(
    vscode.commands.registerCommand("blaxel.refresh", () =>
      refresh(treeDataProvider, resourceProvider)
    ),
    vscode.commands.registerCommand("blaxel.selectResource", selectResource),
    treeView,
    vscode.workspace.registerFileSystemProvider(
      "blaxel",
      resourceDocProvider,
      { isReadonly: true }
    )
  );
}

async function selectResource(resourceType: string, resourceId: string) {
  try {
    const uri = vscode.Uri.parse(
      `blaxel://${resourceId}/${resourceId}.yaml?resourceType=${resourceType}&resourceId=${resourceId}`
    );
    const doc = await vscode.workspace.openTextDocument(uri);
    await vscode.window.showTextDocument(doc);
  } catch (err) {
    vscode.window.showErrorMessage(
      `Blaxel: Failed to select resource because ${(err as Error).message}`
    );
  }
}

async function refresh(
  treeDataProvider: BlaxelExplorer,
  resourceProvider: BlaxelWorkspaceProvider
) {
  const statusBarMessage = vscode.window.setStatusBarMessage(
    "Refreshing resources..."
  );
  resourceProvider
    .refreshResources()
    .then(() => treeDataProvider.refresh())
    .catch((err) => {
      vscode.window.showErrorMessage(
        `Blaxel: Failed to refresh resources because ${err.message}`
      );
    })
    .finally(() => {
      statusBarMessage.dispose();
    });
}

// This method is called when your extension is deactivated
export function deactivate() {}
