from typing import TYPE_CHECKING, Any, TypeVar, Union, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.template_variable import TemplateVariable


T = TypeVar("T", bound="Template")


@_attrs_define
class Template:
    """Blaxel template

    Attributes:
        default_branch (Union[Unset, str]): Default branch of the template
        description (Union[Unset, str]): Description of the template
        name (Union[Unset, str]): Name of the template
        sha (Union[Unset, str]): SHA of the variable
        topics (Union[Unset, list[str]]): Topic of the template
        url (Union[Unset, str]): URL of the template
        variables (Union[Unset, list['TemplateVariable']]): Variables of the template
    """

    default_branch: Union[Unset, str] = UNSET
    description: Union[Unset, str] = UNSET
    name: Union[Unset, str] = UNSET
    sha: Union[Unset, str] = UNSET
    topics: Union[Unset, list[str]] = UNSET
    url: Union[Unset, str] = UNSET
    variables: Union[Unset, list["TemplateVariable"]] = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        default_branch = self.default_branch

        description = self.description

        name = self.name

        sha = self.sha

        topics: Union[Unset, list[str]] = UNSET
        if not isinstance(self.topics, Unset):
            topics = self.topics

        url = self.url

        variables: Union[Unset, list[dict[str, Any]]] = UNSET
        if not isinstance(self.variables, Unset):
            variables = []
            for variables_item_data in self.variables:
                if type(variables_item_data) == dict:
                    variables_item = variables_item_data
                else:
                    variables_item = variables_item_data.to_dict()
                variables.append(variables_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if default_branch is not UNSET:
            field_dict["defaultBranch"] = default_branch
        if description is not UNSET:
            field_dict["description"] = description
        if name is not UNSET:
            field_dict["name"] = name
        if sha is not UNSET:
            field_dict["sha"] = sha
        if topics is not UNSET:
            field_dict["topics"] = topics
        if url is not UNSET:
            field_dict["url"] = url
        if variables is not UNSET:
            field_dict["variables"] = variables

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: dict[str, Any]) -> T:
        from ..models.template_variable import TemplateVariable

        if not src_dict:
            return None
        d = src_dict.copy()
        default_branch = d.pop("defaultBranch", UNSET)

        description = d.pop("description", UNSET)

        name = d.pop("name", UNSET)

        sha = d.pop("sha", UNSET)

        topics = cast(list[str], d.pop("topics", UNSET))

        url = d.pop("url", UNSET)

        variables = []
        _variables = d.pop("variables", UNSET)
        for variables_item_data in _variables or []:
            variables_item = TemplateVariable.from_dict(variables_item_data)

            variables.append(variables_item)

        template = cls(
            default_branch=default_branch,
            description=description,
            name=name,
            sha=sha,
            topics=topics,
            url=url,
            variables=variables,
        )

        template.additional_properties = d
        return template

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
