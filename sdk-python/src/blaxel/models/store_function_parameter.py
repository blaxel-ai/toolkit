from typing import Any, TypeVar, Union

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="StoreFunctionParameter")


@_attrs_define
class StoreFunctionParameter:
    """Store function parameter

    Attributes:
        description (Union[Unset, str]): Store function parameter description
        name (Union[Unset, str]): Store function parameter name
        required (Union[Unset, bool]): Store function parameter required
        type_ (Union[Unset, str]): Store function parameter type
    """

    description: Union[Unset, str] = UNSET
    name: Union[Unset, str] = UNSET
    required: Union[Unset, bool] = UNSET
    type_: Union[Unset, str] = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        description = self.description

        name = self.name

        required = self.required

        type_ = self.type_

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if description is not UNSET:
            field_dict["description"] = description
        if name is not UNSET:
            field_dict["name"] = name
        if required is not UNSET:
            field_dict["required"] = required
        if type_ is not UNSET:
            field_dict["type"] = type_

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: dict[str, Any]) -> T:
        if not src_dict:
            return None
        d = src_dict.copy()
        description = d.pop("description", UNSET)

        name = d.pop("name", UNSET)

        required = d.pop("required", UNSET)

        type_ = d.pop("type", UNSET)

        store_function_parameter = cls(
            description=description,
            name=name,
            required=required,
            type_=type_,
        )

        store_function_parameter.additional_properties = d
        return store_function_parameter

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
