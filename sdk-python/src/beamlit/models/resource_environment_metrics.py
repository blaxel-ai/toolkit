from typing import TYPE_CHECKING, Any, TypeVar, Union

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.metric import Metric
    from ..models.qps import QPS
    from ..models.resource_environment_metrics_inference_per_region import (
        ResourceEnvironmentMetricsInferencePerRegion,
    )
    from ..models.resource_environment_metrics_query_per_region_per_code import (
        ResourceEnvironmentMetricsQueryPerRegionPerCode,
    )


T = TypeVar("T", bound="ResourceEnvironmentMetrics")


@_attrs_define
class ResourceEnvironmentMetrics:
    """Metrics for a single resource deployment (eg. model deployment, function deployment)

    Attributes:
        inference_global (Union[Unset, list['Metric']]): Array of metrics
        inference_per_region (Union[Unset, ResourceEnvironmentMetricsInferencePerRegion]): Historical requests (in last
            24 hours) per location, for the model deployment
        query_global (Union[Unset, float]): Number of requests done on the resource for the model deployment
        query_per_code_global (Union[Unset, QPS]): Query per second per element, can be per response status code (e.g.
            200, 400) or per location
        query_per_region (Union[Unset, QPS]): Query per second per element, can be per response status code (e.g. 200,
            400) or per location
        query_per_region_per_code (Union[Unset, ResourceEnvironmentMetricsQueryPerRegionPerCode]): Number of requests
            done on the resource for the model deployment
    """

    inference_global: Union[Unset, list["Metric"]] = UNSET
    inference_per_region: Union[Unset, "ResourceEnvironmentMetricsInferencePerRegion"] = UNSET
    query_global: Union[Unset, float] = UNSET
    query_per_code_global: Union[Unset, "QPS"] = UNSET
    query_per_region: Union[Unset, "QPS"] = UNSET
    query_per_region_per_code: Union[Unset, "ResourceEnvironmentMetricsQueryPerRegionPerCode"] = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        inference_global: Union[Unset, list[dict[str, Any]]] = UNSET
        if not isinstance(self.inference_global, Unset):
            inference_global = []
            for componentsschemas_array_metric_item_data in self.inference_global:
                componentsschemas_array_metric_item = componentsschemas_array_metric_item_data.to_dict()
                inference_global.append(componentsschemas_array_metric_item)

        inference_per_region: Union[Unset, dict[str, Any]] = UNSET
        if not isinstance(self.inference_per_region, Unset):
            inference_per_region = self.inference_per_region.to_dict()

        query_global = self.query_global

        query_per_code_global: Union[Unset, dict[str, Any]] = UNSET
        if not isinstance(self.query_per_code_global, Unset):
            query_per_code_global = self.query_per_code_global.to_dict()

        query_per_region: Union[Unset, dict[str, Any]] = UNSET
        if not isinstance(self.query_per_region, Unset):
            query_per_region = self.query_per_region.to_dict()

        query_per_region_per_code: Union[Unset, dict[str, Any]] = UNSET
        if not isinstance(self.query_per_region_per_code, Unset):
            query_per_region_per_code = self.query_per_region_per_code.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if inference_global is not UNSET:
            field_dict["inferenceGlobal"] = inference_global
        if inference_per_region is not UNSET:
            field_dict["inferencePerRegion"] = inference_per_region
        if query_global is not UNSET:
            field_dict["query_global"] = query_global
        if query_per_code_global is not UNSET:
            field_dict["queryPerCodeGlobal"] = query_per_code_global
        if query_per_region is not UNSET:
            field_dict["queryPerRegion"] = query_per_region
        if query_per_region_per_code is not UNSET:
            field_dict["queryPerRegionPerCode"] = query_per_region_per_code

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: dict[str, Any]) -> T:
        from ..models.metric import Metric
        from ..models.qps import QPS
        from ..models.resource_environment_metrics_inference_per_region import (
            ResourceEnvironmentMetricsInferencePerRegion,
        )
        from ..models.resource_environment_metrics_query_per_region_per_code import (
            ResourceEnvironmentMetricsQueryPerRegionPerCode,
        )

        if not src_dict:
            return None
        d = src_dict.copy()
        inference_global = []
        _inference_global = d.pop("inferenceGlobal", UNSET)
        for componentsschemas_array_metric_item_data in _inference_global or []:
            componentsschemas_array_metric_item = Metric.from_dict(componentsschemas_array_metric_item_data)

            inference_global.append(componentsschemas_array_metric_item)

        _inference_per_region = d.pop("inferencePerRegion", UNSET)
        inference_per_region: Union[Unset, ResourceEnvironmentMetricsInferencePerRegion]
        if isinstance(_inference_per_region, Unset):
            inference_per_region = UNSET
        else:
            inference_per_region = ResourceEnvironmentMetricsInferencePerRegion.from_dict(_inference_per_region)

        query_global = d.pop("query_global", UNSET)

        _query_per_code_global = d.pop("queryPerCodeGlobal", UNSET)
        query_per_code_global: Union[Unset, QPS]
        if isinstance(_query_per_code_global, Unset):
            query_per_code_global = UNSET
        else:
            query_per_code_global = QPS.from_dict(_query_per_code_global)

        _query_per_region = d.pop("queryPerRegion", UNSET)
        query_per_region: Union[Unset, QPS]
        if isinstance(_query_per_region, Unset):
            query_per_region = UNSET
        else:
            query_per_region = QPS.from_dict(_query_per_region)

        _query_per_region_per_code = d.pop("queryPerRegionPerCode", UNSET)
        query_per_region_per_code: Union[Unset, ResourceEnvironmentMetricsQueryPerRegionPerCode]
        if isinstance(_query_per_region_per_code, Unset):
            query_per_region_per_code = UNSET
        else:
            query_per_region_per_code = ResourceEnvironmentMetricsQueryPerRegionPerCode.from_dict(
                _query_per_region_per_code
            )

        resource_environment_metrics = cls(
            inference_global=inference_global,
            inference_per_region=inference_per_region,
            query_global=query_global,
            query_per_code_global=query_per_code_global,
            query_per_region=query_per_region,
            query_per_region_per_code=query_per_region_per_code,
        )

        resource_environment_metrics.additional_properties = d
        return resource_environment_metrics

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
