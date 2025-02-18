from typing import TYPE_CHECKING, Any, TypeVar, Union, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.runtime_readiness_probe import RuntimeReadinessProbe
    from ..models.runtime_resources import RuntimeResources


T = TypeVar("T", bound="Runtime")


@_attrs_define
class Runtime:
    """Set of configurations for a deployment

    Attributes:
        args (Union[Unset, list[Any]]): The arguments to pass to the deployment runtime
        command (Union[Unset, list[Any]]): The command to run the deployment
        endpoint_name (Union[Unset, str]): Endpoint Name of the model. In case of hf_private_endpoint, it is the
            endpoint name. In case of hf_public_endpoint, it is not used.
        envs (Union[Unset, list[Any]]): The env variables to set in the deployment. Should be a list of Kubernetes
            EnvVar types
        image (Union[Unset, str]): The Docker image for the deployment
        metric_port (Union[Unset, int]): The port to serve the metrics on
        model (Union[Unset, str]): The slug name of the origin model at HuggingFace.
        organization (Union[Unset, str]): The organization of the model
        readiness_probe (Union[Unset, RuntimeReadinessProbe]): The readiness probe. Should be a Kubernetes Probe type
        resources (Union[Unset, RuntimeResources]): The resources for the deployment. Should be a Kubernetes
            ResourceRequirements type
        serving_port (Union[Unset, int]): The port to serve the model on
        type_ (Union[Unset, str]): The type of origin for the deployment (hf_private_endpoint, hf_public_endpoint)
    """

    args: Union[Unset, list[Any]] = UNSET
    command: Union[Unset, list[Any]] = UNSET
    endpoint_name: Union[Unset, str] = UNSET
    envs: Union[Unset, list[Any]] = UNSET
    image: Union[Unset, str] = UNSET
    metric_port: Union[Unset, int] = UNSET
    model: Union[Unset, str] = UNSET
    organization: Union[Unset, str] = UNSET
    readiness_probe: Union[Unset, "RuntimeReadinessProbe"] = UNSET
    resources: Union[Unset, "RuntimeResources"] = UNSET
    serving_port: Union[Unset, int] = UNSET
    type_: Union[Unset, str] = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        args: Union[Unset, list[Any]] = UNSET
        if not isinstance(self.args, Unset):
            args = self.args

        command: Union[Unset, list[Any]] = UNSET
        if not isinstance(self.command, Unset):
            command = self.command

        endpoint_name = self.endpoint_name

        envs: Union[Unset, list[Any]] = UNSET
        if not isinstance(self.envs, Unset):
            envs = self.envs

        image = self.image

        metric_port = self.metric_port

        model = self.model

        organization = self.organization

        readiness_probe: Union[Unset, dict[str, Any]] = UNSET
        if (
            self.readiness_probe
            and not isinstance(self.readiness_probe, Unset)
            and not isinstance(self.readiness_probe, dict)
        ):
            readiness_probe = self.readiness_probe.to_dict()
        elif self.readiness_probe and isinstance(self.readiness_probe, dict):
            readiness_probe = self.readiness_probe

        resources: Union[Unset, dict[str, Any]] = UNSET
        if self.resources and not isinstance(self.resources, Unset) and not isinstance(self.resources, dict):
            resources = self.resources.to_dict()
        elif self.resources and isinstance(self.resources, dict):
            resources = self.resources

        serving_port = self.serving_port

        type_ = self.type_

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if args is not UNSET:
            field_dict["args"] = args
        if command is not UNSET:
            field_dict["command"] = command
        if endpoint_name is not UNSET:
            field_dict["endpointName"] = endpoint_name
        if envs is not UNSET:
            field_dict["envs"] = envs
        if image is not UNSET:
            field_dict["image"] = image
        if metric_port is not UNSET:
            field_dict["metricPort"] = metric_port
        if model is not UNSET:
            field_dict["model"] = model
        if organization is not UNSET:
            field_dict["organization"] = organization
        if readiness_probe is not UNSET:
            field_dict["readinessProbe"] = readiness_probe
        if resources is not UNSET:
            field_dict["resources"] = resources
        if serving_port is not UNSET:
            field_dict["servingPort"] = serving_port
        if type_ is not UNSET:
            field_dict["type"] = type_

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: dict[str, Any]) -> T:
        from ..models.runtime_readiness_probe import RuntimeReadinessProbe
        from ..models.runtime_resources import RuntimeResources

        if not src_dict:
            return None
        d = src_dict.copy()
        args = cast(list[Any], d.pop("args", UNSET))

        command = cast(list[Any], d.pop("command", UNSET))

        endpoint_name = d.pop("endpointName", UNSET)

        envs = cast(list[Any], d.pop("envs", UNSET))

        image = d.pop("image", UNSET)

        metric_port = d.pop("metricPort", UNSET)

        model = d.pop("model", UNSET)

        organization = d.pop("organization", UNSET)

        _readiness_probe = d.pop("readinessProbe", UNSET)
        readiness_probe: Union[Unset, RuntimeReadinessProbe]
        if isinstance(_readiness_probe, Unset):
            readiness_probe = UNSET
        else:
            readiness_probe = RuntimeReadinessProbe.from_dict(_readiness_probe)

        _resources = d.pop("resources", UNSET)
        resources: Union[Unset, RuntimeResources]
        if isinstance(_resources, Unset):
            resources = UNSET
        else:
            resources = RuntimeResources.from_dict(_resources)

        serving_port = d.pop("servingPort", UNSET)

        type_ = d.pop("type", UNSET)

        runtime = cls(
            args=args,
            command=command,
            endpoint_name=endpoint_name,
            envs=envs,
            image=image,
            metric_port=metric_port,
            model=model,
            organization=organization,
            readiness_probe=readiness_probe,
            resources=resources,
            serving_port=serving_port,
            type_=type_,
        )

        runtime.additional_properties = d
        return runtime

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
