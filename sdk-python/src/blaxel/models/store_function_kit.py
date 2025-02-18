from typing import TYPE_CHECKING, Any, TypeVar, Union

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.store_function_parameter import StoreFunctionParameter


T = TypeVar("T", bound="StoreFunctionKit")


@_attrs_define
class StoreFunctionKit:
    """Store function kit

    Attributes:
        description (Union[Unset, str]): Description of the function kit, very important for the agent to work with your
            kit
        name (Union[Unset, str]): The kit name, very important for the agent to work with your kit
        parameters (Union[Unset, list['StoreFunctionParameter']]): Kit parameters, for your kit to be callable with an
            Agent
    """

    description: Union[Unset, str] = UNSET
    name: Union[Unset, str] = UNSET
    parameters: Union[Unset, list["StoreFunctionParameter"]] = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        description = self.description

        name = self.name

        parameters: Union[Unset, list[dict[str, Any]]] = UNSET
        if not isinstance(self.parameters, Unset):
            parameters = []
            for parameters_item_data in self.parameters:
                parameters_item = parameters_item_data.to_dict()
                parameters.append(parameters_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if description is not UNSET:
            field_dict["description"] = description
        if name is not UNSET:
            field_dict["name"] = name
        if parameters is not UNSET:
            field_dict["parameters"] = parameters

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: dict[str, Any]) -> T:
        from ..models.store_function_parameter import StoreFunctionParameter

        if not src_dict:
            return None
        d = src_dict.copy()
        description = d.pop("description", UNSET)

        name = d.pop("name", UNSET)

        parameters = []
        _parameters = d.pop("parameters", UNSET)
        for parameters_item_data in _parameters or []:
            parameters_item = StoreFunctionParameter.from_dict(parameters_item_data)

            parameters.append(parameters_item)

        store_function_kit = cls(
            description=description,
            name=name,
            parameters=parameters,
        )

        store_function_kit.additional_properties = d
        return store_function_kit

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
