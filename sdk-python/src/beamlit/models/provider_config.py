from typing import TYPE_CHECKING, Any, Dict, List, Type, TypeVar, Union, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.runtime import Runtime


T = TypeVar("T", bound="ProviderConfig")


@_attrs_define
class ProviderConfig:
    """Model provider config

    Attributes:
        accessToken (Union[Unset, str]): The access token to use for the provider
        filename (Union[Unset, str]): The file name to use for the model
        presigned_url (Union[Unset, List[Any]]): The presigned URLs to upload the model to
        runtime (Union[Unset, Runtime]): Set of configurations for a deployment
    """

    accessToken: Union[Unset, str] = UNSET
    filename: Union[Unset, str] = UNSET
    presigned_url: Union[Unset, List[Any]] = UNSET
    runtime: Union[Unset, "Runtime"] = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        accessToken = self.accessToken

        filename = self.filename

        presigned_url: Union[Unset, List[Any]] = UNSET
        if not isinstance(self.presigned_url, Unset):
            presigned_url = self.presigned_url

        runtime: Union[Unset, Dict[str, Any]] = UNSET
        if not isinstance(self.runtime, Unset):
            runtime = self.runtime.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if accessToken is not UNSET:
            field_dict["accessToken"] = accessToken
        if filename is not UNSET:
            field_dict["filename"] = filename
        if presigned_url is not UNSET:
            field_dict["presigned_url"] = presigned_url
        if runtime is not UNSET:
            field_dict["runtime"] = runtime

        return field_dict

    @classmethod
    def from_dict(cls: Type[T], src_dict: dict[str, Any]) -> T:
        from ..models.runtime import Runtime

        d = src_dict.copy()
        accessToken = d.pop("accessToken", UNSET)

        filename = d.pop("filename", UNSET)

        presigned_url = cast(List[Any], d.pop("presigned_url", UNSET))

        _runtime = d.pop("runtime", UNSET)
        runtime: Union[Unset, Runtime]
        if isinstance(_runtime, Unset):
            runtime = UNSET
        else:
            runtime = Runtime.from_dict(_runtime)

        provider_config = cls(
            accessToken=accessToken,
            filename=filename,
            presigned_url=presigned_url,
            runtime=runtime,
        )

        provider_config.additional_properties = d
        return provider_config

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
