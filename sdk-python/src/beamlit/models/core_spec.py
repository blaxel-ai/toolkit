from typing import TYPE_CHECKING, Any, TypeVar, Union, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.core_spec_configurations import CoreSpecConfigurations
    from ..models.flavor import Flavor
    from ..models.model_private_cluster import ModelPrivateCluster
    from ..models.pod_template_spec import PodTemplateSpec
    from ..models.runtime import Runtime
    from ..models.serverless_config import ServerlessConfig


T = TypeVar("T", bound="CoreSpec")


@_attrs_define
class CoreSpec:
    """Core specification

    Attributes:
        configurations (Union[Unset, CoreSpecConfigurations]): Optional configurations for the object
        enabled (Union[Unset, bool]): Enable or disable the agent
        flavors (Union[Unset, list['Flavor']]): Types of hardware available for deployments
        integration_connections (Union[Unset, list[str]]):
        pod_template (Union[Unset, PodTemplateSpec]): Pod template specification
        policies (Union[Unset, list[str]]):
        private_clusters (Union[Unset, ModelPrivateCluster]): Private cluster where the model deployment is deployed
        runtime (Union[Unset, Runtime]): Set of configurations for a deployment
        sandbox (Union[Unset, bool]): Sandbox mode
        serverless_config (Union[Unset, ServerlessConfig]): Configuration for a serverless deployment
    """

    configurations: Union[Unset, "CoreSpecConfigurations"] = UNSET
    enabled: Union[Unset, bool] = UNSET
    flavors: Union[Unset, list["Flavor"]] = UNSET
    integration_connections: Union[Unset, list[str]] = UNSET
    pod_template: Union[Unset, "PodTemplateSpec"] = UNSET
    policies: Union[Unset, list[str]] = UNSET
    private_clusters: Union[Unset, "ModelPrivateCluster"] = UNSET
    runtime: Union[Unset, "Runtime"] = UNSET
    sandbox: Union[Unset, bool] = UNSET
    serverless_config: Union[Unset, "ServerlessConfig"] = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        configurations: Union[Unset, dict[str, Any]] = UNSET
        if self.configurations and not isinstance(self.configurations, Unset):
            configurations = self.configurations.to_dict()

        enabled = self.enabled

        flavors: Union[Unset, list[dict[str, Any]]] = UNSET
        if not isinstance(self.flavors, Unset):
            flavors = []
            for componentsschemas_flavors_item_data in self.flavors:
                componentsschemas_flavors_item = componentsschemas_flavors_item_data.to_dict()
                flavors.append(componentsschemas_flavors_item)

        integration_connections: Union[Unset, list[str]] = UNSET
        if not isinstance(self.integration_connections, Unset):
            integration_connections = self.integration_connections

        pod_template: Union[Unset, dict[str, Any]] = UNSET
        if self.pod_template and not isinstance(self.pod_template, Unset):
            pod_template = self.pod_template.to_dict()

        policies: Union[Unset, list[str]] = UNSET
        if not isinstance(self.policies, Unset):
            policies = self.policies

        private_clusters: Union[Unset, dict[str, Any]] = UNSET
        if self.private_clusters and not isinstance(self.private_clusters, Unset):
            private_clusters = self.private_clusters.to_dict()

        runtime: Union[Unset, dict[str, Any]] = UNSET
        if self.runtime and not isinstance(self.runtime, Unset):
            runtime = self.runtime.to_dict()

        sandbox = self.sandbox

        serverless_config: Union[Unset, dict[str, Any]] = UNSET
        if self.serverless_config and not isinstance(self.serverless_config, Unset):
            serverless_config = self.serverless_config.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if configurations is not UNSET:
            field_dict["configurations"] = configurations
        if enabled is not UNSET:
            field_dict["enabled"] = enabled
        if flavors is not UNSET:
            field_dict["flavors"] = flavors
        if integration_connections is not UNSET:
            field_dict["integrationConnections"] = integration_connections
        if pod_template is not UNSET:
            field_dict["podTemplate"] = pod_template
        if policies is not UNSET:
            field_dict["policies"] = policies
        if private_clusters is not UNSET:
            field_dict["privateClusters"] = private_clusters
        if runtime is not UNSET:
            field_dict["runtime"] = runtime
        if sandbox is not UNSET:
            field_dict["sandbox"] = sandbox
        if serverless_config is not UNSET:
            field_dict["serverlessConfig"] = serverless_config

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: dict[str, Any]) -> T:
        from ..models.core_spec_configurations import CoreSpecConfigurations
        from ..models.flavor import Flavor
        from ..models.model_private_cluster import ModelPrivateCluster
        from ..models.pod_template_spec import PodTemplateSpec
        from ..models.runtime import Runtime
        from ..models.serverless_config import ServerlessConfig

        if not src_dict:
            return None
        d = src_dict.copy()
        _configurations = d.pop("configurations", UNSET)
        configurations: Union[Unset, CoreSpecConfigurations]
        if isinstance(_configurations, Unset):
            configurations = UNSET
        else:
            configurations = CoreSpecConfigurations.from_dict(_configurations)

        enabled = d.pop("enabled", UNSET)

        flavors = []
        _flavors = d.pop("flavors", UNSET)
        for componentsschemas_flavors_item_data in _flavors or []:
            componentsschemas_flavors_item = Flavor.from_dict(componentsschemas_flavors_item_data)

            flavors.append(componentsschemas_flavors_item)

        integration_connections = cast(list[str], d.pop("integrationConnections", UNSET))

        _pod_template = d.pop("podTemplate", UNSET)
        pod_template: Union[Unset, PodTemplateSpec]
        if isinstance(_pod_template, Unset):
            pod_template = UNSET
        else:
            pod_template = PodTemplateSpec.from_dict(_pod_template)

        policies = cast(list[str], d.pop("policies", UNSET))

        _private_clusters = d.pop("privateClusters", UNSET)
        private_clusters: Union[Unset, ModelPrivateCluster]
        if isinstance(_private_clusters, Unset):
            private_clusters = UNSET
        else:
            private_clusters = ModelPrivateCluster.from_dict(_private_clusters)

        _runtime = d.pop("runtime", UNSET)
        runtime: Union[Unset, Runtime]
        if isinstance(_runtime, Unset):
            runtime = UNSET
        else:
            runtime = Runtime.from_dict(_runtime)

        sandbox = d.pop("sandbox", UNSET)

        _serverless_config = d.pop("serverlessConfig", UNSET)
        serverless_config: Union[Unset, ServerlessConfig]
        if isinstance(_serverless_config, Unset):
            serverless_config = UNSET
        else:
            serverless_config = ServerlessConfig.from_dict(_serverless_config)

        core_spec = cls(
            configurations=configurations,
            enabled=enabled,
            flavors=flavors,
            integration_connections=integration_connections,
            pod_template=pod_template,
            policies=policies,
            private_clusters=private_clusters,
            runtime=runtime,
            sandbox=sandbox,
            serverless_config=serverless_config,
        )

        core_spec.additional_properties = d
        return core_spec

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
