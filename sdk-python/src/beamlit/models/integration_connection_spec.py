from typing import TYPE_CHECKING, Any, TypeVar, Union

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.integration_connection_config import IntegrationConnectionConfig
    from ..models.integration_connection_secret import IntegrationConnectionSecret


T = TypeVar("T", bound="IntegrationConnectionSpec")


@_attrs_define
class IntegrationConnectionSpec:
    """Integration connection specification

    Attributes:
        config (Union[Unset, IntegrationConnectionConfig]): Integration config
        integration (Union[Unset, str]): Integration type
        sandbox (Union[Unset, bool]): Sandbox mode
        secret (Union[Unset, IntegrationConnectionSecret]): Integration secret
    """

    config: Union[Unset, "IntegrationConnectionConfig"] = UNSET
    integration: Union[Unset, str] = UNSET
    sandbox: Union[Unset, bool] = UNSET
    secret: Union[Unset, "IntegrationConnectionSecret"] = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        config: Union[Unset, dict[str, Any]] = UNSET
        if self.config and not isinstance(self.config, Unset):
            config = self.config.to_dict()

        integration = self.integration

        sandbox = self.sandbox

        secret: Union[Unset, dict[str, Any]] = UNSET
        if self.secret and not isinstance(self.secret, Unset):
            secret = self.secret.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if config is not UNSET:
            field_dict["config"] = config
        if integration is not UNSET:
            field_dict["integration"] = integration
        if sandbox is not UNSET:
            field_dict["sandbox"] = sandbox
        if secret is not UNSET:
            field_dict["secret"] = secret

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: dict[str, Any]) -> T:
        from ..models.integration_connection_config import IntegrationConnectionConfig
        from ..models.integration_connection_secret import IntegrationConnectionSecret

        if not src_dict:
            return None
        d = src_dict.copy()
        _config = d.pop("config", UNSET)
        config: Union[Unset, IntegrationConnectionConfig]
        if isinstance(_config, Unset):
            config = UNSET
        else:
            config = IntegrationConnectionConfig.from_dict(_config)

        integration = d.pop("integration", UNSET)

        sandbox = d.pop("sandbox", UNSET)

        _secret = d.pop("secret", UNSET)
        secret: Union[Unset, IntegrationConnectionSecret]
        if isinstance(_secret, Unset):
            secret = UNSET
        else:
            secret = IntegrationConnectionSecret.from_dict(_secret)

        integration_connection_spec = cls(
            config=config,
            integration=integration,
            sandbox=sandbox,
            secret=secret,
        )

        integration_connection_spec.additional_properties = d
        return integration_connection_spec

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
