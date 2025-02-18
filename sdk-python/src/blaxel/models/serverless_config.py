from typing import Any, TypeVar, Union

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="ServerlessConfig")


@_attrs_define
class ServerlessConfig:
    """Configuration for a serverless deployment

    Attributes:
        last_pod_retention_period (Union[Unset, str]): The minimum amount of time that the last replica will remain
            active AFTER a scale-to-zero decision is made
        max_num_replicas (Union[Unset, int]): The maximum number of replicas for the deployment.
        metric (Union[Unset, str]): Metric watched to make scaling decisions. Can be "cpu" or "memory" or "rps" or
            "concurrency"
        min_num_replicas (Union[Unset, int]): The minimum number of replicas for the deployment. Can be 0 or 1 (in which
            case the deployment is always running in at least one location).
        scale_down_delay (Union[Unset, str]): The time window which must pass at reduced concurrency before a scale-down
            decision is applied. This can be useful, for example, to keep containers around for a configurable duration to
            avoid a cold start penalty if new requests come in.
        scale_up_minimum (Union[Unset, int]): The minimum number of replicas that will be created when the deployment
            scales up from zero.
        stable_window (Union[Unset, str]): The sliding time window over which metrics are averaged to provide the input
            for scaling decisions
        target (Union[Unset, str]): Target value for the watched metric
    """

    last_pod_retention_period: Union[Unset, str] = UNSET
    max_num_replicas: Union[Unset, int] = UNSET
    metric: Union[Unset, str] = UNSET
    min_num_replicas: Union[Unset, int] = UNSET
    scale_down_delay: Union[Unset, str] = UNSET
    scale_up_minimum: Union[Unset, int] = UNSET
    stable_window: Union[Unset, str] = UNSET
    target: Union[Unset, str] = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        last_pod_retention_period = self.last_pod_retention_period

        max_num_replicas = self.max_num_replicas

        metric = self.metric

        min_num_replicas = self.min_num_replicas

        scale_down_delay = self.scale_down_delay

        scale_up_minimum = self.scale_up_minimum

        stable_window = self.stable_window

        target = self.target

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if last_pod_retention_period is not UNSET:
            field_dict["lastPodRetentionPeriod"] = last_pod_retention_period
        if max_num_replicas is not UNSET:
            field_dict["maxNumReplicas"] = max_num_replicas
        if metric is not UNSET:
            field_dict["metric"] = metric
        if min_num_replicas is not UNSET:
            field_dict["minNumReplicas"] = min_num_replicas
        if scale_down_delay is not UNSET:
            field_dict["scaleDownDelay"] = scale_down_delay
        if scale_up_minimum is not UNSET:
            field_dict["scaleUpMinimum"] = scale_up_minimum
        if stable_window is not UNSET:
            field_dict["stableWindow"] = stable_window
        if target is not UNSET:
            field_dict["target"] = target

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: dict[str, Any]) -> T:
        if not src_dict:
            return None
        d = src_dict.copy()
        last_pod_retention_period = d.pop("lastPodRetentionPeriod", UNSET)

        max_num_replicas = d.pop("maxNumReplicas", UNSET)

        metric = d.pop("metric", UNSET)

        min_num_replicas = d.pop("minNumReplicas", UNSET)

        scale_down_delay = d.pop("scaleDownDelay", UNSET)

        scale_up_minimum = d.pop("scaleUpMinimum", UNSET)

        stable_window = d.pop("stableWindow", UNSET)

        target = d.pop("target", UNSET)

        serverless_config = cls(
            last_pod_retention_period=last_pod_retention_period,
            max_num_replicas=max_num_replicas,
            metric=metric,
            min_num_replicas=min_num_replicas,
            scale_down_delay=scale_down_delay,
            scale_up_minimum=scale_up_minimum,
            stable_window=stable_window,
            target=target,
        )

        serverless_config.additional_properties = d
        return serverless_config

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
